/*
 *  Copyright (c) 2022 Avesha, Inc. All rights reserved.
 *
 *  SPDX-License-Identifier: Apache-2.0
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

package resolver

import (
	"context"
	"errors"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// clusterGVK is the hub-side Cluster CR carrying status.activeController.
//
// Read unstructured rather than through the shared github.com/kubeslice/apis
// types on purpose. This package needs four scalars out of one status field,
// and reading them untyped means it does not depend on an apis release shipping
// ActiveControllerInfo — which would otherwise put a third repository's release
// cadence on the critical path of a feature that is otherwise self-contained.
// The typed watch the worker already runs over its own Cluster CR is untouched.
var clusterGVK = schema.GroupVersionKind{
	Group:   "controller.kubeslice.io",
	Version: "v1alpha1",
	Kind:    "Cluster",
}

// ClusterReader reads one object from one hub. Narrowed to what the probe needs
// so a caller can hand over a controller-runtime client, and a test can hand
// over anything.
type ClusterReader interface {
	Get(ctx context.Context, key types.NamespacedName, obj *unstructured.Unstructured) error
}

// clientReader adapts a controller-runtime client to ClusterReader.
type clientReader struct{ c client.Client }

func (r clientReader) Get(ctx context.Context, key types.NamespacedName, obj *unstructured.Unstructured) error {
	return r.c.Get(ctx, key, obj)
}

// NewClusterReader adapts a controller-runtime client for use as a probe target.
func NewClusterReader(c client.Client) ClusterReader {
	return clientReader{c: c}
}

// ProbeConfig describes which object a probe reads and how long it may take.
type ProbeConfig struct {
	// ClusterName is this worker's own Cluster CR, the object every hub keeps a
	// copy of and stamps activeController onto.
	ClusterName string
	// Namespace is the hub-side project namespace holding that CR.
	Namespace string
	// Timeout bounds each read. Zero means DefaultProbeTimeout.
	Timeout time.Duration
}

// NewProbe returns a Prober that reads each candidate's copy of this worker's
// Cluster CR through the reader built for that candidate by readerFor.
//
// readerFor is a function rather than a prepared map because a candidate's
// client is built from its own endpoint and credentials, and the caller owns
// that construction. Building them once at startup and never rebuilding them is
// the intent: these probes must keep working across a failover, so they are
// deliberately independent of the worker's primary hub connection, which is the
// thing a failover replaces.
func NewProbe(readerFor func(HubCandidate) (ClusterReader, error), cfg ProbeConfig) Prober {
	if cfg.Timeout <= 0 {
		cfg.Timeout = DefaultProbeTimeout
	}
	// Built once per candidate and kept, which is what the comment above
	// describes and what this did not do: readerFor was called inside the
	// closure below, so every poll constructed a fresh client and with it a
	// fresh lazy RESTMapper, whose first use costs a discovery round-trip
	// against that hub — once per candidate per poll, forever. A failed
	// construction is deliberately not cached, so a hub that could not be
	// built at one poll is retried at the next.
	var (
		mu      sync.Mutex
		readers = make(map[string]ClusterReader)
	)
	readerOnce := func(candidate HubCandidate) (ClusterReader, error) {
		mu.Lock()
		defer mu.Unlock()
		if r, built := readers[candidate.Endpoint]; built {
			return r, nil
		}
		r, err := readerFor(candidate)
		if err != nil {
			return nil, err
		}
		readers[candidate.Endpoint] = r
		return r, nil
	}

	return func(ctx context.Context, candidate HubCandidate) Verdict {
		reader, err := readerOnce(candidate)
		if err != nil {
			return Verdict{Candidate: candidate, Err: err}
		}

		readCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()

		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(clusterGVK)
		key := types.NamespacedName{Name: cfg.ClusterName, Namespace: cfg.Namespace}
		if err := reader.Get(readCtx, key, obj); err != nil {
			// An error carrying an API status came from the hub's own API
			// server, so the hub answered. NotFound (this worker has no
			// Cluster CR there) and Forbidden/Unauthorized (RBAC on that hub)
			// are misconfigurations, not outages, and reporting either as
			// unreachable would hide one behind a connectivity error. Only a
			// transport failure means the hub never answered at all.
			var status apierrors.APIStatus
			if errors.As(err, &status) {
				return Verdict{Candidate: candidate, Reachable: true, Err: err}
			}
			return Verdict{Candidate: candidate, Err: err}
		}

		return Verdict{
			Candidate: candidate,
			Reachable: true,
			Claim:     claimFrom(obj, candidate.Name),
		}
	}
}

// claimFrom decodes status.activeController, returning nil when the field is
// absent or unusable.
//
// A partially-written field is treated as absent rather than as an error: an
// endpoint or identity that is missing cannot be acted on, and a worker that
// refused to resolve at all because one hub published something malformed would
// be broken by the other hub's bug.
func claimFrom(obj *unstructured.Unstructured, source string) *Claim {
	raw, found, err := unstructured.NestedMap(obj.Object, "status", "activeController")
	if err != nil || !found || raw == nil {
		return nil
	}
	endpoint, _, _ := unstructured.NestedString(raw, "endpoint")
	identity, _, _ := unstructured.NestedString(raw, "activeIdentity")
	if endpoint == "" || identity == "" {
		return nil
	}
	claim := &Claim{Endpoint: endpoint, Identity: identity, Source: source}
	if stamp, ok, _ := unstructured.NestedString(raw, "lastUpdated"); ok && stamp != "" {
		// Absence or a malformed stamp leaves the zero time, which orders last
		// in the tie-break. A hub that cannot say when it last declared itself
		// should not win against one that can.
		if parsed, err := time.Parse(time.RFC3339, stamp); err == nil {
			claim.LastUpdated = parsed
		}
	}
	return claim
}
