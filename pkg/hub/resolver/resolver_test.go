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
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var (
	hubA = HubCandidate{Name: "primary", Endpoint: "https://hub-a.example:6443"}
	hubB = HubCandidate{Name: "secondary", Endpoint: "https://hub-b.example:6443"}
)

func candidates() []HubCandidate { return []HubCandidate{hubA, hubB} }

// declares builds the verdict a hub gives when it reports someone as Active.
// The declared hub is often not the hub answering: a Standby's mirrored copy
// names the Active, which is the whole basis of the resolution rule.
func declares(answering HubCandidate, about HubCandidate, identity string, age time.Duration) Verdict {
	return Verdict{
		Candidate: answering,
		Reachable: true,
		Claim: &Claim{
			Endpoint:    about.Endpoint,
			Identity:    identity,
			LastUpdated: time.Now().Add(-age),
			Source:      answering.Name,
		},
	}
}

func silent(c HubCandidate) Verdict { return Verdict{Candidate: c, Reachable: true} }
func unreachable(c HubCandidate) Verdict {
	return Verdict{Candidate: c, Err: fmt.Errorf("simulated: connection refused")}
}

// staticProbe answers each candidate from a fixed table.
func staticProbe(verdicts map[string]Verdict) Prober {
	return func(_ context.Context, c HubCandidate) Verdict {
		if v, ok := verdicts[c.Endpoint]; ok {
			return v
		}
		return unreachable(c)
	}
}

func newResolver(t *testing.T, probe Prober, confirmations int) *Resolver {
	t.Helper()
	r, err := New(candidates(), probe, Options{SwitchConfirmations: confirmations})
	mustNoErr(t, err)
	return r
}

// resolveN drives the resolver n times against the same verdicts, which is how
// the confirmation counter is exercised.
func resolveN(r *Resolver, n int) *Claim {
	var last *Claim
	for i := 0; i < n; i++ {
		last = r.Resolve(context.Background())
	}
	return last
}

func TestNew_Validation(t *testing.T) {
	probe := staticProbe(nil)
	_, err := New(nil, probe, Options{})
	assert.Error(t, err, "a resolver with no candidates can never resolve anything")

	_, err = New(candidates(), nil, Options{})
	assert.Error(t, err, "a resolver with no probe can never resolve anything")

	_, err = New([]HubCandidate{{Name: "broken"}}, probe, Options{})
	assert.Error(t, err, "a candidate with no endpoint is unusable")

	_, err = New([]HubCandidate{hubA, {Name: "dup", Endpoint: hubA.Endpoint}}, probe, Options{})
	assert.Error(t, err, "duplicate endpoints would let one hub vote twice in the tie-break")
}

// TestResolve_SteadyState is the ordinary case and the one most easily misread:
// the Active declares itself and the Standby's mirror repeats that same
// declaration, so BOTH hubs answer naming hub A. Two claims about one hub is a
// unanimous result, not a conflict.
func TestResolve_SteadyState(t *testing.T) {
	r := newResolver(t, staticProbe(map[string]Verdict{
		hubA.Endpoint: declares(hubA, hubA, "hub-a-1", 2*time.Second),
		hubB.Endpoint: declares(hubB, hubA, "hub-a-1", 3*time.Second),
	}), 1)

	got := r.Resolve(context.Background())
	mustClaim(t, got, "")
	assert.Equal(t, "hub-a-1", got.Identity)
	assert.Equal(t, hubA.Endpoint, got.Endpoint)
}

// TestResolve_OnlyTheActiveIsReachable covers failover's own shape: the Active
// is gone and only the promoted hub answers, naming itself.
func TestResolve_OnlyTheActiveIsReachable(t *testing.T) {
	r := newResolver(t, staticProbe(map[string]Verdict{
		hubA.Endpoint: unreachable(hubA),
		hubB.Endpoint: declares(hubB, hubB, "hub-b-1", time.Second),
	}), 1)

	got := r.Resolve(context.Background())
	mustClaim(t, got, "")
	assert.Equal(t, "hub-b-1", got.Identity)
	assert.Equal(t, hubB.Endpoint, got.Endpoint)
}

// TestResolve_DisagreementPrefersTheFresherDeclaration is the recovered-old-
// Active case: both hubs are up and each names itself. Not split-brain
// resolution, which is an explicit non-goal — just a single-valued answer.
func TestResolve_DisagreementPrefersTheFresherDeclaration(t *testing.T) {
	r := newResolver(t, staticProbe(map[string]Verdict{
		hubA.Endpoint: declares(hubA, hubA, "hub-a-1", time.Hour),
		hubB.Endpoint: declares(hubB, hubB, "hub-b-1", time.Second),
	}), 1)

	got := r.Resolve(context.Background())
	mustClaim(t, got, "")
	assert.Equal(t, "hub-b-1", got.Identity, "the newer declaration must win")
}

// TestResolve_EqualTimestampsAreDeterministic guards the nastiest edge: if two
// equally-stale conflicting claims resolved differently between polls, the
// confirmation counter could never accumulate and the worker would never settle.
func TestResolve_EqualTimestampsAreDeterministic(t *testing.T) {
	stamp := time.Now().Add(-time.Minute)
	probe := staticProbe(map[string]Verdict{
		hubA.Endpoint: {Candidate: hubA, Reachable: true,
			Claim: &Claim{Endpoint: hubA.Endpoint, Identity: "hub-a-1", LastUpdated: stamp}},
		hubB.Endpoint: {Candidate: hubB, Reachable: true,
			Claim: &Claim{Endpoint: hubB.Endpoint, Identity: "hub-b-1", LastUpdated: stamp}},
	})

	first := newResolver(t, probe, 1).Resolve(context.Background())
	mustClaim(t, first, "")
	for i := 0; i < 20; i++ {
		again := newResolver(t, probe, 1).Resolve(context.Background())
		mustClaim(t, again, "")
		assert.Equal(t, first.Identity, again.Identity,
			"an equal-timestamp tie must resolve the same way every time")
	}
}

// TestResolve_RejectsAnUnconfiguredEndpoint pins the trust boundary. Without
// it, write access to one Cluster CR would be enough to point a worker's hub
// connection at any address at all.
func TestResolve_RejectsAnUnconfiguredEndpoint(t *testing.T) {
	rogue := HubCandidate{Name: "rogue", Endpoint: "https://attacker.example:6443"}
	r := newResolver(t, staticProbe(map[string]Verdict{
		hubA.Endpoint: declares(hubA, rogue, "rogue-1", time.Second),
		hubB.Endpoint: unreachable(hubB),
	}), 1)

	assert.Nil(t, r.Resolve(context.Background()),
		"a claim naming an endpoint nobody configured must never be followed")
}

// TestResolve_NoClaimsLeavesTheCallerAlone covers the two ways a worker ends up
// with nothing to act on. Both must mean "change nothing" rather than
// "disconnect", or a hub blip becomes a worker outage.
func TestResolve_NoClaimsLeavesTheCallerAlone(t *testing.T) {
	t.Run("non-HA hubs publish nothing", func(t *testing.T) {
		r := newResolver(t, staticProbe(map[string]Verdict{
			hubA.Endpoint: silent(hubA),
			hubB.Endpoint: silent(hubB),
		}), 1)
		assert.Nil(t, r.Resolve(context.Background()))
		assert.Nil(t, r.Current(), "the non-HA path must stay a clean no-op")
	})

	t.Run("neither hub is reachable", func(t *testing.T) {
		r := newResolver(t, staticProbe(map[string]Verdict{
			hubA.Endpoint: unreachable(hubA),
			hubB.Endpoint: unreachable(hubB),
		}), 1)
		assert.Nil(t, r.Resolve(context.Background()))
	})
}

// TestResolve_TotalOutageKeepsTheEstablishedWinner is the one that matters most
// operationally: once a worker knows who the Active is, losing sight of both
// hubs must not retract that. Otherwise every network hiccup would tear down a
// working connection.
func TestResolve_TotalOutageKeepsTheEstablishedWinner(t *testing.T) {
	verdicts := map[string]Verdict{
		hubA.Endpoint: declares(hubA, hubA, "hub-a-1", time.Second),
		hubB.Endpoint: declares(hubB, hubA, "hub-a-1", time.Second),
	}
	r := newResolver(t, func(_ context.Context, c HubCandidate) Verdict { return verdicts[c.Endpoint] }, 1)
	mustClaim(t, r.Resolve(context.Background()), "")

	verdicts[hubA.Endpoint] = unreachable(hubA)
	verdicts[hubB.Endpoint] = unreachable(hubB)

	got := r.Resolve(context.Background())
	mustClaim(t, got, "a total outage must not retract a known Active")
	assert.Equal(t, "hub-a-1", got.Identity)
}

// TestResolve_RequiresConsecutiveConfirmations is the anti-flap rule. A single
// divergent poll must not move a worker's hub connection.
func TestResolve_RequiresConsecutiveConfirmations(t *testing.T) {
	verdicts := map[string]Verdict{
		hubA.Endpoint: declares(hubA, hubA, "hub-a-1", time.Second),
		hubB.Endpoint: declares(hubB, hubA, "hub-a-1", time.Second),
	}
	r := newResolver(t, func(_ context.Context, c HubCandidate) Verdict { return verdicts[c.Endpoint] }, 3)

	got := resolveN(r, 1)
	mustClaim(t, got, "the first winner is taken at once; there is no connection to protect yet")
	assert.Equal(t, "hub-a-1", got.Identity)

	// hub B takes over.
	verdicts[hubA.Endpoint] = unreachable(hubA)
	verdicts[hubB.Endpoint] = declares(hubB, hubB, "hub-b-1", time.Second)

	for i := 1; i < 3; i++ {
		got = r.Resolve(context.Background())
		assert.Equal(t, "hub-a-1", got.Identity,
			"poll %d of 3: the switch must not be reported before it is confirmed", i)
	}
	got = r.Resolve(context.Background())
	assert.Equal(t, "hub-b-1", got.Identity, "the switch lands on the confirming poll")
}

// TestResolve_AFlappingChallengerNeverWins pins that confirmations must be
// CONSECUTIVE. A hub that wins every other poll is exactly the instability the
// counter exists to absorb, and a cumulative counter would eventually let it
// through.
func TestResolve_AFlappingChallengerNeverWins(t *testing.T) {
	verdicts := map[string]Verdict{
		hubA.Endpoint: declares(hubA, hubA, "hub-a-1", time.Second),
		hubB.Endpoint: declares(hubB, hubA, "hub-a-1", time.Second),
	}
	r := newResolver(t, func(_ context.Context, c HubCandidate) Verdict { return verdicts[c.Endpoint] }, 3)
	mustClaim(t, resolveN(r, 3), "")

	challenger := map[string]Verdict{
		hubA.Endpoint: unreachable(hubA),
		hubB.Endpoint: declares(hubB, hubB, "hub-b-1", time.Second),
	}
	incumbent := map[string]Verdict{
		hubA.Endpoint: declares(hubA, hubA, "hub-a-1", time.Second),
		hubB.Endpoint: declares(hubB, hubA, "hub-a-1", time.Second),
	}
	for i := 0; i < 10; i++ {
		for k, v := range challenger {
			verdicts[k] = v
		}
		r.Resolve(context.Background())
		for k, v := range incumbent {
			verdicts[k] = v
		}
		got := r.Resolve(context.Background())
		assert.Equal(t, "hub-a-1", got.Identity,
			"an alternating challenger must never accumulate its way to a switch")
	}
}

// TestResolve_RepublicationDoesNotResetConfirmations guards a subtle way the
// anti-flap counter could deadlock: the Active republishes on a timer, so
// LastUpdated differs on every poll. If sameTarget compared it, no challenger
// would ever agree with itself twice and no switch could ever be confirmed.
func TestResolve_RepublicationDoesNotResetConfirmations(t *testing.T) {
	age := 10 * time.Second
	r := newResolver(t, func(_ context.Context, c HubCandidate) Verdict {
		age -= time.Second // every poll sees a fresher declaration
		if c.Endpoint == hubA.Endpoint {
			return unreachable(hubA)
		}
		return declares(hubB, hubB, "hub-b-1", age)
	}, 3)

	got := resolveN(r, 3)
	mustClaim(t, got, "a steadily-republishing hub must still confirm")
	assert.Equal(t, "hub-b-1", got.Identity)
}

// TestResolve_ProbeRespectsContext checks the resolver passes its context down
// and does not swallow cancellation, so a shutting-down worker is not held up
// by a hub that stopped answering.
func TestResolve_ProbeRespectsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	seen := false
	r := newResolver(t, func(probeCtx context.Context, c HubCandidate) Verdict {
		seen = true
		assert.Error(t, probeCtx.Err(), "the caller's context must reach the probe")
		return unreachable(c)
	}, 1)

	assert.Nil(t, r.Resolve(ctx))
	assert.True(t, seen, "the probe must actually have been called")
}

// mustNoErr and mustClaim give the fail-fast behaviour testify's require would,
// without importing it: only testify/assert and testify/mock are vendored here,
// and adding a package to vendor/ for two helpers is not worth the diff.
func mustNoErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func mustClaim(t *testing.T, got *Claim, msg string) *Claim {
	t.Helper()
	if got == nil {
		if msg == "" {
			msg = "expected a resolved active hub, got none"
		}
		t.Fatal(msg)
	}
	return got
}

// TestResolve_FirstWinnerNeedsNoConfirmation is the regression test for a bug
// the live failover demo found. Callers resolve once before opening a
// connection, so if the very first selection also had to clear the confirmation
// threshold, that single call could never reach it and startup resolution would
// silently fall back to the configured primary at any setting above one —
// including when the primary is the hub that just lost leadership.
//
// The threshold guards a *change* of hub. Before anything is established there
// is no connection to protect and nothing to flap between.
func TestResolve_FirstWinnerNeedsNoConfirmation(t *testing.T) {
	for _, confirmations := range []int{1, 2, 5} {
		r := newResolver(t, staticProbe(map[string]Verdict{
			hubA.Endpoint: unreachable(hubA),
			hubB.Endpoint: declares(hubB, hubB, "hub-b-1", time.Second),
		}), confirmations)

		got := r.Resolve(context.Background())
		mustClaim(t, got, "a single startup resolve must produce an answer")
		assert.Equal(t, "hub-b-1", got.Identity,
			"confirmations=%d: startup must not fall back when a hub plainly declares itself", confirmations)
	}
}
