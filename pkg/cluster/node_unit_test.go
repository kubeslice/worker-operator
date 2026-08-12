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
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetNodeIP(t *testing.T) {
	tests := []struct {
		name              string
		isNetworkPresent  bool
		nodes             []runtime.Object
		expectedIPs       []string
		expectedError     bool
		expectedEmptyList bool
	}{
		{
			name:             "get external IPs with network present",
			isNetworkPresent: true,
			nodes: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
						Labels: map[string]string{
							"kubeslice.io/node-type": "gateway",
						},
					},
					Status: corev1.NodeStatus{
						Addresses: []corev1.NodeAddress{
							{Type: corev1.NodeExternalIP, Address: "1.2.3.4"},
							{Type: corev1.NodeInternalIP, Address: "10.0.0.1"},
						},
					},
				},
			},
			expectedIPs:   []string{"1.2.3.4"},
			expectedError: false,
		},
		{
			name:             "fallback to internal IPs when external IPs not available",
			isNetworkPresent: true,
			nodes: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
						Labels: map[string]string{
							"kubeslice.io/node-type": "gateway",
						},
					},
					Status: corev1.NodeStatus{
						Addresses: []corev1.NodeAddress{
							{Type: corev1.NodeInternalIP, Address: "10.0.0.1"},
						},
					},
				},
			},
			expectedIPs:   []string{"10.0.0.1"},
			expectedError: false,
		},
		{
			name:             "multiple gateway nodes",
			isNetworkPresent: true,
			nodes: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
						Labels: map[string]string{
							"kubeslice.io/node-type": "gateway",
						},
					},
					Status: corev1.NodeStatus{
						Addresses: []corev1.NodeAddress{
							{Type: corev1.NodeExternalIP, Address: "1.2.3.4"},
						},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node2",
						Labels: map[string]string{
							"kubeslice.io/node-type": "gateway",
						},
					},
					Status: corev1.NodeStatus{
						Addresses: []corev1.NodeAddress{
							{Type: corev1.NodeExternalIP, Address: "5.6.7.8"},
						},
					},
				},
			},
			expectedIPs:   []string{"1.2.3.4", "5.6.7.8"},
			expectedError: false,
		},
		{
			name:             "no network present - return all nodes",
			isNetworkPresent: false,
			nodes: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
					},
					Status: corev1.NodeStatus{
						Addresses: []corev1.NodeAddress{
							{Type: corev1.NodeExternalIP, Address: "1.2.3.4"},
						},
					},
				},
			},
			expectedIPs:   []string{"1.2.3.4"},
			expectedError: false,
		},
		{
			name:             "no nodes available",
			isNetworkPresent: true,
			nodes:            []runtime.Object{},
			expectedIPs:      []string{""},
			expectedError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientBuilder().WithRuntimeObjects(tt.nodes...).Build()
			nodeInfo = &NodeInfo{}

			result, err := GetNodeIP(client, tt.isNetworkPresent)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedIPs, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedIPs, result)
			}
		})
	}
}

func TestGetNodeExternalIpList(t *testing.T) {
	tests := []struct {
		name             string
		isNetworkPresent bool
		nodes            []runtime.Object
		expectedLength   int
		expectedError    bool
	}{
		{
			name:             "get gateway nodes when network present",
			isNetworkPresent: true,
			nodes: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "gateway-node",
						Labels: map[string]string{
							"kubeslice.io/node-type": "gateway",
						},
					},
					Status: corev1.NodeStatus{
						Addresses: []corev1.NodeAddress{
							{Type: corev1.NodeExternalIP, Address: "1.2.3.4"},
						},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "worker-node",
						Labels: map[string]string{
							"kubeslice.io/node-type": "worker",
						},
					},
					Status: corev1.NodeStatus{
						Addresses: []corev1.NodeAddress{
							{Type: corev1.NodeExternalIP, Address: "5.6.7.8"},
						},
					},
				},
			},
			expectedLength: 1,
			expectedError:  false,
		},
		{
			name:             "get all nodes when network not present",
			isNetworkPresent: false,
			nodes: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
					},
					Status: corev1.NodeStatus{
						Addresses: []corev1.NodeAddress{
							{Type: corev1.NodeExternalIP, Address: "1.2.3.4"},
						},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node2",
					},
					Status: corev1.NodeStatus{
						Addresses: []corev1.NodeAddress{
							{Type: corev1.NodeExternalIP, Address: "5.6.7.8"},
						},
					},
				},
			},
			expectedLength: 2,
			expectedError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientBuilder().WithRuntimeObjects(tt.nodes...).Build()
			nodeInfo := &NodeInfo{Client: client}

			result, err := nodeInfo.getNodeExternalIpList(tt.isNetworkPresent)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedLength, len(result))
			}
		})
	}
}

func TestGetNodeExternalIpListGlobal(t *testing.T) {
	nodeInfo = &NodeInfo{
		NodeIPList: []string{"1.2.3.4", "5.6.7.8"},
	}

	result := GetNodeExternalIpList()
	assert.Equal(t, []string{"1.2.3.4", "5.6.7.8"}, result)
}

func TestPopulateNodeIpList(t *testing.T) {
	tests := []struct {
		name             string
		isNetworkPresent bool
		nodes            []runtime.Object
		expectedIPs      []string
		expectedError    bool
	}{
		{
			name:             "populate with external IPs",
			isNetworkPresent: true,
			nodes: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
						Labels: map[string]string{
							"kubeslice.io/node-type": "gateway",
						},
					},
					Status: corev1.NodeStatus{
						Addresses: []corev1.NodeAddress{
							{Type: corev1.NodeExternalIP, Address: "1.2.3.4"},
							{Type: corev1.NodeInternalIP, Address: "10.0.0.1"},
						},
					},
				},
			},
			expectedIPs:   []string{"1.2.3.4"},
			expectedError: false,
		},
		{
			name:             "populate with internal IPs when no external",
			isNetworkPresent: true,
			nodes: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
						Labels: map[string]string{
							"kubeslice.io/node-type": "gateway",
						},
					},
					Status: corev1.NodeStatus{
						Addresses: []corev1.NodeAddress{
							{Type: corev1.NodeInternalIP, Address: "10.0.0.1"},
							{Type: corev1.NodeHostName, Address: "node1"},
						},
					},
				},
			},
			expectedIPs:   []string{"10.0.0.1"},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientBuilder().WithRuntimeObjects(tt.nodes...).Build()
			nodeInfo := &NodeInfo{Client: client}

			err := nodeInfo.populateNodeIpList(tt.isNetworkPresent)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedIPs, nodeInfo.NodeIPList)
			}
		})
	}
}
