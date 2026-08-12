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

package slicegateway

import (
	"testing"

	gwsidecarpb "github.com/kubeslice/gateway-sidecar/pkg/sidecar/sidecarpb"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	webhook "github.com/kubeslice/worker-operator/pkg/webhook/pod"
	"github.com/stretchr/testify/assert"
)

func TestIsClient(t *testing.T) {
	tests := []struct {
		name     string
		sliceGw  *kubeslicev1beta1.SliceGateway
		expected bool
	}{
		{
			name: "Client gateway",
			sliceGw: &kubeslicev1beta1.SliceGateway{
				Status: kubeslicev1beta1.SliceGatewayStatus{
					Config: kubeslicev1beta1.SliceGatewayConfig{
						SliceGatewayHostType: "Client",
					},
				},
			},
			expected: true,
		},
		{
			name: "Server gateway",
			sliceGw: &kubeslicev1beta1.SliceGateway{
				Status: kubeslicev1beta1.SliceGatewayStatus{
					Config: kubeslicev1beta1.SliceGatewayConfig{
						SliceGatewayHostType: "Server",
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isClient(tt.sliceGw)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsServer(t *testing.T) {
	tests := []struct {
		name     string
		sliceGw  *kubeslicev1beta1.SliceGateway
		expected bool
	}{
		{
			name: "Server gateway",
			sliceGw: &kubeslicev1beta1.SliceGateway{
				Status: kubeslicev1beta1.SliceGatewayStatus{
					Config: kubeslicev1beta1.SliceGatewayConfig{
						SliceGatewayHostType: "Server",
					},
				},
			},
			expected: true,
		},
		{
			name: "Client gateway",
			sliceGw: &kubeslicev1beta1.SliceGateway{
				Status: kubeslicev1beta1.SliceGatewayStatus{
					Config: kubeslicev1beta1.SliceGatewayConfig{
						SliceGatewayHostType: "Client",
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isServer(tt.sliceGw)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetPodType(t *testing.T) {
	tests := []struct {
		name     string
		labels   map[string]string
		expected string
	}{
		{
			name: "Pod with inject label",
			labels: map[string]string{
				webhook.PodInjectLabelKey: "slicegateway",
			},
			expected: "slicegateway",
		},
		{
			name: "NSM nsmgr-daemonset",
			labels: map[string]string{
				"app": "nsmgr-daemonset",
			},
			expected: "nsm",
		},
		{
			name: "NSM kernel-plane",
			labels: map[string]string{
				"app": "nsm-kernel-plane",
			},
			expected: "nsm",
		},
		{
			name:     "No matching labels",
			labels:   map[string]string{"other": "value"},
			expected: "",
		},
		{
			name:     "Empty labels",
			labels:   map[string]string{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getPodType(tt.labels)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetGwSvcNameFromDepName(t *testing.T) {
	tests := []struct {
		name     string
		depName  string
		expected string
	}{
		{
			name:     "Simple deployment name",
			depName:  "my-gateway",
			expected: "svc-my-gateway",
		},
		{
			name:     "Empty deployment name",
			depName:  "",
			expected: "svc-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getGwSvcNameFromDepName(tt.depName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		element  string
		expected bool
	}{
		{
			name:     "Element exists",
			slice:    []string{"a", "b", "c"},
			element:  "b",
			expected: true,
		},
		{
			name:     "Element does not exist",
			slice:    []string{"a", "b", "c"},
			element:  "d",
			expected: false,
		},
		{
			name:     "Empty slice",
			slice:    []string{},
			element:  "a",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.slice, tt.element)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContainsWithIndex(t *testing.T) {
	tests := []struct {
		name          string
		slice         []int
		element       int
		expectedFound bool
		expectedIndex int
	}{
		{
			name:          "Element exists at index 1",
			slice:         []int{10, 20, 30},
			element:       20,
			expectedFound: true,
			expectedIndex: 1,
		},
		{
			name:          "Element does not exist",
			slice:         []int{10, 20, 30},
			element:       40,
			expectedFound: false,
			expectedIndex: 0,
		},
		{
			name:          "Empty slice",
			slice:         []int{},
			element:       10,
			expectedFound: false,
			expectedIndex: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found, index := containsWithIndex(tt.slice, tt.element)
			assert.Equal(t, tt.expectedFound, found)
			assert.Equal(t, tt.expectedIndex, index)
		})
	}
}

func TestGetPodIPs(t *testing.T) {
	tests := []struct {
		name     string
		sliceGw  *kubeslicev1beta1.SliceGateway
		expected []string
	}{
		{
			name: "Multiple pods with IPs",
			sliceGw: &kubeslicev1beta1.SliceGateway{
				Status: kubeslicev1beta1.SliceGatewayStatus{
					GatewayPodStatus: []*kubeslicev1beta1.GwPodInfo{
						{PodIP: "10.0.0.1"},
						{PodIP: "10.0.0.2"},
					},
				},
			},
			expected: []string{"10.0.0.1", "10.0.0.2"},
		},
		{
			name: "No pods",
			sliceGw: &kubeslicev1beta1.SliceGateway{
				Status: kubeslicev1beta1.SliceGatewayStatus{
					GatewayPodStatus: []*kubeslicev1beta1.GwPodInfo{},
				},
			},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getPodIPs(tt.sliceGw)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetPodNames(t *testing.T) {
	tests := []struct {
		name     string
		sliceGw  *kubeslicev1beta1.SliceGateway
		expected []string
	}{
		{
			name: "Multiple pods with names",
			sliceGw: &kubeslicev1beta1.SliceGateway{
				Status: kubeslicev1beta1.SliceGatewayStatus{
					GatewayPodStatus: []*kubeslicev1beta1.GwPodInfo{
						{PodName: "pod-1"},
						{PodName: "pod-2"},
					},
				},
			},
			expected: []string{"pod-1", "pod-2"},
		},
		{
			name: "No pods",
			sliceGw: &kubeslicev1beta1.SliceGateway{
				Status: kubeslicev1beta1.SliceGatewayStatus{
					GatewayPodStatus: []*kubeslicev1beta1.GwPodInfo{},
				},
			},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getPodNames(tt.sliceGw)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetDepNameFromPodName(t *testing.T) {
	tests := []struct {
		name      string
		sliceGwID string
		podName   string
		expected  string
	}{
		{
			name:      "Valid pod name",
			sliceGwID: "slice-gw",
			podName:   "slice-gw-0-1-abc123",
			expected:  "slice-gw-0-1",
		},
		{
			name:      "Empty slice gateway ID",
			sliceGwID: "",
			podName:   "slice-gw-0-1-abc123",
			expected:  "",
		},
		{
			name:      "Empty pod name",
			sliceGwID: "slice-gw",
			podName:   "",
			expected:  "",
		},
		{
			name:      "Pod name without prefix",
			sliceGwID: "slice-gw",
			podName:   "other-pod-name",
			expected:  "",
		},
		{
			name:      "Pod name with insufficient parts",
			sliceGwID: "slice-gw",
			podName:   "slice-gw-0",
			expected:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetDepNameFromPodName(tt.sliceGwID, tt.podName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFindGwPodInfo(t *testing.T) {
	tests := []struct {
		name         string
		gwPodStatus  []*kubeslicev1beta1.GwPodInfo
		podName      string
		expectedName string
		expectNil    bool
	}{
		{
			name: "Pod found",
			gwPodStatus: []*kubeslicev1beta1.GwPodInfo{
				{PodName: "pod-1"},
				{PodName: "pod-2"},
			},
			podName:      "pod-2",
			expectedName: "pod-2",
			expectNil:    false,
		},
		{
			name: "Pod not found",
			gwPodStatus: []*kubeslicev1beta1.GwPodInfo{
				{PodName: "pod-1"},
			},
			podName:   "pod-3",
			expectNil: true,
		},
		{
			name:        "Empty pod status",
			gwPodStatus: []*kubeslicev1beta1.GwPodInfo{},
			podName:     "pod-1",
			expectNil:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findGwPodInfo(tt.gwPodStatus, tt.podName)
			if tt.expectNil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tt.expectedName, result.PodName)
			}
		})
	}
}

func TestGetPeerGwPodName(t *testing.T) {
	tests := []struct {
		name        string
		gwPodName   string
		sliceGw     *kubeslicev1beta1.SliceGateway
		expected    string
		expectError bool
	}{
		{
			name:      "Valid peer pod with tunnel up",
			gwPodName: "pod-1",
			sliceGw: &kubeslicev1beta1.SliceGateway{
				Status: kubeslicev1beta1.SliceGatewayStatus{
					GatewayPodStatus: []*kubeslicev1beta1.GwPodInfo{
						{
							PodName:     "pod-1",
							PeerPodName: "peer-pod-1",
							TunnelStatus: kubeslicev1beta1.TunnelStatus{
								Status: int32(gwsidecarpb.TunnelStatusType_GW_TUNNEL_STATE_UP),
							},
						},
					},
				},
			},
			expected:    "peer-pod-1",
			expectError: false,
		},
		{
			name:      "Pod not found",
			gwPodName: "pod-2",
			sliceGw: &kubeslicev1beta1.SliceGateway{
				Status: kubeslicev1beta1.SliceGatewayStatus{
					GatewayPodStatus: []*kubeslicev1beta1.GwPodInfo{
						{PodName: "pod-1"},
					},
				},
			},
			expected:    "",
			expectError: true,
		},
		{
			name:      "Tunnel down",
			gwPodName: "pod-1",
			sliceGw: &kubeslicev1beta1.SliceGateway{
				Status: kubeslicev1beta1.SliceGatewayStatus{
					GatewayPodStatus: []*kubeslicev1beta1.GwPodInfo{
						{
							PodName: "pod-1",
							TunnelStatus: kubeslicev1beta1.TunnelStatus{
								Status: int32(gwsidecarpb.TunnelStatusType_GW_TUNNEL_STATE_DOWN),
							},
						},
					},
				},
			},
			expected:    "",
			expectError: true,
		},
		{
			name:      "Peer pod name empty",
			gwPodName: "pod-1",
			sliceGw: &kubeslicev1beta1.SliceGateway{
				Status: kubeslicev1beta1.SliceGatewayStatus{
					GatewayPodStatus: []*kubeslicev1beta1.GwPodInfo{
						{
							PodName:     "pod-1",
							PeerPodName: "",
							TunnelStatus: kubeslicev1beta1.TunnelStatus{
								Status: int32(gwsidecarpb.TunnelStatusType_GW_TUNNEL_STATE_UP),
							},
						},
					},
				},
			},
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetPeerGwPodName(tt.gwPodName, tt.sliceGw)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestGetRemoteDepName(t *testing.T) {
	tests := []struct {
		name         string
		remoteGwID   string
		localDepName string
		expected     string
	}{
		{
			name:         "Valid deployment name",
			remoteGwID:   "remote-gw",
			localDepName: "local-gw-0-1",
			expected:     "remote-gw-0-1",
		},
		{
			name:         "Another valid case",
			remoteGwID:   "remote",
			localDepName: "local-5-10",
			expected:     "remote-5-10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetRemoteDepName(tt.remoteGwID, tt.localDepName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetLocalNSMIPs(t *testing.T) {
	tests := []struct {
		name     string
		sliceGw  *kubeslicev1beta1.SliceGateway
		expected []string
	}{
		{
			name: "Multiple pods with NSM IPs",
			sliceGw: &kubeslicev1beta1.SliceGateway{
				Status: kubeslicev1beta1.SliceGatewayStatus{
					GatewayPodStatus: []*kubeslicev1beta1.GwPodInfo{
						{LocalNsmIP: "192.168.0.1"},
						{LocalNsmIP: "192.168.0.2"},
					},
				},
			},
			expected: []string{"192.168.0.1", "192.168.0.2"},
		},
		{
			name: "No pods",
			sliceGw: &kubeslicev1beta1.SliceGateway{
				Status: kubeslicev1beta1.SliceGatewayStatus{
					GatewayPodStatus: []*kubeslicev1beta1.GwPodInfo{},
				},
			},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getLocalNSMIPs(tt.sliceGw)
			assert.Equal(t, tt.expected, result)
		})
	}
}
