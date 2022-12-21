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

	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetExcludedNsmIps(t *testing.T) {
	var tests = []struct {
		description string
		configMap   string
		namespace   string
		expected    []string
		objs        []runtime.Object
	}{
		{"filter namespace", "nsm-config", "demo", []string{"192.168.0.0/16", "10.96.0.0/12"}, []runtime.Object{configMap("nsm-config", "demo", `
Prefixes:
- 192.168.0.0/16
- 10.96.0.0/12		
`)}},

		{"single prefix data", "nsm-config", "demo", []string{"192.168.0.0/16"}, []runtime.Object{configMap("nsm-config", "demo", `
Prefixes:
- 192.168.0.0/16		
`)}},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			client := fake.ClientBuilder{}
			client.WithRuntimeObjects(test.objs...)
			cluster := NewCluster(client.Build(), "fakeCluster")
			actual, err := cluster.GetNsmExcludedPrefix(context.Background(), test.configMap, test.namespace)
			t.Log(actual)
			if err != nil {
				t.Logf("Unexpected error: %s", err)
			}
			if diff := cmp.Diff(actual, test.expected); diff != "" {
				t.Errorf("%T differ (-got, +want): %s", test.expected, diff)
				return
			}
		})
	}
}
func TestCluster_GetClusterInfo(t *testing.T) {
	var tests = []struct {
		description string
		objs        []runtime.Object
		expected    *ClusterInfo
	}{
		{
			description: "GKE Cluster Node",
			objs: []runtime.Object{getNodeList(
				"gke-alice-3-alice-3-main-pool-ace60e8a-ds90", "gce://atomic-dev/us-west1-b/gke-alice-3-alice-3-main-pool-ace60e8a-ds90\"", "us-west1",
			)},
			expected: &ClusterInfo{
				Name: "fakeCluster",
				ClusterProperty: ClusterProperty{
					GeoLocation: GeoLocation{
						CloudProvider: "gcp",
						CloudRegion:   "us-west1",
					},
				},
			},
		},
		{
			description: "AWS Cluster Node",
			objs: []runtime.Object{
				getNodeList(
					"ip-15-2-115-11.ec2.internal", "aws:///us-east-1c/i-0231e110103311", "us-east-1",
				),
			},
			expected: &ClusterInfo{
				Name: "fakeCluster",
				ClusterProperty: ClusterProperty{
					GeoLocation: GeoLocation{
						CloudProvider: "aws",
						CloudRegion:   "us-east-1",
					},
				},
			},
		},
		{
			description: "AKS Cluster Node",
			objs: []runtime.Object{
				getNodeList(
					"fakeCluster",
					"azure:///subscriptions/244691-2f70-44bc-9f7e-114f70ab8843/resourceGroups/mc_demoresource_aks-testing-1_eastus/providers/Microsoft.Compute/virtualMachineScaleSets/aks-alice-38597086-vmss/virtualMachines/0",
					"eastus",
				),
			},
			expected: &ClusterInfo{
				Name: "fakeCluster",
				ClusterProperty: ClusterProperty{
					GeoLocation: GeoLocation{
						CloudProvider: "azure",
						CloudRegion:   "eastus",
					},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			client := fake.ClientBuilder{}
			client.WithRuntimeObjects(test.objs...)
			cluster := NewCluster(client.Build(), "fakeCluster")
			actual, err := cluster.GetClusterInfo(context.Background())
			t.Log(actual)
			if err != nil {
				t.Logf("Unexpected error: %s", err)
			}
			if diff := cmp.Diff(actual, test.expected); diff != "" {
				t.Errorf("%T differ (-got, +want): %s", test.expected, diff)
				return
			}
		})
	}
}

func configMap(name, namespace, data string) *v1.ConfigMap {
	configMapData := make(map[string]string)
	configMapData["excluded_prefixes_output.yaml"] = data
	configMap := v1.ConfigMap{
		// TypeMeta: metav1.TypeMeta{
		// 	Kind:       "ConfigMap",
		// 	APIVersion: "v1",
		// },
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: configMapData,
	}
	return &configMap
}
func getNodeList(name, providerID, region string) *v1.NodeList {
	nodeList := v1.NodeList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "NodeList",
			APIVersion: "v1",
		},
		Items: []v1.Node{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
					Labels: map[string]string{
						"topology.kubernetes.io/region": region,
					},
				},
				Spec: v1.NodeSpec{
					ProviderID: providerID,
				},
			},
		},
	}
	return &nodeList
}
