/*
 *  Copyright (c) 2026 Avesha, Inc. All rights reserved.
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

package controllers

import (
	"context"
	"time"

	spokev1alpha1 "github.com/kubeslice/apis/pkg/worker/v1alpha1"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// tunnelStateUp is the value gateway-sidecar reports (via getTunnelState) on a
// gateway pod whose tunnel is established. It mirrors the "UP" string set on
// SliceGateway.Status.GatewayPodStatus[].TunnelStatus.TunnelState.
const tunnelStateUp = "UP"

// gatewayStatusRefreshInterval is how often the hub reconciler re-checks the
// local SliceGateway tunnel status and reports it up, since it does not watch
// the mesh cluster directly.
const gatewayStatusRefreshInterval = 30 * time.Second

// deriveGatewayConnectionState aggregates the per-pod tunnel states of a local
// SliceGateway into a single WorkerSliceGateway connection state. It is HA-aware:
// the gateway is Connected when at least one pod's tunnel is up, NotConnected
// when all pods are down, and Pending when no pod status has been reported yet.
func deriveGatewayConnectionState(pods []*kubeslicev1beta1.GwPodInfo) string {
	if len(pods) == 0 {
		return spokev1alpha1.GatewayConnectionStatePending
	}
	for _, pod := range pods {
		if pod != nil && pod.TunnelStatus.TunnelState == tunnelStateUp {
			return spokev1alpha1.GatewayConnectionStateConnected
		}
	}
	return spokev1alpha1.GatewayConnectionStateNotConnected
}

// reconcileGatewayConnectionStatus derives the gateway's connection state from
// the local SliceGateway's pod tunnel status and, when it has changed, writes it
// to the WorkerSliceGateway.status on the hub so the controller can aggregate
// slice-level topology convergence. The write is guarded against conflicts by
// re-fetching the latest object and retrying.
func (r *SliceGwReconciler) reconcileGatewayConnectionStatus(ctx context.Context, sliceGw *spokev1alpha1.WorkerSliceGateway, meshSliceGw *kubeslicev1beta1.SliceGateway) error {
	state := deriveGatewayConnectionState(meshSliceGw.Status.GatewayPodStatus)
	if sliceGw.Status.ConnectionState == state {
		return nil
	}
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latest := &spokev1alpha1.WorkerSliceGateway{}
		if err := r.Get(ctx, client.ObjectKey{Name: sliceGw.Name, Namespace: sliceGw.Namespace}, latest); err != nil {
			return err
		}
		if latest.Status.ConnectionState == state {
			return nil
		}
		now := metav1.Now()
		latest.Status.ConnectionState = state
		latest.Status.LastTransitionTime = &now
		return r.Status().Update(ctx, latest)
	})
}
