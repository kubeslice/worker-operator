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
	"testing"

	spokev1alpha1 "github.com/kubeslice/apis/pkg/worker/v1alpha1"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
)

// sampleHubGateway returns a hub WorkerSliceGateway spec with all the fields the
// config propagation reads, with RouteEntireSliceSubnet set as requested.
func sampleHubGateway(route bool) *spokev1alpha1.WorkerSliceGateway {
	gw := &spokev1alpha1.WorkerSliceGateway{}
	gw.Spec.SliceName = "slice"
	gw.Spec.GatewayType = "OpenVPN"
	gw.Spec.GatewayHostType = "Client"
	gw.Spec.GatewayConnectivityType = "NodePort"
	gw.Spec.GatewayProtocol = "UDP"
	gw.Spec.GatewayNumber = 1
	gw.Spec.RouteEntireSliceSubnet = route
	gw.Spec.LocalGatewayConfig = spokev1alpha1.SliceGatewayConfig{
		GatewayName: "gw-1", GatewaySubnet: "10.11.16.0/20", VpnIp: "10.11.255.2",
	}
	gw.Spec.RemoteGatewayConfig = spokev1alpha1.SliceGatewayConfig{
		GatewayName: "gw-2", GatewaySubnet: "10.11.0.0/20", ClusterName: "worker-1", VpnIp: "10.11.255.1",
	}
	return gw
}

// TestNewMeshGatewayConfig_PropagatesRouteEntireSliceSubnet verifies the
// controller-set RouteEntireSliceSubnet flag is copied onto the local
// SliceGateway status config, in both states, along with a couple of other
// fields as a sanity check.
func TestNewMeshGatewayConfig_PropagatesRouteEntireSliceSubnet(t *testing.T) {
	mesh := &kubeslicev1beta1.SliceGateway{}
	for _, route := range []bool{true, false} {
		cfg := newMeshGatewayConfig(mesh, sampleHubGateway(route))
		if cfg.RouteEntireSliceSubnet != route {
			t.Errorf("RouteEntireSliceSubnet: got %v, want %v", cfg.RouteEntireSliceSubnet, route)
		}
		if cfg.SliceName != "slice" {
			t.Errorf("SliceName not propagated: got %q", cfg.SliceName)
		}
		if string(cfg.SliceGatewayType) != "OpenVPN" {
			t.Errorf("SliceGatewayType not propagated: got %q", cfg.SliceGatewayType)
		}
		if cfg.SliceGatewayRemoteSubnet != "10.11.0.0/20" {
			t.Errorf("SliceGatewayRemoteSubnet not propagated: got %q", cfg.SliceGatewayRemoteSubnet)
		}
	}
}

// TestStaticGatewayConfigChanged_DetectsRouteFlag verifies the change-detection
// treats a RouteEntireSliceSubnet flip as a change (so the worker re-syncs when
// the controller toggles the flag), and reports no change when everything
// already matches.
func TestStaticGatewayConfigChanged_DetectsRouteFlag(t *testing.T) {
	hub := sampleHubGateway(true)
	mesh := &kubeslicev1beta1.SliceGateway{}
	// seed the local config to exactly match the hub spec → no change expected
	mesh.Status.Config = newMeshGatewayConfig(mesh, hub)
	if staticGatewayConfigChanged(mesh, hub) {
		t.Fatal("expected no change when local config already matches the hub spec")
	}
	// flip only the flag on the hub spec → must be detected as a change
	hub.Spec.RouteEntireSliceSubnet = false
	if !staticGatewayConfigChanged(mesh, hub) {
		t.Fatal("expected a RouteEntireSliceSubnet flip to be detected as a change")
	}
}
