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

// Package failover connects pkg/hub/resolver to the worker's actual hub
// connections: it decides which hub to start against, and notices when that
// answer changes.
//
// Everything here is inert unless HUB_SECONDARY_HOST_ENDPOINT is set. A worker
// with one hub configured — every deployment that exists today — builds no
// resolver, opens no extra clients, and connects exactly as it always has.
package failover

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	hub "github.com/kubeslice/worker-operator/pkg/hub/hubclient"
	"github.com/kubeslice/worker-operator/pkg/hub/resolver"
	"github.com/kubeslice/worker-operator/pkg/utils"
)

// Candidate names used in logs and metric labels. They identify the configured
// slot, not the role a hub currently holds — which hub is Active is exactly the
// thing being resolved, and may change while the process runs.
const (
	primaryName   = "primary"
	secondaryName = "secondary"
)

const (
	defaultSecondaryTokenFile = "/var/run/secrets/kubernetes.io/hub-secondary-serviceaccount/token"
	defaultSecondaryCAFile    = "/var/run/secrets/kubernetes.io/hub-secondary-serviceaccount/ca.crt"
	defaultInterval           = 10 * time.Second
	defaultConfirmations      = 2
)

var (
	hubSwitchesTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "kubeslice_worker_hub_switches_total",
		Help: "Number of times this worker resolved a different active hub and restarted to follow it.",
	})
	hubProbeErrorsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "kubeslice_worker_hub_probe_errors_total",
		Help: "Failed reads of a hub's copy of this worker's Cluster CR, by configured hub slot.",
	}, []string{"hub"})
	// controllerReconnectAttemptsTotal counts every resolved winner this
	// worker acted on at startup, by whether it matched the configured
	// primary or a resolved switch. It is deliberately not "every poll" —
	// gather() already runs every tick and hubProbeErrorsTotal covers probe
	// failures; this metric is about connection *decisions*, issue #469's ask.
	controllerReconnectAttemptsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "kubeslice_worker_controller_reconnect_attempts_total",
		Help: "Startup hub-connection decisions this worker has made, by result.",
	}, []string{"result"})
	controllerLastSyncTime = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "kubeslice_worker_controller_last_sync_time_seconds",
		Help: "Unix time of the last resolved hub-connection decision at startup.",
	})
)

func init() {
	ctrlmetrics.Registry.MustRegister(hubSwitchesTotal, hubProbeErrorsTotal,
		controllerReconnectAttemptsTotal, controllerLastSyncTime)
}

// Config is the failover-following configuration, read from the environment.
type Config struct {
	SecondaryEndpoint  string
	SecondaryTokenFile string
	SecondaryCAFile    string
	ClusterName        string
	Namespace          string
	Interval           time.Duration
	Timeout            time.Duration
	Confirmations      int
}

// ConfigFromEnv reads the configuration. Only SecondaryEndpoint has no usable
// default: without a second hub to compare against there is nothing to resolve,
// which is why it doubles as the feature's on switch.
func ConfigFromEnv() Config {
	return Config{
		SecondaryEndpoint:  utils.GetEnvOrDefault("HUB_SECONDARY_HOST_ENDPOINT", ""),
		SecondaryTokenFile: utils.GetEnvOrDefault("HUB_SECONDARY_TOKEN_FILE", defaultSecondaryTokenFile),
		SecondaryCAFile:    utils.GetEnvOrDefault("HUB_SECONDARY_CA_FILE", defaultSecondaryCAFile),
		ClusterName:        hub.ClusterName,
		Namespace:          hub.ProjectNamespace,
		Interval:           durationFromEnv("HUB_RESOLVE_INTERVAL", defaultInterval),
		Timeout:            durationFromEnv("HUB_RESOLVE_TIMEOUT", resolver.DefaultProbeTimeout),
		Confirmations:      intFromEnv("HUB_SWITCH_CONFIRMATIONS", defaultConfirmations),
	}
}

// Enabled reports whether a second hub is configured. Everything in this
// package is a no-op when it is not.
func (c Config) Enabled() bool { return c.SecondaryEndpoint != "" }

func durationFromEnv(key string, fallback time.Duration) time.Duration {
	raw := utils.GetEnvOrDefault(key, "")
	if raw == "" {
		return fallback
	}
	// An unparseable value falls back rather than failing: this is a tuning
	// knob, and refusing to start over a malformed one would be a worse
	// outcome than running at the default.
	parsed, err := time.ParseDuration(raw)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func intFromEnv(key string, fallback int) int {
	raw := utils.GetEnvOrDefault(key, "")
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

// Follower resolves which hub is Active and reports when that changes.
type Follower struct {
	resolver   *resolver.Resolver
	byEndpoint map[string]hub.Connection
	primary    hub.Connection
	interval   time.Duration
	log        logr.Logger
}

// New builds a Follower over the primary hub and the configured secondary.
//
// readerFor exists so tests can supply their own hub readers; production passes
// nil and gets real clients. The clients it builds are opened once and never
// rebuilt: they have to keep working across a failover, which is precisely what
// makes them useless if they are tied to the connection a failover replaces.
func New(cfg Config, primary hub.Connection, log logr.Logger,
	readerFor func(hub.Connection) (resolver.ClusterReader, error)) (*Follower, error) {

	if !cfg.Enabled() {
		return nil, fmt.Errorf("no secondary hub configured")
	}
	secondary := hub.Connection{
		Endpoint:  cfg.SecondaryEndpoint,
		TokenFile: cfg.SecondaryTokenFile,
		CAFile:    cfg.SecondaryCAFile,
	}
	if secondary.Endpoint == primary.Endpoint {
		return nil, fmt.Errorf("the secondary hub endpoint %q is the same as the primary", secondary.Endpoint)
	}
	if readerFor == nil {
		readerFor = newClusterReader
	}

	byEndpoint := map[string]hub.Connection{
		primary.Endpoint:   primary,
		secondary.Endpoint: secondary,
	}
	candidates := []resolver.HubCandidate{
		{Name: primaryName, Endpoint: primary.Endpoint, TokenFile: primary.TokenFile, CAFile: primary.CAFile},
		{Name: secondaryName, Endpoint: secondary.Endpoint, TokenFile: secondary.TokenFile, CAFile: secondary.CAFile},
	}

	probe := resolver.NewProbe(
		func(c resolver.HubCandidate) (resolver.ClusterReader, error) {
			return readerFor(hub.Connection{Endpoint: c.Endpoint, TokenFile: c.TokenFile, CAFile: c.CAFile})
		},
		resolver.ProbeConfig{ClusterName: cfg.ClusterName, Namespace: cfg.Namespace, Timeout: cfg.Timeout},
	)
	// Count probe failures without changing the verdict: an unreachable hub is
	// already handled by the rule, but a hub that is quietly unreachable for
	// days is the thing an operator needs to see before a failover, not after.
	//
	// Every failed read counts, which is what the metric's help text promises
	// and what gating on Reachable alone did not deliver: a hub that answers
	// but has no Cluster CR for this worker returns Reachable with an error,
	// and that misconfiguration used to be invisible here as well as in the
	// resolver's log.
	counted := func(ctx context.Context, c resolver.HubCandidate) resolver.Verdict {
		v := probe(ctx, c)
		if !v.Reachable || v.Err != nil {
			hubProbeErrorsTotal.WithLabelValues(c.Name).Inc()
		}
		return v
	}

	r, err := resolver.New(candidates, counted, resolver.Options{
		SwitchConfirmations: cfg.Confirmations,
		Log:                 log,
	})
	if err != nil {
		return nil, err
	}
	interval := cfg.Interval
	if interval <= 0 {
		interval = defaultInterval
	}
	return &Follower{
		resolver:   r,
		byEndpoint: byEndpoint,
		primary:    primary,
		interval:   interval,
		log:        log,
	}, nil
}

// StartupConnection resolves once and returns the hub to connect to, plus
// whether that hub was reached by following a resolved switch rather than
// falling back to the configured primary. The bool is issue #469's
// "ReconnectedAfterFailover vs Connected" distinction: main.go has no other
// way to know, since a fresh process looks identical whether it just started
// normally or was just restarted to follow a failover.
//
// Falling back to the primary when nothing resolves is deliberate. A worker
// that refused to start because it could not reach a hub would turn a hub
// outage into a worker outage, and the primary is the same hub it would have
// used before any of this existed.
func (f *Follower) StartupConnection(ctx context.Context) (hub.Connection, bool) {
	claim := f.resolver.Resolve(ctx)
	if claim == nil {
		f.log.Info("no active hub resolved at startup; using the configured primary",
			"endpoint", f.primary.Endpoint)
		controllerReconnectAttemptsTotal.WithLabelValues("unresolved").Inc()
		controllerLastSyncTime.SetToCurrentTime()
		return f.primary, false
	}
	conn, ok := f.byEndpoint[claim.Endpoint]
	if !ok {
		// Unreachable in practice: the resolver already refuses claims naming
		// endpoints outside the candidate set, and the candidates are built
		// from this same map. Handled anyway so a future change to either side
		// cannot silently produce a connection with mismatched credentials.
		f.log.Info("resolved hub is not a configured connection; using the primary",
			"resolvedEndpoint", claim.Endpoint)
		controllerReconnectAttemptsTotal.WithLabelValues("unresolved").Inc()
		controllerLastSyncTime.SetToCurrentTime()
		return f.primary, false
	}
	f.log.Info("connecting to the resolved active hub",
		"identity", claim.Identity, "endpoint", conn.Endpoint)
	viaSwitch := conn.Endpoint != f.primary.Endpoint
	result := "primary"
	if viaSwitch {
		result = "resolved-switch"
	}
	controllerReconnectAttemptsTotal.WithLabelValues(result).Inc()
	controllerLastSyncTime.SetToCurrentTime()
	return conn, viaSwitch
}

// Watch polls until ctx is done, calling onSwitch once the resolved active hub
// differs from the one this process started against.
//
// Acting on the change is the caller's business, and in practice means shutting
// the process down so it restarts and resolves again. This function does not
// exit the process itself: a package that reads state should not also decide to
// terminate, and keeping them apart is what makes this testable.
func (f *Follower) Watch(ctx context.Context, startedWith hub.Connection, onSwitch func(resolver.Claim)) {
	ticker := time.NewTicker(f.interval)
	defer ticker.Stop()
	f.log.Info("watching for hub failover", "interval", f.interval, "connectedTo", startedWith.Endpoint)

	for {
		select {
		case <-ctx.Done():
			f.log.Info("hub failover watch stopped", "reason", ctx.Err())
			return
		case <-ticker.C:
			claim := f.resolver.Resolve(ctx)
			if claim == nil || claim.Endpoint == startedWith.Endpoint {
				continue
			}
			hubSwitchesTotal.Inc()
			f.log.Info("the active hub changed; this worker must reconnect",
				"from", startedWith.Endpoint, "to", claim.Endpoint, "identity", claim.Identity)
			onSwitch(*claim)
			return
		}
	}
}

// newClusterReader opens a client to one hub. Deliberately uncached: this reads
// a single object on a timer, and a cache would add an informer, a watch and a
// resync against a hub this worker may not even be talking to.
func newClusterReader(conn hub.Connection) (resolver.ClusterReader, error) {
	c, err := client.New(conn.RestConfig(), client.Options{})
	if err != nil {
		return nil, err
	}
	return resolver.NewClusterReader(c), nil
}
