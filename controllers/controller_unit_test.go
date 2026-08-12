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

package controllers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestExists(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		element  string
		expected bool
	}{
		{
			name:     "element exists",
			slice:    []string{"foo", "bar", "baz"},
			element:  "bar",
			expected: true,
		},
		{
			name:     "element does not exist",
			slice:    []string{"foo", "bar", "baz"},
			element:  "qux",
			expected: false,
		},
		{
			name:     "empty slice",
			slice:    []string{},
			element:  "foo",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := exists(tt.slice, tt.element)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetSlice(t *testing.T) {
	tests := []struct {
		name          string
		sliceName     string
		objs          []runtime.Object
		expectedError bool
		expectedName  string
	}{
		{
			name:      "get existing slice",
			sliceName: "test-slice",
			objs: []runtime.Object{
				&kubeslicev1beta1.Slice{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-slice",
						Namespace: "kubeslice-system",
					},
				},
			},
			expectedError: false,
			expectedName:  "test-slice",
		},
		{
			name:          "slice not found",
			sliceName:     "non-existent",
			objs:          []runtime.Object{},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = kubeslicev1beta1.AddToScheme(scheme)
			client := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(tt.objs...).Build()

			result, err := GetSlice(context.Background(), client, tt.sliceName)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedName, result.Name)
			}
		})
	}
}

func TestGetSliceGatewayList(t *testing.T) {
	tests := []struct {
		name           string
		sliceName      string
		objs           []runtime.Object
		expectedCount  int
		expectedError  bool
	}{
		{
			name:      "get slice gateways",
			sliceName: "test-slice",
			objs: []runtime.Object{
				&kubeslicev1beta1.SliceGateway{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "gw1",
						Namespace: "kubeslice-system",
						Labels: map[string]string{
							ApplicationNamespaceSelectorLabelKey: "test-slice",
						},
					},
				},
				&kubeslicev1beta1.SliceGateway{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "gw2",
						Namespace: "kubeslice-system",
						Labels: map[string]string{
							ApplicationNamespaceSelectorLabelKey: "test-slice",
						},
					},
				},
			},
			expectedCount: 2,
			expectedError: false,
		},
		{
			name:          "no gateways found",
			sliceName:     "test-slice",
			objs:          []runtime.Object{},
			expectedCount: 0,
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = kubeslicev1beta1.AddToScheme(scheme)
			client := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(tt.objs...).Build()

			result, err := GetSliceGatewayList(context.Background(), client, tt.sliceName)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedCount, len(result.Items))
			}
		})
	}
}

func TestGetSliceGatewayServers(t *testing.T) {
	tests := []struct {
		name          string
		sliceName     string
		objs          []runtime.Object
		expectedCount int
	}{
		{
			name:      "get server gateways",
			sliceName: "test-slice",
			objs: []runtime.Object{
				&kubeslicev1beta1.SliceGateway{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "gw-server",
						Namespace: "kubeslice-system",
						Labels: map[string]string{
							ApplicationNamespaceSelectorLabelKey: "test-slice",
						},
					},
					Status: kubeslicev1beta1.SliceGatewayStatus{
						Config: kubeslicev1beta1.SliceGatewayConfig{
							SliceGatewayHostType: "Server",
						},
					},
				},
				&kubeslicev1beta1.SliceGateway{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "gw-client",
						Namespace: "kubeslice-system",
						Labels: map[string]string{
							ApplicationNamespaceSelectorLabelKey: "test-slice",
						},
					},
					Status: kubeslicev1beta1.SliceGatewayStatus{
						Config: kubeslicev1beta1.SliceGatewayConfig{
							SliceGatewayHostType: "Client",
						},
					},
				},
			},
			expectedCount: 1,
		},
		{
			name:          "no server gateways",
			sliceName:     "test-slice",
			objs:          []runtime.Object{},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = kubeslicev1beta1.AddToScheme(scheme)
			client := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(tt.objs...).Build()

			result, err := GetSliceGatewayServers(context.Background(), client, tt.sliceName)

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCount, len(result))
		})
	}
}

func TestGetSliceGwServices(t *testing.T) {
	tests := []struct {
		name          string
		sliceName     string
		objs          []runtime.Object
		expectedCount int
	}{
		{
			name:      "get slice gateway services",
			sliceName: "test-slice",
			objs: []runtime.Object{
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "svc1",
						Namespace: "kubeslice-system",
						Labels: map[string]string{
							ApplicationNamespaceSelectorLabelKey: "test-slice",
						},
					},
				},
			},
			expectedCount: 1,
		},
		{
			name:          "no services found",
			sliceName:     "test-slice",
			objs:          []runtime.Object{},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = corev1.AddToScheme(scheme)
			client := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(tt.objs...).Build()

			result, err := GetSliceGwServices(context.Background(), client, tt.sliceName)

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCount, len(result.Items))
		})
	}
}

func TestGetSliceRouterPodNameAndIP(t *testing.T) {
	tests := []struct {
		name           string
		sliceName      string
		objs           []runtime.Object
		expectedName   string
		expectedIP     string
		expectedError  bool
	}{
		{
			name:      "get running router pod",
			sliceName: "test-slice",
			objs: []runtime.Object{
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "vl3-router-pod",
						Namespace: "kubeslice-system",
						Labels: map[string]string{
							"networkservicemesh.io/impl": "vl3-service-test-slice",
						},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						PodIP: "10.0.0.1",
					},
				},
			},
			expectedName:  "vl3-router-pod",
			expectedIP:    "10.0.0.1",
			expectedError: false,
		},
		{
			name:      "pod not running",
			sliceName: "test-slice",
			objs: []runtime.Object{
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "vl3-router-pod",
						Namespace: "kubeslice-system",
						Labels: map[string]string{
							"networkservicemesh.io/impl": "vl3-service-test-slice",
						},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodPending,
					},
				},
			},
			expectedName:  "",
			expectedIP:    "",
			expectedError: false,
		},
		{
			name:          "no pod found",
			sliceName:     "test-slice",
			objs:          []runtime.Object{},
			expectedName:  "",
			expectedIP:    "",
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = corev1.AddToScheme(scheme)
			client := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(tt.objs...).Build()

			name, ip, err := GetSliceRouterPodNameAndIP(context.Background(), client, tt.sliceName)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedName, name)
				assert.Equal(t, tt.expectedIP, ip)
			}
		})
	}
}

func TestGetSliceGatewayEdgeServices(t *testing.T) {
	tests := []struct {
		name          string
		sliceName     string
		objs          []runtime.Object
		expectedCount int
	}{
		{
			name:      "get edge services",
			sliceName: "test-slice",
			objs: []runtime.Object{
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "edge-svc",
						Namespace: "kubeslice-system",
						Labels: map[string]string{
							SliceGatewaySelectorLabelKey: "test-slice",
							SliceGatewayEdgeTypeLabelKey: "LoadBalancer",
						},
					},
				},
			},
			expectedCount: 1,
		},
		{
			name:          "no edge services found",
			sliceName:     "test-slice",
			objs:          []runtime.Object{},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = corev1.AddToScheme(scheme)
			client := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(tt.objs...).Build()

			result, err := GetSliceGatewayEdgeServices(context.Background(), client, tt.sliceName)

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCount, len(result.Items))
		})
	}
}

func TestContructNetworkPolicyObject(t *testing.T) {
	tests := []struct {
		name      string
		slice     *kubeslicev1beta1.Slice
		appNs     string
		expectedPolicyName string
	}{
		{
			name: "construct network policy",
			slice: &kubeslicev1beta1.Slice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice",
					Namespace: "kubeslice-system",
				},
			Status: kubeslicev1beta1.SliceStatus{
				SliceConfig: &kubeslicev1beta1.SliceConfig{
					NamespaceIsolationProfile: &kubeslicev1beta1.NamespaceIsolationProfile{
						AllowedNamespaces: []string{"allowed-ns"},
					},
				},
			},
			},
			appNs:              "app-namespace",
			expectedPolicyName: "test-slice-app-namespace",
		},
		{
			name: "construct network policy with empty allowed namespaces",
			slice: &kubeslicev1beta1.Slice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "slice2",
					Namespace: "kubeslice-system",
				},
			Status: kubeslicev1beta1.SliceStatus{
				SliceConfig: &kubeslicev1beta1.SliceConfig{
					NamespaceIsolationProfile: &kubeslicev1beta1.NamespaceIsolationProfile{
						AllowedNamespaces: []string{},
					},
				},
			},
			},
			appNs:              "test-ns",
			expectedPolicyName: "slice2-test-ns",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ContructNetworkPolicyObject(context.Background(), tt.slice, tt.appNs)

			assert.NotNil(t, result)
			assert.Equal(t, tt.expectedPolicyName, result.Name)
			assert.Equal(t, tt.appNs, result.Namespace)
			assert.NotNil(t, result.Spec.Ingress)
			assert.NotNil(t, result.Spec.Egress)
		})
	}
}

func TestGetSliceIngressGwPod(t *testing.T) {
	tests := []struct {
		name             string
		slice            *kubeslicev1beta1.Slice
		expectedEnabled  bool
		expectedPod      *kubeslicev1beta1.AppPod
		expectedError    bool
	}{
		{
			name: "ingress enabled with pod",
			slice: &kubeslicev1beta1.Slice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice",
					Namespace: "kubeslice-system",
				},
				Status: kubeslicev1beta1.SliceStatus{
					SliceConfig: &kubeslicev1beta1.SliceConfig{
						ExternalGatewayConfig: &kubeslicev1beta1.ExternalGatewayConfig{
							Ingress: &kubeslicev1beta1.ExternalGatewayConfigOptions{
								Enabled: true,
							},
						},
					},
					AppPods: []kubeslicev1beta1.AppPod{
						{
							PodName:      "test-ingressgateway-pod",
							PodNamespace: "kubeslice-system",
							NsmIP:        "10.0.0.1",
						},
					},
				},
			},
			expectedEnabled: true,
			expectedPod: &kubeslicev1beta1.AppPod{
				PodName:      "test-ingressgateway-pod",
				PodNamespace: "kubeslice-system",
				NsmIP:        "10.0.0.1",
			},
			expectedError: false,
		},
		{
			name: "ingress enabled without pod",
			slice: &kubeslicev1beta1.Slice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice",
					Namespace: "kubeslice-system",
				},
				Status: kubeslicev1beta1.SliceStatus{
					SliceConfig: &kubeslicev1beta1.SliceConfig{
						ExternalGatewayConfig: &kubeslicev1beta1.ExternalGatewayConfig{
							Ingress: &kubeslicev1beta1.ExternalGatewayConfigOptions{
								Enabled: true,
							},
						},
					},
					AppPods: []kubeslicev1beta1.AppPod{},
				},
			},
			expectedEnabled: true,
			expectedPod:     nil,
			expectedError:   false,
		},
		{
			name: "ingress disabled",
			slice: &kubeslicev1beta1.Slice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice",
					Namespace: "kubeslice-system",
				},
				Status: kubeslicev1beta1.SliceStatus{
					SliceConfig: &kubeslicev1beta1.SliceConfig{
						ExternalGatewayConfig: &kubeslicev1beta1.ExternalGatewayConfig{
							Ingress: &kubeslicev1beta1.ExternalGatewayConfigOptions{
								Enabled: false,
							},
						},
					},
				},
			},
			expectedEnabled: false,
			expectedPod:     nil,
			expectedError:   false,
		},
		{
			name: "no external gateway config",
			slice: &kubeslicev1beta1.Slice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice",
					Namespace: "kubeslice-system",
				},
				Status: kubeslicev1beta1.SliceStatus{
					SliceConfig: &kubeslicev1beta1.SliceConfig{},
				},
			},
			expectedEnabled: false,
			expectedPod:     nil,
			expectedError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = kubeslicev1beta1.AddToScheme(scheme)
			client := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(tt.slice).Build()

			enabled, pod, err := GetSliceIngressGwPod(context.Background(), client, tt.slice.Name)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedEnabled, enabled)
				if tt.expectedPod != nil {
					assert.NotNil(t, pod)
					assert.Equal(t, tt.expectedPod.PodName, pod.PodName)
				} else {
					assert.Nil(t, pod)
				}
			}
		})
	}
}

func TestGetSliceOverlayNetworkType(t *testing.T) {
	tests := []struct {
		name          string
		slice         *kubeslicev1beta1.Slice
		expected      string
		expectedError bool
	}{
		{
			name: "get network type",
			slice: &kubeslicev1beta1.Slice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice",
					Namespace: "kubeslice-system",
				},
				Status: kubeslicev1beta1.SliceStatus{
					SliceConfig: &kubeslicev1beta1.SliceConfig{
						SliceOverlayNetworkDeploymentMode: "single-network",
					},
				},
			},
			expected:      "single-network",
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = kubeslicev1beta1.AddToScheme(scheme)
			client := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(tt.slice).Build()

			result, err := GetSliceOverlayNetworkType(context.Background(), client, tt.slice.Name)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, string(result))
			}
		})
	}
}
