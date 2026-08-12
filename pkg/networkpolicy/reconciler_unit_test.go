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
package networkpolicy

import (
	"context"
	"net"
	"testing"

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		element  string
		expected bool
	}{
		{
			name:     "element exists in slice",
			slice:    []string{"foo", "bar", "baz"},
			element:  "bar",
			expected: true,
		},
		{
			name:     "element does not exist in slice",
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
		{
			name:     "case insensitive match",
			slice:    []string{"Foo", "Bar", "Baz"},
			element:  "foo",
			expected: true,
		},
		{
			name:     "case insensitive no match",
			slice:    []string{"Foo", "Bar", "Baz"},
			element:  "qux",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Contains(&tt.slice, tt.element)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		{
			name:     "loopback IPv4",
			ip:       "127.0.0.1",
			expected: true,
		},
		{
			name:     "RFC1918 10.0.0.0/8",
			ip:       "10.0.0.1",
			expected: true,
		},
		{
			name:     "RFC1918 172.16.0.0/12",
			ip:       "172.16.0.1",
			expected: true,
		},
		{
			name:     "RFC1918 172.31.255.255",
			ip:       "172.31.255.255",
			expected: true,
		},
		{
			name:     "RFC1918 192.168.0.0/16",
			ip:       "192.168.1.1",
			expected: true,
		},
		{
			name:     "link-local 169.254.0.0/16",
			ip:       "169.254.169.254",
			expected: true,
		},
		{
			name:     "IPv6 loopback",
			ip:       "::1",
			expected: true,
		},
		{
			name:     "IPv6 link-local",
			ip:       "fe80::1",
			expected: true,
		},
		{
			name:     "IPv6 unique local",
			ip:       "fc00::1",
			expected: true,
		},
		{
			name:     "public IPv4",
			ip:       "8.8.8.8",
			expected: false,
		},
		{
			name:     "public IPv4 2",
			ip:       "1.1.1.1",
			expected: false,
		},
		{
			name:     "public IPv6",
			ip:       "2001:4860:4860::8888",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &NetpolReconciler{}
			err := r.initPrivateIPBlocks()
			assert.NoError(t, err)

			ip := net.ParseIP(tt.ip)
			result := r.isPrivateIP(ip)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestInitPrivateIPBlocks(t *testing.T) {
	tests := []struct {
		name           string
		expectedLength int
	}{
		{
			name:           "initialize private IP blocks",
			expectedLength: 8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &NetpolReconciler{}
			err := r.initPrivateIPBlocks()
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedLength, len(r.privateIPBlocks))
		})
	}
}

func TestGetAppNamespacesBySliceNameAndLabel(t *testing.T) {
	tests := []struct {
		name             string
		sliceName        string
		selectorLabelKey string
		namespaces       []runtime.Object
		expectedCount    int
		expectedError    bool
	}{
		{
			name:             "get application namespaces",
			sliceName:        "test-slice",
			selectorLabelKey: "kubeslice.io/slice",
			namespaces: []runtime.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-ns-1",
						Labels: map[string]string{
							"kubeslice.io/slice": "test-slice",
						},
					},
				},
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-ns-2",
						Labels: map[string]string{
							"kubeslice.io/slice": "test-slice",
						},
					},
				},
			},
			expectedCount: 2,
			expectedError: false,
		},
		{
			name:             "no matching namespaces",
			sliceName:        "test-slice",
			selectorLabelKey: "kubeslice.io/slice",
			namespaces:       []runtime.Object{},
			expectedCount:    0,
			expectedError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = corev1.AddToScheme(scheme)
			client := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(tt.namespaces...).Build()

			reconciler := &NetpolReconciler{
				Client: client,
			}

			result, err := reconciler.GetAppNamespacesBySliceNameAndLabel(
				context.Background(),
				tt.sliceName,
				tt.selectorLabelKey,
			)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedCount, len(result))
			}
		})
	}
}

func TestGetAllowedNamespacesBySliceNameAndLabel(t *testing.T) {
	tests := []struct {
		name          string
		slice         *kubeslicev1beta1.Slice
		expectedCount int
	}{
		{
			name: "get allowed namespaces",
			slice: &kubeslicev1beta1.Slice{
				Status: kubeslicev1beta1.SliceStatus{
					SliceConfig: &kubeslicev1beta1.SliceConfig{
						NamespaceIsolationProfile: &kubeslicev1beta1.NamespaceIsolationProfile{
							AllowedNamespaces: []string{"allowed-ns-1", "allowed-ns-2"},
						},
					},
				},
			},
			expectedCount: 2,
		},
		{
			name: "no allowed namespaces",
			slice: &kubeslicev1beta1.Slice{
				Status: kubeslicev1beta1.SliceStatus{
					SliceConfig: &kubeslicev1beta1.SliceConfig{
						NamespaceIsolationProfile: &kubeslicev1beta1.NamespaceIsolationProfile{
							AllowedNamespaces: []string{},
						},
					},
				},
			},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reconciler := &NetpolReconciler{}

			result, err := reconciler.GetAllowedNamespacesBySliceNameAndLabel(
				context.Background(),
				tt.slice,
				"kubeslice.io/namespace",
			)

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCount, len(result))
		})
	}
}
