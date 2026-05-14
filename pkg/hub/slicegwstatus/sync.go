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

// Package slicegwstatus holds pure helpers for hub WorkerSliceGateway vs spoke SliceGateway status sync (issue #470 / hub-and-spoke peering groundwork).
package slicegwstatus

import (
	"strconv"

	spokev1alpha1 "github.com/kubeslice/apis/pkg/worker/v1alpha1"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	hubutils "github.com/kubeslice/worker-operator/pkg/hub"
)

// WorkerSliceGatewayTargetsCluster returns true when the hub WorkerSliceGateway is intended for this worker cluster.
func WorkerSliceGatewayTargetsCluster(hubGW *spokev1alpha1.WorkerSliceGateway, localClusterName string) bool {
	return hubGW.Spec.LocalGatewayConfig.ClusterName == localClusterName
}

// MeshSliceGatewayStatusOutOfSync returns true when the spoke SliceGateway status.Config should be refreshed from the hub WorkerSliceGateway (static + client remote node fields).
func MeshSliceGatewayStatusOutOfSync(mesh *kubeslicev1beta1.SliceGateway, hub *spokev1alpha1.WorkerSliceGateway) bool {
	if mesh.Status.Config.SliceGatewayID != hub.Spec.LocalGatewayConfig.GatewayName ||
		mesh.Status.Config.SliceGatewaySubnet != hub.Spec.LocalGatewayConfig.GatewaySubnet ||
		mesh.Status.Config.SliceGatewayRemoteSubnet != hub.Spec.RemoteGatewayConfig.GatewaySubnet ||
		mesh.Status.Config.SliceGatewayHostType != hub.Spec.GatewayHostType ||
		mesh.Status.Config.SliceGatewayRemoteClusterID != hub.Spec.RemoteGatewayConfig.ClusterName ||
		mesh.Status.Config.SliceGatewayRemoteGatewayID != hub.Spec.RemoteGatewayConfig.GatewayName ||
		mesh.Status.Config.SliceGatewayName != strconv.Itoa(hub.Spec.GatewayNumber) ||
		mesh.Status.Config.SliceGatewayConnectivityType != hub.Spec.GatewayConnectivityType ||
		mesh.Status.Config.SliceGatewayProtocol != hub.Spec.GatewayProtocol ||
		mesh.Status.Config.SliceGatewayType != hub.Spec.GatewayType {
		return true
	}
	if mesh.Status.Config.SliceGatewayHostType == "Client" {
		if !hubutils.ListEqual(mesh.Status.Config.SliceGatewayRemoteNodeIPs, hub.Spec.RemoteGatewayConfig.NodeIps) {
			return true
		}
		if !hubutils.ListEqual(mesh.Status.Config.SliceGatewayNodePorts, hub.Spec.RemoteGatewayConfig.NodePorts) {
			return true
		}
	}
	return false
}
