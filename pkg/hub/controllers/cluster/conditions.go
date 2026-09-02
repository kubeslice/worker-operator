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

package cluster

import (
	"context"

	hubv1alpha1 "github.com/kubeslice/apis/pkg/controller/v1alpha1"
	ossEvents "github.com/kubeslice/worker-operator/events"
	"github.com/kubeslice/worker-operator/pkg/utils"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Condition types and reasons for issue #469. Reason strings are shared with
// pkg/hub/hubclient.ClassifyConnectionError where they overlap
// (DialFailed/CertVerificationFailed) so a caller wiring one into the other
// cannot typo a mismatch.
const (
	ConditionControllerConnected      = "ControllerConnected"
	ConditionControllerEndpointSynced = "ControllerEndpointSynced"

	ReasonConnected                = "Connected"
	ReasonReconnectedAfterFailover = "ReconnectedAfterFailover"
	ReasonEndpointNotConfigured    = "EndpointNotConfigured"
	ReasonReconnecting             = "Reconnecting"
	ReasonEndpointUpToDate         = "EndpointUpToDate"
)

// ConnectionInfo is what main.go already knows about this worker's hub
// connection before the hub manager starts: whether failover-following is
// configured at all, and whether the connection now in use was reached by
// following a resolved switch rather than the configured primary. It is a
// startup-time fact, not a live signal.
type ConnectionInfo struct {
	Enabled     bool
	Reconnected bool
}

// setConnectionConditions records this worker's hub-connection health on its
// own Cluster CR, via the standard idempotent SetStatusCondition helper (a
// repeated no-op call never bumps LastTransitionTime).
//
// This only ever sets Connected, ReconnectedAfterFailover or
// EndpointNotConfigured — never DialFailed or CertVerificationFailed. Those
// two are real reason values (see pkg/hub/hubclient.ClassifyConnectionError,
// unit-tested there) but this method only runs because Reconcile just read cr
// successfully: a hub this worker cannot reach is a hub it cannot write
// "unreachable" to either, on that same hub's own copy of the object. That is
// the same conclusion issue #467 reached before deferring the durable surface
// here. kubeslice_worker_hub_probe_errors_total and logs remain the live
// down-detection channel; this condition is the durable, hub-side-visible
// record of the connection's last known good state.
func (r *Reconciler) setConnectionConditions(ctx context.Context, cr *hubv1alpha1.Cluster) {
	connected := metav1.Condition{
		Type:    ConditionControllerConnected,
		Status:  metav1.ConditionTrue,
		Reason:  ReasonConnected,
		Message: "worker is reconciling against this hub",
	}
	synced := metav1.Condition{
		Type:    ConditionControllerEndpointSynced,
		Status:  metav1.ConditionTrue,
		Reason:  ReasonEndpointUpToDate,
		Message: "connected to the currently resolved active hub",
	}

	switch {
	case !r.connInfo.Enabled:
		connected.Status = metav1.ConditionUnknown
		connected.Reason = ReasonEndpointNotConfigured
		connected.Message = "no secondary hub configured; failover-following is inactive"
		synced.Status = metav1.ConditionUnknown
		synced.Reason = ReasonEndpointNotConfigured
		synced.Message = "no secondary hub configured; failover-following is inactive"
	case r.connInfo.Reconnected && !r.reportedReconnect:
		connected.Reason = ReasonReconnectedAfterFailover
		connected.Message = "resumed reconciliation after following a resolved hub failover"
	}

	changed := meta.SetStatusCondition(&cr.Status.Conditions, connected)
	meta.SetStatusCondition(&cr.Status.Conditions, synced)

	if r.connInfo.Reconnected && !r.reportedReconnect {
		utils.RecordEvent(ctx, r.EventRecorder, cr, nil, ossEvents.EventControllerEndpointChanged, controllerName)
		r.reportedReconnect = true
	}
	if changed && connected.Reason == ReasonConnected {
		utils.RecordEvent(ctx, r.EventRecorder, cr, nil, ossEvents.EventControllerConnected, controllerName)
	}
}
