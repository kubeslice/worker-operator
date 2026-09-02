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

// Package resolver decides which of a worker's two pre-provisioned hub clusters
// is currently the Active one, so the worker can follow a controller failover
// without manual intervention.
//
// A worker cannot learn about a failover from the hub that just failed: the
// promotion is recorded on the *other* hub. So each hub publishes
// status.activeController on this worker's own Cluster CR while it holds
// leadership, and a Standby's mirrored copy repeats the Active's declaration.
// This package reads that field from both endpoints and applies one rule to
// decide who to talk to.
//
// It resolves; it does not connect. Nothing here opens or closes the worker's
// hub connections — that is the caller's job, and keeping the two apart is what
// makes this testable without a cluster. See worker-operator issue #467 and
// kubeslice-controller issue #297.
package resolver

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/go-logr/logr"
)

const (
	// DefaultProbeTimeout bounds a single read of one hub's copy of the Cluster
	// CR. Every networked read here is bounded, and not as a formality: an API
	// server that accepts a connection and then stops answering (a powered-off
	// node, a partition dropping packets) leaves an unbounded read hanging until
	// the OS TCP timeout, which is minutes. The kubeslice-controller side of
	// this feature shipped that bug and measured a single read blocking for ~12
	// seconds against a stopped API server before the connection broke.
	DefaultProbeTimeout = 5 * time.Second

	// DefaultSwitchConfirmations is how many consecutive polls must agree on a
	// new winner before Resolve reports the switch. A worker that re-points its
	// hub connection on a single divergent poll would flap through every
	// transient blip; requiring agreement costs one poll interval of latency and
	// removes that entire class of behaviour.
	DefaultSwitchConfirmations = 2
)

// HubCandidate is one pre-provisioned hub endpoint and the credentials to reach
// it. The set of candidates is fixed at startup and never derived from cluster
// state — that is what makes it a trust boundary rather than a suggestion.
type HubCandidate struct {
	// Name is a stable label for logs and metrics ("primary", "secondary").
	// It has no relationship to which hub is currently Active.
	Name      string
	Endpoint  string
	TokenFile string
	CAFile    string
}

// Claim is one hub's statement about which controller currently holds
// leadership. It is the decoded form of status.activeController.
type Claim struct {
	// Endpoint is where the Active hub says it is reachable.
	Endpoint string
	// Identity is the Active hub's leader identity, stable across its restarts.
	Identity string
	// LastUpdated is when the Active last refreshed the declaration. It exists
	// so two conflicting claims can be ordered; see Resolve.
	LastUpdated time.Time
	// Source names the candidate this claim was read from, which is not
	// necessarily the hub the claim is about — a Standby's mirrored copy names
	// the Active.
	Source string
}

// Verdict is the outcome of probing one candidate.
type Verdict struct {
	Candidate HubCandidate
	// Reachable reports whether the probe got an answer at all.
	Reachable bool
	// Claim is nil when the hub answered but published nothing. That is the
	// ordinary non-HA case, not a failure.
	Claim *Claim
	Err   error
}

// Prober reads one candidate's copy of this worker's Cluster CR. It is a field
// on Resolver rather than a hardcoded call so the decision logic can be tested
// without a cluster, and so a caller can supply a client built however it likes.
type Prober func(ctx context.Context, candidate HubCandidate) Verdict

// Options configures a Resolver. Zero-valued fields fall back to the Default*
// constants.
type Options struct {
	// SwitchConfirmations is how many consecutive agreeing polls are required
	// before a change of winner is reported.
	//
	// There is deliberately no timeout here: a candidate read is bounded by
	// ProbeConfig.Timeout, which belongs to the Prober the caller supplies.
	// This struct used to carry a ProbeTimeout that New never read, which
	// advertised a knob that did nothing.
	SwitchConfirmations int
	Log                 logr.Logger
}

// Resolver applies the active-hub selection rule across a fixed candidate set.
// It is not safe for concurrent use: it keeps the confirmation counter that
// suppresses flapping, and it is meant to be driven by a single polling loop.
type Resolver struct {
	candidates []HubCandidate
	probe      Prober

	confirmations int
	log           logr.Logger

	// current is the winner Resolve last reported, and pending/pendingCount are
	// the challenger accumulating confirmations against it.
	current      *Claim
	pending      *Claim
	pendingCount int
}

// New builds a Resolver over a fixed candidate set.
func New(candidates []HubCandidate, probe Prober, opts Options) (*Resolver, error) {
	if len(candidates) == 0 {
		return nil, fmt.Errorf("resolver requires at least one hub candidate")
	}
	if probe == nil {
		return nil, fmt.Errorf("resolver requires a probe function")
	}
	seen := make(map[string]struct{}, len(candidates))
	for _, c := range candidates {
		if c.Endpoint == "" {
			return nil, fmt.Errorf("hub candidate %q has no endpoint", c.Name)
		}
		if _, dup := seen[c.Endpoint]; dup {
			return nil, fmt.Errorf("duplicate hub candidate endpoint %q", c.Endpoint)
		}
		seen[c.Endpoint] = struct{}{}
	}
	if opts.SwitchConfirmations <= 0 {
		opts.SwitchConfirmations = DefaultSwitchConfirmations
	}
	// A zero logr.Logger has a nil sink and panics on first use, so an Options
	// literal that simply omits Log would take the worker down on the first
	// poll rather than at construction.
	if opts.Log.GetSink() == nil {
		opts.Log = logr.Discard()
	}
	return &Resolver{
		candidates:    candidates,
		probe:         probe,
		confirmations: opts.SwitchConfirmations,
		log:           opts.Log,
	}, nil
}

// Current returns the winner most recently confirmed, or nil if none ever has
// been. It never blocks and performs no I/O.
func (r *Resolver) Current() *Claim {
	return r.current
}

// Resolve probes every candidate once and returns the hub the worker should be
// talking to: the winner confirmed on this poll if there is one, and otherwise
// the last winner confirmed on any earlier poll.
//
// It therefore returns nil only until the first confirmation ever happens, and
// a non-nil result is not by itself news — every poll after the first returns
// something, usually the same claim as last time. Callers compare it against
// the hub they are already connected to rather than treating each answer as a
// switch (StartupConnection and Watch in the failover package both do).
//
// Falling back to the previous winner is a real answer, not an error: with no
// usable claim this poll, the correct behaviour is to leave the existing
// connection alone. A worker that disconnected whenever it was unsure would
// turn every hub blip into a worker outage, which is strictly worse than
// talking to a hub that might be stale.
func (r *Resolver) Resolve(ctx context.Context) *Claim {
	claims := r.gather(ctx)
	winner := r.pick(claims)
	if winner == nil {
		// Deliberately does NOT reset the pending challenger. A single poll in
		// which both hubs happen to be unreachable should not undo confirmations
		// already accumulated; only a competing claim displaces one.
		return r.current
	}
	return r.confirm(winner)
}

// gather probes every candidate and returns the claims worth considering.
func (r *Resolver) gather(ctx context.Context) []Claim {
	var claims []Claim
	for _, candidate := range r.candidates {
		verdict := r.probe(ctx, candidate)
		switch {
		case !verdict.Reachable:
			r.log.V(1).Info("hub candidate unreachable", "hub", candidate.Name,
				"endpoint", candidate.Endpoint, "error", verdict.Err)
		case verdict.Err != nil:
			// Reachable but the read still failed — NewProbe's NotFound case:
			// the hub answered and this worker has no Cluster CR there. That is
			// a misconfiguration (wrong cluster name or namespace, or RBAC on
			// that hub), not the silent non-HA path below, and folding the two
			// together dropped the error without trace. Still V(1), because it
			// is also what a Standby looks like before it has mirrored this
			// worker's CR; a persistent one is meant to be found through
			// kubeslice_worker_hub_probe_errors_total, which counts it.
			r.log.V(1).Info("hub candidate answered but its copy of this worker's Cluster CR could not be read",
				"hub", candidate.Name, "endpoint", candidate.Endpoint, "error", verdict.Err)
		case verdict.Claim == nil:
			// The hub answered and published nothing. This is what a non-HA
			// deployment looks like from here, and it must stay silent at info
			// level or every worker in every existing cluster logs a warning
			// forever.
			r.log.V(1).Info("hub candidate published no activeController", "hub", candidate.Name)
		case !r.known(verdict.Claim.Endpoint):
			// The trust boundary. The field chooses among endpoints an operator
			// pre-provisioned; it does not get to point this worker at an
			// address nobody configured. Anything else would make write access
			// to one Cluster CR enough to redirect a worker's hub connection.
			r.log.Info("ignoring activeController naming an unconfigured endpoint",
				"hub", candidate.Name, "declaredEndpoint", verdict.Claim.Endpoint,
				"declaredIdentity", verdict.Claim.Identity)
		default:
			claims = append(claims, *verdict.Claim)
		}
	}
	return claims
}

// known reports whether endpoint belongs to a configured candidate.
func (r *Resolver) known(endpoint string) bool {
	for _, c := range r.candidates {
		if c.Endpoint == endpoint {
			return true
		}
	}
	return false
}

// pick reduces the surviving claims to at most one winner.
//
// Agreement is the normal case, not the exception, and misreading that is the
// easiest way to get this wrong: in steady state the Active declares itself and
// the Standby's mirrored copy repeats the same declaration, so both hubs
// answer with the same identity. Two claims naming one hub is one winner.
//
// Disagreement means two hubs each believe they are Active — most plausibly an
// old Active that came back after a promotion and still names itself. Split
// brain is an explicit non-goal of the design and this does not resolve it; it
// only keeps the worker's behaviour single-valued while it lasts, by preferring
// the fresher declaration.
func (r *Resolver) pick(claims []Claim) *Claim {
	if len(claims) == 0 {
		return nil
	}
	distinct := make(map[string]struct{}, len(claims))
	for _, c := range claims {
		distinct[c.Identity] = struct{}{}
	}
	if len(distinct) == 1 {
		winner := claims[0]
		return &winner
	}

	// Sorted rather than max-scanned so the outcome is total and repeatable:
	// identical LastUpdated values must not resolve differently between polls,
	// or the anti-flap counter below could never accumulate.
	sorted := make([]Claim, len(claims))
	copy(sorted, claims)
	sort.SliceStable(sorted, func(i, j int) bool {
		if !sorted[i].LastUpdated.Equal(sorted[j].LastUpdated) {
			return sorted[i].LastUpdated.After(sorted[j].LastUpdated)
		}
		return sorted[i].Identity < sorted[j].Identity
	})
	r.log.Info("hubs disagree about which controller is active; preferring the freshest declaration",
		"chosen", sorted[0].Identity, "chosenLastUpdated", sorted[0].LastUpdated,
		"rejected", sorted[1].Identity, "rejectedLastUpdated", sorted[1].LastUpdated)
	winner := sorted[0]
	return &winner
}

// confirm applies the anti-flap rule and returns the claim the caller should
// act on: either the established winner, or a challenger that has now agreed
// with itself enough times to replace it.
func (r *Resolver) confirm(winner *Claim) *Claim {
	// The confirmation rule guards a *change* of hub: it exists so a single
	// divergent poll cannot re-point a worker that is already connected
	// somewhere. With nothing established yet there is no connection to
	// protect and nothing to flap between, so the first winner is taken as it
	// stands.
	//
	// Requiring confirmations here instead makes startup resolution useless at
	// any setting above one: the caller resolves once before opening a
	// connection, that single call can never reach the threshold, and the
	// worker falls back to its configured primary every time — including when
	// the primary is the hub that just lost leadership. Found by running the
	// failover demo, where startup logged "not yet confirmed, seen 1 need 2"
	// and then ignored a correctly resolved Active.
	if r.current == nil {
		r.current = winner
		r.pending = nil
		r.pendingCount = 0
		r.log.Info("active hub resolved", "identity", winner.Identity,
			"endpoint", winner.Endpoint, "previous", "<none>")
		return r.current
	}
	if sameTarget(r.current, winner) {
		// Re-confirmation of the status quo clears any half-accumulated
		// challenger; a challenger has to win consecutive polls, not cumulative
		// ones.
		r.pending = nil
		r.pendingCount = 0
		return r.current
	}

	if r.pending != nil && sameTarget(r.pending, winner) {
		r.pendingCount++
	} else {
		r.pending = winner
		r.pendingCount = 1
	}

	if r.pendingCount < r.confirmations {
		r.log.Info("candidate active hub not yet confirmed; keeping the current one",
			"candidate", winner.Identity, "seen", r.pendingCount, "need", r.confirmations)
		return r.current
	}

	previous := "<none>"
	if r.current != nil {
		previous = r.current.Identity
	}
	r.current = winner
	r.pending = nil
	r.pendingCount = 0
	r.log.Info("active hub resolved", "identity", winner.Identity,
		"endpoint", winner.Endpoint, "previous", previous)
	return r.current
}

// sameTarget reports whether two claims name the same hub. Compared on identity
// and endpoint only: LastUpdated advances on every republication and would make
// every claim differ from itself, so including it would reset the confirmation
// counter on every poll and no switch would ever be confirmed.
func sameTarget(a, b *Claim) bool {
	return a.Identity == b.Identity && a.Endpoint == b.Endpoint
}
