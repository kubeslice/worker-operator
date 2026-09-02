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
	"crypto/x509"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"

	hub "github.com/kubeslice/worker-operator/pkg/hub/hubclient"
)

// This file covers worker-operator issue #468's four robustness scenarios,
// at whatever level each one is actually testable — see docs/hub-failover.md
// and pkg/hub/controllers/cluster/conditions_test.go for the rest.

// Scenario 1, clean failover/reconnect: StartupConnection's second return
// value is the only signal main.go has for "did I just follow a switch", and
// it has to be right in both directions, not just the switched-to case
// already covered by TestStartupConnection_UsesTheWinnersOwnCredentials.
func TestRobustness_ReconnectSignalMatchesWhatActuallyHappened(t *testing.T) {
	cases := []struct {
		name          string
		table         map[string]*fakeReader
		wantEndpoint  string
		wantViaSwitch bool
	}{
		{
			name: "resolves to the secondary: a real switch",
			table: map[string]*fakeReader{
				primaryEndpoint:   {err: fmt.Errorf("hub A gone")},
				secondaryEndpoint: {activeController: declaring(secondaryEndpoint, "hub-b-1")},
			},
			wantEndpoint:  secondaryEndpoint,
			wantViaSwitch: true,
		},
		{
			name: "resolves to the primary: not a switch",
			table: map[string]*fakeReader{
				primaryEndpoint:   {activeController: declaring(primaryEndpoint, "hub-a-1")},
				secondaryEndpoint: {activeController: declaring(primaryEndpoint, "hub-a-1")},
			},
			wantEndpoint:  primaryEndpoint,
			wantViaSwitch: false,
		},
		{
			name: "nothing resolves: falls back, not a switch",
			table: map[string]*fakeReader{
				primaryEndpoint:   {},
				secondaryEndpoint: {},
			},
			wantEndpoint:  primaryEndpoint,
			wantViaSwitch: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFollower(t, testConfig(), c.table)
			conn, reconnected := f.StartupConnection(context.Background())
			assert.Equal(t, c.wantEndpoint, conn.Endpoint)
			assert.Equal(t, c.wantViaSwitch, reconnected)
		})
	}
}

// Scenario 2, cert trust validation: a candidate whose probe fails on a bad
// CA must be treated exactly like any other unreachable candidate (the
// worker still falls back safely) while the underlying error is still
// separately classifiable as a certificate problem, not a generic dial
// failure — which is what lets an operator reading the eventual
// CertVerificationFailed reason (see pkg/hub/hubclient.ClassifyConnectionError)
// trust it names the right kind of misconfiguration.
func TestRobustness_CertVerificationFailureIsSafeAndDistinguishable(t *testing.T) {
	certErr := fmt.Errorf("tls: failed to verify certificate: %w", x509.UnknownAuthorityError{})
	table := map[string]*fakeReader{
		primaryEndpoint:   {activeController: declaring(primaryEndpoint, "hub-a-1")},
		secondaryEndpoint: {err: certErr},
	}
	f := newFollower(t, testConfig(), table)

	conn, reconnected := f.StartupConnection(context.Background())
	assert.Equal(t, primaryConn(), conn, "a cert failure on the secondary must not block startup on the primary")
	assert.False(t, reconnected)

	assert.Equal(t, hub.ReasonCertVerificationFailed, hub.ClassifyConnectionError(certErr),
		"the same error a real TLS handshake failure would produce must classify as a cert problem, not a generic dial failure")
}

// Scenario 4, non-HA backward compat: New already refuses to build a
// Follower without a secondary configured (TestNew_Rejects). This pins the
// other half — the zero-value Config used throughout main.go's guarded path
// reports Enabled()==false, so that guard is the only way to ever reach a
// live Follower at all.
func TestRobustness_NonHANeverConstructsAFollower(t *testing.T) {
	var cfg Config
	assert.False(t, cfg.Enabled())
	_, err := New(cfg, primaryConn(), logr.Discard(), nil)
	assert.Error(t, err, "a zero-value Config, exactly what an existing non-HA deployment has, must never yield a live Follower")
}
