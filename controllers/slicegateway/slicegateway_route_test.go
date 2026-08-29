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

package slicegateway

import "testing"

func TestRemoteNsmSubnetForRoute(t *testing.T) {
	cases := []struct {
		name                   string
		routeEntireSliceSubnet bool
		gatewayRemoteSubnet    string
		sliceSubnet            string
		wantSubnet             string
		wantReady              bool
	}{
		{
			name:                "full-mesh/normal gateway uses peer gateway subnet",
			gatewayRemoteSubnet: "10.1.1.0/24",
			sliceSubnet:         "10.1.0.0/16",
			wantSubnet:          "10.1.1.0/24",
			wantReady:           true,
		},
		{
			name:                   "spoke-to-hub gateway uses entire slice subnet",
			routeEntireSliceSubnet: true,
			gatewayRemoteSubnet:    "10.1.1.0/24",
			sliceSubnet:            "10.1.0.0/16",
			wantSubnet:             "10.1.0.0/16",
			wantReady:              true,
		},
		{
			name:                   "spoke-to-hub gateway not ready when slice subnet unknown",
			routeEntireSliceSubnet: true,
			gatewayRemoteSubnet:    "10.1.1.0/24",
			sliceSubnet:            "",
			wantSubnet:             "",
			wantReady:              false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			subnet, ready := remoteNsmSubnetForRoute(tc.routeEntireSliceSubnet, tc.gatewayRemoteSubnet, tc.sliceSubnet)
			if subnet != tc.wantSubnet || ready != tc.wantReady {
				t.Fatalf("got (%q, %v), want (%q, %v)", subnet, ready, tc.wantSubnet, tc.wantReady)
			}
		})
	}
}
