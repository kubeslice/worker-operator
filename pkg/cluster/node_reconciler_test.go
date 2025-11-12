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
	"sort"
	"testing"

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"github.com/kubeslice/worker-operator/controllers"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestNodeReconciler_Reconcile(t *testing.T) {
	tests := []struct {
		description       string
		objs              []runtime.Object
		expectedNodeIPs   []string
		expectError       bool
		expectedErrorMsg  string
		initialNodeIPList []string
	}{
		{
			description: "Gateway nodes with external IPs - successful reconcile",
			objs: []runtime.Object{
				createTestNode("gateway-1", true, corev1.ConditionTrue, "10.0.1.100", "192.168.1.10"),
				createTestNode("gateway-2", true, corev1.ConditionTrue, "10.0.1.101", "192.168.1.11"),
			},
			expectedNodeIPs:   []string{"10.0.1.100", "10.0.1.101"},
			expectError:       false,
			initialNodeIPList: []string{},
		},
		{
			description: "Gateway nodes fallback to internal IPs",
			objs: []runtime.Object{
				createTestNode("gateway-1", true, corev1.ConditionTrue, "", "192.168.1.10"),
				createTestNode("gateway-2", true, corev1.ConditionTrue, "", "192.168.1.11"),
			},
			expectedNodeIPs:   []string{"192.168.1.10", "192.168.1.11"},
			expectError:       false,
			initialNodeIPList: []string{},
		},
		{
			description: "Mixed ready and not ready gateway nodes",
			objs: []runtime.Object{
				createTestNode("gateway-1", true, corev1.ConditionTrue, "10.0.1.100", ""),
				createTestNode("gateway-2", true, corev1.ConditionFalse, "10.0.1.101", ""), // Not ready
				createTestNode("gateway-3", true, corev1.ConditionTrue, "10.0.1.102", ""),
			},
			expectedNodeIPs:   []string{"10.0.1.100", "10.0.1.102"},
			expectError:       false,
			initialNodeIPList: []string{},
		},
		{
			description: "No gateway nodes available",
			objs: []runtime.Object{
				createTestNode("worker-1", false, corev1.ConditionTrue, "10.0.1.200", ""),
			},
			expectedNodeIPs:  []string{},
			expectError:      true,
			expectedErrorMsg: "no gateway nodes available",
		},
		{
			description: "Gateway nodes exist but none are ready",
			objs: []runtime.Object{
				createTestNode("gateway-1", true, corev1.ConditionFalse, "10.0.1.100", ""),
				createTestNode("gateway-2", true, corev1.ConditionFalse, "10.0.1.101", ""),
			},
			expectedNodeIPs:  []string{},
			expectError:      true,
			expectedErrorMsg: "number of nodeIPs is zero, reconciling",
		},
		{
			description: "Gateway nodes ready but no IP addresses",
			objs: []runtime.Object{
				createTestNode("gateway-1", true, corev1.ConditionTrue, "", ""), // No IP addresses
				createTestNode("gateway-2", true, corev1.ConditionTrue, "", ""), // No IP addresses
			},
			expectedNodeIPs:  []string{},
			expectError:      true,
			expectedErrorMsg: "number of nodeIPs is zero, reconciling",
		},
		{
			description: "Node IPs unchanged - no update needed",
			objs: []runtime.Object{
				createTestNode("gateway-1", true, corev1.ConditionTrue, "10.0.1.100", ""),
				createTestNode("gateway-2", true, corev1.ConditionTrue, "10.0.1.101", ""),
			},
			expectedNodeIPs:   []string{"10.0.1.100", "10.0.1.101"},
			expectError:       false,
			initialNodeIPList: []string{"10.0.1.100", "10.0.1.101"},
		},
		{
			description: "Node IPs changed - update needed",
			objs: []runtime.Object{
				createTestNode("gateway-1", true, corev1.ConditionTrue, "10.0.1.100", ""),
				createTestNode("gateway-3", true, corev1.ConditionTrue, "10.0.1.102", ""),
			},
			expectedNodeIPs:   []string{"10.0.1.100", "10.0.1.102"},
			expectError:       false,
			initialNodeIPList: []string{"10.0.1.100", "10.0.1.101"},
		},
		{
			description: "External IPs preferred over internal IPs",
			objs: []runtime.Object{
				createTestNode("gateway-1", true, corev1.ConditionTrue, "10.0.1.100", "192.168.1.10"),
				createTestNode("gateway-2", true, corev1.ConditionTrue, "10.0.1.101", "192.168.1.11"),
			},
			expectedNodeIPs:   []string{"10.0.1.100", "10.0.1.101"}, // External IPs should be chosen
			expectError:       false,
			initialNodeIPList: []string{},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			// Reset global state
			nodeInfo = &NodeInfo{
				NodeIPList: test.initialNodeIPList,
			}

			client := fake.NewClientBuilder().
				WithRuntimeObjects(test.objs...).
				Build()

			reconciler := &NodeReconciler{
				Client: client,
				Log:    zap.New(zap.UseDevMode(true)),
			}

			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name: "test-node",
				},
			}

			result, err := reconciler.Reconcile(context.Background(), req)

			if test.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}
				if test.expectedErrorMsg != "" && err.Error() != test.expectedErrorMsg {
					t.Errorf("Expected error message %q, got %q", test.expectedErrorMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %s", err)
				return
			}

			if result.Requeue {
				t.Errorf("Expected no requeue, but got requeue")
			}

			nodeInfo.Lock()
			actualNodeIPs := make([]string, len(nodeInfo.NodeIPList))
			copy(actualNodeIPs, nodeInfo.NodeIPList)
			nodeInfo.Unlock()

			expectedSorted := make([]string, len(test.expectedNodeIPs))
			copy(expectedSorted, test.expectedNodeIPs)
			sort.Strings(expectedSorted)
			sort.Strings(actualNodeIPs)

			if diff := cmp.Diff(expectedSorted, actualNodeIPs); diff != "" {
				t.Errorf("NodeIPList differ (-want +got): %s", diff)
			}
		})
	}
}

func TestFilterAvailableNodes(t *testing.T) {
	tests := []struct {
		description   string
		inputNodes    corev1.NodeList
		expectedCount int
		expectedNames []string
	}{
		{
			description: "All nodes ready",
			inputNodes: corev1.NodeList{
				Items: []corev1.Node{
					*createTestNode("node-1", true, corev1.ConditionTrue, "", ""),
					*createTestNode("node-2", true, corev1.ConditionTrue, "", ""),
				},
			},
			expectedCount: 2,
			expectedNames: []string{"node-1", "node-2"},
		},
		{
			description: "Mixed ready and not ready nodes",
			inputNodes: corev1.NodeList{
				Items: []corev1.Node{
					*createTestNode("node-1", true, corev1.ConditionTrue, "", ""),
					*createTestNode("node-2", true, corev1.ConditionFalse, "", ""),
					*createTestNode("node-3", true, corev1.ConditionTrue, "", ""),
				},
			},
			expectedCount: 2,
			expectedNames: []string{"node-1", "node-3"},
		},
		{
			description: "No ready nodes",
			inputNodes: corev1.NodeList{
				Items: []corev1.Node{
					*createTestNode("node-1", true, corev1.ConditionFalse, "", ""),
					*createTestNode("node-2", true, corev1.ConditionUnknown, "", ""),
				},
			},
			expectedCount: 0,
			expectedNames: []string{},
		},
		{
			description: "Empty node list",
			inputNodes: corev1.NodeList{
				Items: []corev1.Node{},
			},
			expectedCount: 0,
			expectedNames: []string{},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			result := filterAvailableNodes(test.inputNodes)

			if len(result.Items) != test.expectedCount {
				t.Errorf("Expected %d ready nodes, got %d", test.expectedCount, len(result.Items))
				return
			}

			actualNames := make([]string, 0)
			for _, node := range result.Items {
				actualNames = append(actualNames, node.Name)
			}

			sort.Strings(actualNames)
			expectedSorted := make([]string, len(test.expectedNames))
			copy(expectedSorted, test.expectedNames)
			sort.Strings(expectedSorted)

			if diff := cmp.Diff(expectedSorted, actualNames); diff != "" {
				t.Errorf("Ready node names differ (-want +got): %s", diff)
			}
		})
	}
}

func TestSameStringSlice(t *testing.T) {
	tests := []struct {
		description string
		slice1      []string
		slice2      []string
		expected    bool
	}{
		{
			description: "Identical slices",
			slice1:      []string{"a", "b", "c"},
			slice2:      []string{"a", "b", "c"},
			expected:    true,
		},
		{
			description: "Same elements different order",
			slice1:      []string{"a", "b", "c"},
			slice2:      []string{"c", "a", "b"},
			expected:    true,
		},
		{
			description: "Different lengths",
			slice1:      []string{"a", "b"},
			slice2:      []string{"a", "b", "c"},
			expected:    false,
		},
		{
			description: "Different elements",
			slice1:      []string{"a", "b", "c"},
			slice2:      []string{"a", "b", "d"},
			expected:    false,
		},
		{
			description: "Empty slices",
			slice1:      []string{},
			slice2:      []string{},
			expected:    true,
		},
		{
			description: "One empty slice",
			slice1:      []string{"a"},
			slice2:      []string{},
			expected:    false,
		},
		{
			description: "Duplicate elements - same count",
			slice1:      []string{"a", "a", "b"},
			slice2:      []string{"a", "b", "a"},
			expected:    true,
		},
		{
			description: "Duplicate elements - different count",
			slice1:      []string{"a", "a", "b"},
			slice2:      []string{"a", "b", "b"},
			expected:    false,
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			result := sameStringSlice(test.slice1, test.slice2)
			if result != test.expected {
				t.Errorf("Expected %v, got %v for slices %v and %v", test.expected, result, test.slice1, test.slice2)
			}
		})
	}
}

func TestNodeReconciler_SetupWithManager(t *testing.T) {
	t.Run("Should setup controller without error", func(t *testing.T) {
		reconciler := &NodeReconciler{
			Log: logr.Discard(),
		}

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("SetupWithManager panicked: %v", r)
			}
		}()

		err := reconciler.SetupWithManager(nil)
		if err == nil {
			t.Error("Expected error when passing nil manager")
		}
	})
}

func createTestNode(name string, isGateway bool, readyStatus corev1.ConditionStatus, externalIP, internalIP string) *corev1.Node {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{
					Type:   corev1.NodeReady,
					Status: readyStatus,
				},
			},
		},
	}

	if isGateway {
		node.ObjectMeta.Labels = map[string]string{
			controllers.NodeTypeSelectorLabelKey: "gateway",
		}
	}

	var addresses []corev1.NodeAddress
	if externalIP != "" {
		addresses = append(addresses, corev1.NodeAddress{
			Type:    corev1.NodeExternalIP,
			Address: externalIP,
		})
	}
	if internalIP != "" {
		addresses = append(addresses, corev1.NodeAddress{
			Type:    corev1.NodeInternalIP,
			Address: internalIP,
		})
	}
	node.Status.Addresses = addresses

	return node
}
