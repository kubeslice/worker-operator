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

package gwsidecar

import (
	"testing"

	sidecar "github.com/kubeslice/gateway-sidecar/pkg/sidecar/sidecarpb"
)

func TestGetTunnelState(t *testing.T) {
	tests := []struct {
		name         string
		tunnelStatus sidecar.TunnelStatusType
		expected     string
	}{
		{
			name:         "tunnel up",
			tunnelStatus: sidecar.TunnelStatusType_GW_TUNNEL_STATE_UP,
			expected:     "UP",
		},
		{
			name:         "tunnel down",
			tunnelStatus: sidecar.TunnelStatusType_GW_TUNNEL_STATE_DOWN,
			expected:     "DOWN",
		},
		{
			name:         "unknown status",
			tunnelStatus: sidecar.TunnelStatusType(999),
			expected:     "UNKNOWN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getTunnelState(tt.tunnelStatus)
			if result != tt.expected {
				t.Errorf("getTunnelState() = %v, want %v", result, tt.expected)
			}
		})
	}
}
