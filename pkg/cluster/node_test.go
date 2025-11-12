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
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/kubeslice/worker-operator/controllers"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetNodeIP(t *testing.T) {
	var tests = []struct {
		description      string
		isNetworkPresent bool
		objs             []runtime.Object
		expected         []string
		expectError      bool
	}{
		{
			description:      "Gateway nodes with external IPs - network present",
			isNetworkPresent: true,
			expected:         []string{"10.0.1.100"},
			objs: []runtime.Object{
				createNodeList([]nodeConfig{
					{name: "gateway-1", isGateway: true, externalIP: "10.0.1.100"},
					{name: "worker-1", externalIP: "10.0.1.200"},
				}),
			},
		},
		{
			description:      "Gateway nodes fallback to internal IPs - network present",
			isNetworkPresent: true,
			expected:         []string{"192.168.1.10"},
			objs: []runtime.Object{
				createNodeList([]nodeConfig{
					{name: "gateway-1", isGateway: true, internalIP: "192.168.1.10"},
				}),
			},
		},
		{
			description:      "All nodes external IPs - network not present",
			isNetworkPresent: false,
			expected:         []string{"10.0.1.100", "10.0.1.200"},
			objs: []runtime.Object{
				createNodeList([]nodeConfig{
					{name: "gateway-1", isGateway: true, externalIP: "10.0.1.100"},
					{name: "worker-1", externalIP: "10.0.1.200"},
				}),
			},
		},
		{
			description:      "All nodes internal IPs - network not present",
			isNetworkPresent: false,
			expected:         []string{"192.168.1.10", "192.168.1.11"},
			objs: []runtime.Object{
				createNodeList([]nodeConfig{
					{name: "node-1", internalIP: "192.168.1.10"},
					{name: "node-2", internalIP: "192.168.1.11"},
				}),
			},
		},
		{
			description:      "No nodes available",
			isNetworkPresent: true,
			expected:         []string{""},
			objs:             []runtime.Object{createNodeList([]nodeConfig{})},
			expectError:      true,
		},
		{
			description:      "No gateway nodes when network present",
			isNetworkPresent: true,
			expected:         []string{""},
			objs: []runtime.Object{
				createNodeList([]nodeConfig{
					{name: "worker-1", externalIP: "10.0.1.200"},
				}),
			},
			expectError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			// Reset global state
			nodeInfo = &NodeInfo{}

			client := fake.NewClientBuilder().WithRuntimeObjects(test.objs...).Build()
			actual, err := GetNodeIP(client, test.isNetworkPresent)

			if test.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %s", err)
				}
			}

			expectedSorted := make([]string, len(test.expected))
			copy(expectedSorted, test.expected)
			sort.Strings(expectedSorted)

			actualSorted := make([]string, len(actual))
			copy(actualSorted, actual)
			sort.Strings(actualSorted)

			if diff := cmp.Diff(expectedSorted, actualSorted); diff != "" {
				t.Errorf("GetNodeIP() differ (-want +got): %s", diff)
			}
		})
	}
}

func TestGetNodeExternalIpList(t *testing.T) {
	t.Run("Should return cached IP list", func(t *testing.T) {
		nodeInfo = &NodeInfo{
			NodeIPList: []string{"192.168.1.1", "192.168.1.2"},
		}

		actual := GetNodeExternalIpList()
		expected := []string{"192.168.1.1", "192.168.1.2"}

		if diff := cmp.Diff(expected, actual); diff != "" {
			t.Errorf("GetNodeExternalIpList() differ (-want +got): %s", diff)
		}
	})
}

type nodeConfig struct {
	name       string
	isGateway  bool
	externalIP string
	internalIP string
}

func createNodeList(configs []nodeConfig) *corev1.NodeList {
	nodeList := &corev1.NodeList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "NodeList",
			APIVersion: "v1",
		},
	}

	for _, config := range configs {
		node := corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: config.name,
			},
		}

		if config.isGateway {
			node.ObjectMeta.Labels = map[string]string{
				controllers.NodeTypeSelectorLabelKey: "gateway",
			}
		}

		var addresses []corev1.NodeAddress
		if config.externalIP != "" {
			addresses = append(addresses, corev1.NodeAddress{
				Type:    corev1.NodeExternalIP,
				Address: config.externalIP,
			})
		}
		if config.internalIP != "" {
			addresses = append(addresses, corev1.NodeAddress{
				Type:    corev1.NodeInternalIP,
				Address: config.internalIP,
			})
		}
		node.Status.Addresses = addresses

		nodeList.Items = append(nodeList.Items, node)
	}

	return nodeList
}
