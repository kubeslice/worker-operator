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
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

const (
	testCluster   = "worker-1"
	testNamespace = "kubeslice-avesha"
)

// readerFunc adapts a plain function to ClusterReader.
type readerFunc func(ctx context.Context, key types.NamespacedName, obj *unstructured.Unstructured) error

func (f readerFunc) Get(ctx context.Context, key types.NamespacedName, obj *unstructured.Unstructured) error {
	return f(ctx, key, obj)
}

// clusterWith builds the hub-side Cluster CR carrying an activeController.
func clusterWith(activeController map[string]interface{}) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{}}
	obj.SetGroupVersionKind(clusterGVK)
	obj.SetName(testCluster)
	obj.SetNamespace(testNamespace)
	if activeController != nil {
		_ = unstructured.SetNestedMap(obj.Object, activeController, "status", "activeController")
	}
	return obj
}

// probeReturning builds a probe whose single reader serves obj (or err).
func probeReturning(t *testing.T, obj *unstructured.Unstructured, err error, timeout time.Duration) Prober {
	t.Helper()
	reader := readerFunc(func(ctx context.Context, key types.NamespacedName, into *unstructured.Unstructured) error {
		assert.Equal(t, testCluster, key.Name, "the probe must read this worker's own Cluster CR")
		assert.Equal(t, testNamespace, key.Namespace)
		if err != nil {
			return err
		}
		into.Object = obj.Object
		return nil
	})
	return NewProbe(
		func(HubCandidate) (ClusterReader, error) { return reader, nil },
		ProbeConfig{ClusterName: testCluster, Namespace: testNamespace, Timeout: timeout},
	)
}

func TestProbe_DecodesAFullDeclaration(t *testing.T) {
	stamp := time.Now().UTC().Truncate(time.Second)
	probe := probeReturning(t, clusterWith(map[string]interface{}{
		"endpoint":       hubA.Endpoint,
		"activeIdentity": "hub-a-1",
		"lastUpdated":    stamp.Format(time.RFC3339),
	}), nil, time.Second)

	got := probe(context.Background(), hubA)
	assert.True(t, got.Reachable)
	claim := mustClaim(t, got.Claim, "a complete activeController must decode")
	assert.Equal(t, hubA.Endpoint, claim.Endpoint)
	assert.Equal(t, "hub-a-1", claim.Identity)
	assert.True(t, stamp.Equal(claim.LastUpdated), "lastUpdated must round-trip")
	assert.Equal(t, hubA.Name, claim.Source, "the claim records which hub answered")
}

// TestProbe_NoFieldIsNotAnError is the non-HA path: a hub that answers and
// publishes nothing is healthy and has nothing to say. Treating that as a
// failure would make every existing worker log errors forever.
func TestProbe_NoFieldIsNotAnError(t *testing.T) {
	probe := probeReturning(t, clusterWith(nil), nil, time.Second)

	got := probe(context.Background(), hubA)
	assert.True(t, got.Reachable, "the hub answered")
	assert.Nil(t, got.Claim)
	assert.NoError(t, got.Err)
}

// TestProbe_PartialDeclarationIsIgnored: a claim missing either half cannot be
// acted on. Treating it as an error instead would let one hub's bug stop the
// worker from resolving against the other.
func TestProbe_PartialDeclarationIsIgnored(t *testing.T) {
	for name, ac := range map[string]map[string]interface{}{
		"no endpoint": {"activeIdentity": "hub-a-1"},
		"no identity": {"endpoint": hubA.Endpoint},
		"both empty":  {"endpoint": "", "activeIdentity": ""},
	} {
		t.Run(name, func(t *testing.T) {
			got := probeReturning(t, clusterWith(ac), nil, time.Second)(context.Background(), hubA)
			assert.True(t, got.Reachable)
			assert.Nil(t, got.Claim, "a half-written declaration must not be acted on")
		})
	}
}

// TestProbe_UnparseableTimestampStillYieldsAClaim: the stamp only orders
// conflicting claims. Dropping an otherwise-valid declaration because its
// timestamp was malformed would be worse than ordering it last.
func TestProbe_UnparseableTimestampStillYieldsAClaim(t *testing.T) {
	probe := probeReturning(t, clusterWith(map[string]interface{}{
		"endpoint":       hubA.Endpoint,
		"activeIdentity": "hub-a-1",
		"lastUpdated":    "not-a-timestamp",
	}), nil, time.Second)

	claim := mustClaim(t, probe(context.Background(), hubA).Claim, "a bad stamp must not void the claim")
	assert.True(t, claim.LastUpdated.IsZero(), "an unusable stamp orders last in the tie-break")
}

// TestProbe_NotFoundIsReachable separates "this hub is down" from "this worker
// is not registered on this hub". Only the first is a connectivity problem, and
// conflating them would hide a real misconfiguration behind a network error.
func TestProbe_NotFoundIsReachable(t *testing.T) {
	notFound := apierrors.NewNotFound(schema.GroupResource{Group: clusterGVK.Group, Resource: "clusters"}, testCluster)
	got := probeReturning(t, nil, notFound, time.Second)(context.Background(), hubA)

	assert.True(t, got.Reachable, "the API server answered; it just has no such object")
	assert.Nil(t, got.Claim)
	assert.Error(t, got.Err)
}

// TestProbe_ForbiddenIsReachable is the RBAC half of the same distinction. A
// hub that refuses the read has still proved it is up, so it belongs on the
// resolver's "answered but could not be read" path; reporting it as
// unreachable sends an operator hunting a network fault that does not exist.
func TestProbe_ForbiddenIsReachable(t *testing.T) {
	forbidden := apierrors.NewForbidden(
		schema.GroupResource{Group: clusterGVK.Group, Resource: "clusters"},
		testCluster,
		fmt.Errorf("clusters.controller.kubeslice.io is forbidden"),
	)
	got := probeReturning(t, nil, forbidden, time.Second)(context.Background(), hubA)

	assert.True(t, got.Reachable, "the API server answered; it just refused the read")
	assert.Nil(t, got.Claim)
	assert.Error(t, got.Err)
}

func TestProbe_TransportFailureIsUnreachable(t *testing.T) {
	got := probeReturning(t, nil, fmt.Errorf("connection refused"), time.Second)(context.Background(), hubA)
	assert.False(t, got.Reachable)
	assert.Error(t, got.Err)
	assert.Nil(t, got.Claim)
}

func TestProbe_ReaderConstructionFailureIsReported(t *testing.T) {
	probe := NewProbe(
		func(HubCandidate) (ClusterReader, error) { return nil, fmt.Errorf("bad kubeconfig") },
		ProbeConfig{ClusterName: testCluster, Namespace: testNamespace},
	)
	got := probe(context.Background(), hubA)
	assert.False(t, got.Reachable)
	assert.Error(t, got.Err)
}

// TestProbe_BoundsAHangingRead is the whole reason every read here is wrapped.
// An API server that accepts a connection and then stops answering — a
// powered-off node, a partition dropping packets — leaves an unbounded read
// hanging until the OS TCP timeout, minutes later. The controller side of this
// feature shipped exactly that bug and measured a single read blocking ~12s
// against a stopped API server.
// TestProbe_BuildsEachCandidateReaderOnce pins what NewProbe's doc comment
// promises: a candidate's client is constructed at the first probe and kept.
// Rebuilding it per poll meant a fresh lazy RESTMapper each time, and a
// discovery round-trip against that hub on its first use.
func TestProbe_BuildsEachCandidateReaderOnce(t *testing.T) {
	built := map[string]int{}
	reader := readerFunc(func(_ context.Context, _ types.NamespacedName, into *unstructured.Unstructured) error {
		into.Object = clusterWith(nil).Object
		return nil
	})
	probe := NewProbe(func(c HubCandidate) (ClusterReader, error) {
		built[c.Endpoint]++
		return reader, nil
	}, ProbeConfig{ClusterName: testCluster, Namespace: testNamespace})

	for i := 0; i < 3; i++ {
		probe(context.Background(), hubA)
		probe(context.Background(), hubB)
	}

	assert.Equal(t, 1, built[hubA.Endpoint], "each candidate's reader must be built exactly once")
	assert.Equal(t, 1, built[hubB.Endpoint])
}

// TestProbe_RebuildsAReaderThatFailedToBuild is the other half: a failed
// construction must not be cached, or a hub that was briefly unreachable at the
// first poll would stay unprobeable for the life of the process.
func TestProbe_RebuildsAReaderThatFailedToBuild(t *testing.T) {
	attempts := 0
	reader := readerFunc(func(_ context.Context, _ types.NamespacedName, into *unstructured.Unstructured) error {
		into.Object = clusterWith(nil).Object
		return nil
	})
	probe := NewProbe(func(HubCandidate) (ClusterReader, error) {
		attempts++
		if attempts == 1 {
			return nil, errors.New("no credentials yet")
		}
		return reader, nil
	}, ProbeConfig{ClusterName: testCluster, Namespace: testNamespace})

	first := probe(context.Background(), hubA)
	assert.False(t, first.Reachable, "a hub whose reader will not build is not reachable")
	assert.Error(t, first.Err)

	second := probe(context.Background(), hubA)
	assert.True(t, second.Reachable, "the next poll must retry the construction")
	assert.Equal(t, 2, attempts)
}

func TestProbe_BoundsAHangingRead(t *testing.T) {
	hang := readerFunc(func(ctx context.Context, _ types.NamespacedName, _ *unstructured.Unstructured) error {
		<-ctx.Done() // never answers; only the probe's own deadline ends this
		return ctx.Err()
	})
	probe := NewProbe(
		func(HubCandidate) (ClusterReader, error) { return hang, nil },
		ProbeConfig{ClusterName: testCluster, Namespace: testNamespace, Timeout: 50 * time.Millisecond},
	)

	start := time.Now()
	got := probe(context.Background(), hubA)
	elapsed := time.Since(start)

	assert.False(t, got.Reachable)
	assert.Error(t, got.Err)
	assert.Less(t, elapsed, 2*time.Second,
		"a hanging hub must not block the poll loop; it returned after %s", elapsed)
}

func TestProbe_DefaultsTheTimeout(t *testing.T) {
	var seen time.Duration
	reader := readerFunc(func(ctx context.Context, _ types.NamespacedName, _ *unstructured.Unstructured) error {
		deadline, ok := ctx.Deadline()
		assert.True(t, ok, "every read must carry a deadline")
		seen = time.Until(deadline)
		return fmt.Errorf("done")
	})
	probe := NewProbe(
		func(HubCandidate) (ClusterReader, error) { return reader, nil },
		ProbeConfig{ClusterName: testCluster, Namespace: testNamespace}, // no Timeout
	)

	probe(context.Background(), hubA)
	assert.Greater(t, seen, time.Duration(0))
	assert.LessOrEqual(t, seen, DefaultProbeTimeout)
}
