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
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNewCluster(t *testing.T) {
	tests := []struct {
		name        string
		clusterName string
	}{
		{
			name:        "create new cluster",
			clusterName: "test-cluster",
		},
		{
			name:        "create cluster with different name",
			clusterName: "another-cluster",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientBuilder().Build()
			result := NewCluster(client, tt.clusterName)

			assert.NotNil(t, result)
			cluster, ok := result.(*Cluster)
			assert.True(t, ok)
			assert.Equal(t, tt.clusterName, cluster.Name)
		})
	}
}

func TestGetClusterLocation(t *testing.T) {
	tests := []struct {
		name              string
		nodes             []runtime.Object
		expectedProvider  string
		expectedRegion    string
		expectedError     bool
	}{
		{
			name: "get GCP cluster location",
			nodes: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
						Labels: map[string]string{
							"topology.kubernetes.io/region": "us-west1",
						},
					},
					Spec: corev1.NodeSpec{
						ProviderID: "gce://project/us-west1-b/instance",
					},
				},
			},
			expectedProvider: "gcp",
			expectedRegion:   "us-west1",
			expectedError:    false,
		},
		{
			name: "get AWS cluster location",
			nodes: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
						Labels: map[string]string{
							"topology.kubernetes.io/region": "us-east-1",
						},
					},
					Spec: corev1.NodeSpec{
						ProviderID: "aws:///us-east-1a/i-1234567890",
					},
				},
			},
			expectedProvider: "aws",
			expectedRegion:   "us-east-1",
			expectedError:    false,
		},
		{
			name: "get Azure cluster location",
			nodes: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
						Labels: map[string]string{
							"topology.kubernetes.io/region": "eastus",
						},
					},
					Spec: corev1.NodeSpec{
						ProviderID: "azure:///subscriptions/sub-id/resourceGroups/rg/providers/Microsoft.Compute/virtualMachineScaleSets/vmss/virtualMachines/0",
					},
				},
			},
			expectedProvider: "azure",
			expectedRegion:   "eastus",
			expectedError:    false,
		},
		{
			name: "empty provider when no providerID",
			nodes: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
						Labels: map[string]string{
							"topology.kubernetes.io/region": "us-west1",
						},
					},
					Spec: corev1.NodeSpec{
						ProviderID: "",
					},
				},
			},
			expectedProvider: "",
			expectedRegion:   "us-west1",
			expectedError:    false,
		},
		{
			name:             "error when no nodes",
			nodes:            []runtime.Object{},
			expectedProvider: "",
			expectedRegion:   "",
			expectedError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientBuilder().WithRuntimeObjects(tt.nodes...).Build()
			c := &Cluster{
				Client: client,
				Name:   "test-cluster",
			}

			result, err := c.getClusterLocation(context.Background())

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedProvider, result.CloudProvider)
				assert.Equal(t, tt.expectedRegion, result.CloudRegion)
			}
		})
	}
}

func TestGetClusterInfo(t *testing.T) {
	tests := []struct {
		name          string
		clusterName   string
		nodes         []runtime.Object
		expectedError bool
	}{
		{
			name:        "get cluster info successfully",
			clusterName: "test-cluster",
			nodes: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
						Labels: map[string]string{
							"topology.kubernetes.io/region": "us-west1",
						},
					},
					Spec: corev1.NodeSpec{
						ProviderID: "gce://project/us-west1-b/instance",
					},
				},
			},
			expectedError: false,
		},
		{
			name:          "error getting cluster info",
			clusterName:   "test-cluster",
			nodes:         []runtime.Object{},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientBuilder().WithRuntimeObjects(tt.nodes...).Build()
			c := &Cluster{
				Client: client,
				Name:   tt.clusterName,
			}

			result, err := c.GetClusterInfo(context.Background())

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.clusterName, result.Name)
			}
		})
	}
}

func TestGetNsmExcludedPrefixErrors(t *testing.T) {
	tests := []struct {
		name          string
		configMap     *corev1.ConfigMap
		expectedError string
	}{
		{
			name: "empty data in configmap",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nsm-config",
					Namespace: "demo",
				},
				Data: map[string]string{},
			},
			expectedError: "prefix data not present in nsm configmap",
		},
		{
			name: "missing excluded_prefixes_output.yaml key",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nsm-config",
					Namespace: "demo",
				},
				Data: map[string]string{
					"other-key": "some-value",
				},
			},
			expectedError: "cni subnet info not present in nsm configmap",
		},
		{
			name: "invalid yaml in configmap",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nsm-config",
					Namespace: "demo",
				},
				Data: map[string]string{
					"excluded_prefixes_output.yaml": "invalid: [yaml: content",
				},
			},
			expectedError: "failed to get prefixes from nsm configmap",
		},
		{
			name: "no Prefixes key in yaml",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nsm-config",
					Namespace: "demo",
				},
				Data: map[string]string{
					"excluded_prefixes_output.yaml": "OtherKey: value",
				},
			},
			expectedError: "failed to get prefixes from nsm configmap",
		},
		{
			name:          "configmap not found",
			configMap:     nil,
			expectedError: "error getting nsm configmap",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var objs []runtime.Object
			if tt.configMap != nil {
				objs = append(objs, tt.configMap)
			}

			client := fake.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			c := &Cluster{
				Client: client,
				Name:   "test-cluster",
			}

		result, err := c.GetNsmExcludedPrefix(context.Background(), "nsm-config", "demo")

		assert.Error(t, err)
		assert.Nil(t, result)
		// Check that error is present, without requiring exact message match
		if tt.name == "configmap not found" {
			assert.Contains(t, err.Error(), "not found")
		} else {
			assert.Contains(t, err.Error(), tt.expectedError)
		}
		})
	}
}

func TestGetPrefixes(t *testing.T) {
	tests := []struct {
		name          string
		configMap     corev1.ConfigMap
		expected      []string
		expectedError bool
	}{
		{
			name: "valid prefixes",
			configMap: corev1.ConfigMap{
				Data: map[string]string{
					"excluded_prefixes_output.yaml": `
Prefixes:
- 192.168.0.0/16
- 10.96.0.0/12
`,
				},
			},
			expected:      []string{"192.168.0.0/16", "10.96.0.0/12"},
			expectedError: false,
		},
		{
			name: "single prefix",
			configMap: corev1.ConfigMap{
				Data: map[string]string{
					"excluded_prefixes_output.yaml": `
Prefixes:
- 192.168.0.0/16
`,
				},
			},
			expected:      []string{"192.168.0.0/16"},
			expectedError: false,
		},
		{
			name: "no Prefixes key",
			configMap: corev1.ConfigMap{
				Data: map[string]string{
					"excluded_prefixes_output.yaml": `
OtherKey:
- value
`,
				},
			},
			expected:      nil,
			expectedError: true,
		},
		{
			name: "invalid yaml",
			configMap: corev1.ConfigMap{
				Data: map[string]string{
					"excluded_prefixes_output.yaml": "invalid yaml content [[[",
				},
			},
			expected:      nil,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := getPrefixes(tt.configMap)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
