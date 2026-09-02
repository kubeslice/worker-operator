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

package failover

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	hub "github.com/kubeslice/worker-operator/pkg/hub/hubclient"
	"github.com/kubeslice/worker-operator/pkg/hub/resolver"
)

const (
	primaryEndpoint   = "https://hub-a.example:6443"
	secondaryEndpoint = "https://hub-b.example:6443"
	clusterName       = "worker-1"
	namespace         = "kubeslice-avesha"
)

func primaryConn() hub.Connection {
	return hub.Connection{Endpoint: primaryEndpoint, TokenFile: "/creds/a/token", CAFile: "/creds/a/ca.crt"}
}

func testConfig() Config {
	return Config{
		SecondaryEndpoint:  secondaryEndpoint,
		SecondaryTokenFile: "/creds/b/token",
		SecondaryCAFile:    "/creds/b/ca.crt",
		ClusterName:        clusterName,
		Namespace:          namespace,
		Interval:           5 * time.Millisecond,
		Timeout:            time.Second,
		Confirmations:      1,
	}
}

// fakeReader serves one hub's copy of the Cluster CR.
type fakeReader struct {
	activeController map[string]interface{}
	err              error
	// sawCreds records the credentials the reader was built with, so a test can
	// prove the endpoint and its token travelled together.
	sawCreds hub.Connection
}

func (f *fakeReader) Get(_ context.Context, _ types.NamespacedName, obj *unstructured.Unstructured) error {
	if f.err != nil {
		return f.err
	}
	obj.Object = map[string]interface{}{}
	if f.activeController != nil {
		_ = unstructured.SetNestedMap(obj.Object, f.activeController, "status", "activeController")
	}
	return nil
}

// declaring builds an activeController naming endpoint.
func declaring(endpoint, identity string) map[string]interface{} {
	return map[string]interface{}{
		"endpoint":       endpoint,
		"activeIdentity": identity,
		"lastUpdated":    time.Now().UTC().Format(time.RFC3339),
	}
}

// readersFor wires a per-endpoint reader table into the readerFor callback,
// recording the connection each reader was constructed from.
func readersFor(table map[string]*fakeReader) func(hub.Connection) (resolver.ClusterReader, error) {
	var mu sync.Mutex
	return func(conn hub.Connection) (resolver.ClusterReader, error) {
		mu.Lock()
		defer mu.Unlock()
		r, ok := table[conn.Endpoint]
		if !ok {
			return nil, fmt.Errorf("no reader for %s", conn.Endpoint)
		}
		r.sawCreds = conn
		return r, nil
	}
}

func newFollower(t *testing.T, cfg Config, table map[string]*fakeReader) *Follower {
	t.Helper()
	f, err := New(cfg, primaryConn(), logr.Discard(), readersFor(table))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return f
}

func TestConfigFromEnv_DisabledWithoutASecondaryHub(t *testing.T) {
	t.Setenv("HUB_SECONDARY_HOST_ENDPOINT", "")
	cfg := ConfigFromEnv()
	assert.False(t, cfg.Enabled(),
		"with no secondary hub there is nothing to resolve, and every existing deployment must stay on this path")
}

func TestConfigFromEnv_DefaultsAndOverrides(t *testing.T) {
	t.Setenv("HUB_SECONDARY_HOST_ENDPOINT", secondaryEndpoint)
	cfg := ConfigFromEnv()
	assert.True(t, cfg.Enabled())
	assert.Equal(t, defaultSecondaryTokenFile, cfg.SecondaryTokenFile)
	assert.Equal(t, defaultInterval, cfg.Interval)
	assert.Equal(t, defaultConfirmations, cfg.Confirmations)

	t.Setenv("HUB_RESOLVE_INTERVAL", "3s")
	t.Setenv("HUB_SWITCH_CONFIRMATIONS", "5")
	cfg = ConfigFromEnv()
	assert.Equal(t, 3*time.Second, cfg.Interval)
	assert.Equal(t, 5, cfg.Confirmations)
}

// TestConfigFromEnv_MalformedValuesFallBack: these are tuning knobs. Refusing to
// start over a typo in one would be a worse outcome than running at the default.
func TestConfigFromEnv_MalformedValuesFallBack(t *testing.T) {
	t.Setenv("HUB_SECONDARY_HOST_ENDPOINT", secondaryEndpoint)
	t.Setenv("HUB_RESOLVE_INTERVAL", "not-a-duration")
	t.Setenv("HUB_SWITCH_CONFIRMATIONS", "-4")
	cfg := ConfigFromEnv()
	assert.Equal(t, defaultInterval, cfg.Interval)
	assert.Equal(t, defaultConfirmations, cfg.Confirmations)
}

func TestNew_Rejects(t *testing.T) {
	_, err := New(Config{}, primaryConn(), logr.Discard(), readersFor(nil))
	assert.Error(t, err, "a disabled config must not produce a follower")

	same := testConfig()
	same.SecondaryEndpoint = primaryEndpoint
	_, err = New(same, primaryConn(), logr.Discard(), readersFor(nil))
	assert.Error(t, err, "two candidates at one endpoint cannot be told apart")
}

// TestStartupConnection_UsesTheWinnersOwnCredentials is the test this whole
// design exists for. The endpoint is read late, inside the client builders, but
// the token and CA paths are package-level vars fixed before main runs — so
// moving the endpoint alone would produce a client pointed at hub B
// authenticating as hub A, which fails in a way that looks like a network fault.
func TestStartupConnection_UsesTheWinnersOwnCredentials(t *testing.T) {
	table := map[string]*fakeReader{
		primaryEndpoint:   {err: fmt.Errorf("simulated: hub A is gone")},
		secondaryEndpoint: {activeController: declaring(secondaryEndpoint, "hub-b-1")},
	}
	f := newFollower(t, testConfig(), table)

	conn, reconnected := f.StartupConnection(context.Background())
	assert.Equal(t, secondaryEndpoint, conn.Endpoint)
	assert.Equal(t, "/creds/b/token", conn.TokenFile, "the winner's token must travel with its endpoint")
	assert.Equal(t, "/creds/b/ca.crt", conn.CAFile, "the winner's CA must travel with its endpoint")
	assert.True(t, reconnected, "a winner other than the primary is a resolved switch")
}

func TestStartupConnection_SteadyStateStaysOnThePrimary(t *testing.T) {
	table := map[string]*fakeReader{
		primaryEndpoint:   {activeController: declaring(primaryEndpoint, "hub-a-1")},
		secondaryEndpoint: {activeController: declaring(primaryEndpoint, "hub-a-1")}, // the mirror
	}
	f := newFollower(t, testConfig(), table)

	conn, reconnected := f.StartupConnection(context.Background())
	assert.Equal(t, primaryConn(), conn, "both hubs naming hub A is agreement, not a conflict")
	assert.False(t, reconnected, "staying on the primary is not a resolved switch")
}

// TestStartupConnection_FallsBackWhenNothingResolves: a worker that refused to
// start because it could not reach a hub would turn a hub outage into a worker
// outage. The primary is the same hub it would have used before any of this.
func TestStartupConnection_FallsBackWhenNothingResolves(t *testing.T) {
	for name, table := range map[string]map[string]*fakeReader{
		"both unreachable": {
			primaryEndpoint:   {err: fmt.Errorf("down")},
			secondaryEndpoint: {err: fmt.Errorf("down")},
		},
		"neither publishes": {
			primaryEndpoint:   {},
			secondaryEndpoint: {},
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFollower(t, testConfig(), table)
			conn, reconnected := f.StartupConnection(context.Background())
			assert.Equal(t, primaryConn(), conn)
			assert.False(t, reconnected, "falling back to the primary is not a resolved switch")
		})
	}
}

func TestWatch_FiresOnceWhenTheActiveHubMoves(t *testing.T) {
	table := map[string]*fakeReader{
		primaryEndpoint:   {err: fmt.Errorf("simulated: hub A died")},
		secondaryEndpoint: {activeController: declaring(secondaryEndpoint, "hub-b-1")},
	}
	f := newFollower(t, testConfig(), table)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var mu sync.Mutex
	var calls []resolver.Claim
	f.Watch(ctx, primaryConn(), func(c resolver.Claim) {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, c)
	})

	mu.Lock()
	defer mu.Unlock()
	if len(calls) != 1 {
		t.Fatalf("expected exactly one switch notification, got %d", len(calls))
	}
	assert.Equal(t, secondaryEndpoint, calls[0].Endpoint)
	assert.Equal(t, "hub-b-1", calls[0].Identity)
}

// TestWatch_QuietWhileTheActiveHubIsUnchanged: the steady state must produce no
// notification at all, or the worker would restart itself on a timer.
func TestWatch_QuietWhileTheActiveHubIsUnchanged(t *testing.T) {
	table := map[string]*fakeReader{
		primaryEndpoint:   {activeController: declaring(primaryEndpoint, "hub-a-1")},
		secondaryEndpoint: {activeController: declaring(primaryEndpoint, "hub-a-1")},
	}
	f := newFollower(t, testConfig(), table)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()

	switched := false
	f.Watch(ctx, primaryConn(), func(resolver.Claim) { switched = true })
	assert.False(t, switched, "an unchanged active hub must never trigger a reconnect")
}

// TestWatch_SilentHubsDoNotTriggerARestart is the non-HA safety net: if the
// field is absent everywhere, the worker must sit still rather than restart.
func TestWatch_SilentHubsDoNotTriggerARestart(t *testing.T) {
	f := newFollower(t, testConfig(), map[string]*fakeReader{
		primaryEndpoint:   {},
		secondaryEndpoint: {},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()

	switched := false
	f.Watch(ctx, primaryConn(), func(resolver.Claim) { switched = true })
	assert.False(t, switched)
}

func TestWatch_StopsWithItsContext(t *testing.T) {
	f := newFollower(t, testConfig(), map[string]*fakeReader{
		primaryEndpoint:   {},
		secondaryEndpoint: {},
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		f.Watch(ctx, primaryConn(), func(resolver.Claim) {})
		close(done)
	}()
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Watch did not return when its context was cancelled")
	}
}
