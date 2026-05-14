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

package slicegwstatus

import (
	"testing"

	hubv1alpha1 "github.com/kubeslice/apis/pkg/controller/v1alpha1"
	spokev1alpha1 "github.com/kubeslice/apis/pkg/worker/v1alpha1"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
)

func TestWorkerSliceGatewayTargetsCluster(t *testing.T) {
	tests := []struct {
		name      string
		local     string
		reconcile string
		want      bool
	}{
		{
			name:      "match",
			local:     "cluster-east",
			reconcile: "cluster-east",
			want:      true,
		},
		{
			name:      "wrong cluster ignored",
			local:     "cluster-west",
			reconcile: "cluster-east",
			want:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hub := &spokev1alpha1.WorkerSliceGateway{
				Spec: spokev1alpha1.WorkerSliceGatewaySpec{
					LocalGatewayConfig: spokev1alpha1.SliceGatewayConfig{
						ClusterName: tt.local,
					},
				},
			}
			if got := WorkerSliceGatewayTargetsCluster(hub, tt.reconcile); got != tt.want {
				t.Fatalf("WorkerSliceGatewayTargetsCluster() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMeshSliceGatewayStatusOutOfSync(t *testing.T) {
	baseHub := func() *spokev1alpha1.WorkerSliceGateway {
		return &spokev1alpha1.WorkerSliceGateway{
			Spec: spokev1alpha1.WorkerSliceGatewaySpec{
				SliceName: "demo-slice",
				LocalGatewayConfig: spokev1alpha1.SliceGatewayConfig{
					GatewayName:   "gw-local",
					GatewaySubnet: "10.1.0.0/16",
					ClusterName:   "local",
				},
				RemoteGatewayConfig: spokev1alpha1.SliceGatewayConfig{
					GatewayName:   "gw-remote",
					GatewaySubnet: "10.2.0.0/16",
					ClusterName:   "remote",
					NodeIps:       []string{"192.0.2.10"},
					NodePorts:     []int{30001},
				},
				GatewayHostType:         "Client",
				GatewayNumber:           0,
				GatewayConnectivityType: "NodePort",
				GatewayProtocol:         "UDP",
				GatewayType:             hubv1alpha1.SliceGatewayTypeOpenVPN,
			},
		}
	}
	baseMesh := func() *kubeslicev1beta1.SliceGateway {
		return &kubeslicev1beta1.SliceGateway{
			Status: kubeslicev1beta1.SliceGatewayStatus{
				Config: kubeslicev1beta1.SliceGatewayConfig{
					SliceName:                    "demo-slice",
					SliceGatewayID:               "gw-local",
					SliceGatewaySubnet:           "10.1.0.0/16",
					SliceGatewayRemoteSubnet:     "10.2.0.0/16",
					SliceGatewayHostType:         "Client",
					SliceGatewayRemoteClusterID:  "remote",
					SliceGatewayRemoteGatewayID:  "gw-remote",
					SliceGatewayRemoteNodeIPs:    []string{"192.0.2.10"},
					SliceGatewayNodePorts:        []int{30001},
					SliceGatewayName:             "0",
					SliceGatewayConnectivityType: "NodePort",
					SliceGatewayProtocol:         "UDP",
					SliceGatewayType:             hubv1alpha1.SliceGatewayTypeOpenVPN,
				},
			},
		}
	}

	tests := []struct {
		name   string
		mutate func(mesh *kubeslicev1beta1.SliceGateway, hub *spokev1alpha1.WorkerSliceGateway)
		want   bool
	}{
		{
			name: "in sync",
			mutate: func(_ *kubeslicev1beta1.SliceGateway, _ *spokev1alpha1.WorkerSliceGateway) {
			},
			want: false,
		},
		{
			name: "remote cluster id drift",
			mutate: func(mesh *kubeslicev1beta1.SliceGateway, _ *spokev1alpha1.WorkerSliceGateway) {
				mesh.Status.Config.SliceGatewayRemoteClusterID = "other"
			},
			want: true,
		},
		{
			name: "client node ip drift",
			mutate: func(_ *kubeslicev1beta1.SliceGateway, hub *spokev1alpha1.WorkerSliceGateway) {
				hub.Spec.RemoteGatewayConfig.NodeIps = []string{"192.0.2.20"}
			},
			want: true,
		},
		{
			name: "client node port drift",
			mutate: func(_ *kubeslicev1beta1.SliceGateway, hub *spokev1alpha1.WorkerSliceGateway) {
				hub.Spec.RemoteGatewayConfig.NodePorts = []int{30002}
			},
			want: true,
		},
		{
			name: "server ignores remote node fields",
			mutate: func(mesh *kubeslicev1beta1.SliceGateway, hub *spokev1alpha1.WorkerSliceGateway) {
				mesh.Status.Config.SliceGatewayHostType = "Server"
				hub.Spec.GatewayHostType = "Server"
				mesh.Status.Config.SliceGatewayRemoteNodeIPs = []string{"1.1.1.1"}
				hub.Spec.RemoteGatewayConfig.NodeIps = []string{"9.9.9.9"}
				mesh.Status.Config.SliceGatewayNodePorts = []int{111}
				hub.Spec.RemoteGatewayConfig.NodePorts = []int{222}
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mesh := baseMesh()
			hub := baseHub()
			tt.mutate(mesh, hub)
			if got := MeshSliceGatewayStatusOutOfSync(mesh, hub); got != tt.want {
				t.Fatalf("MeshSliceGatewayStatusOutOfSync() = %v, want %v", got, tt.want)
			}
		})
	}
}
