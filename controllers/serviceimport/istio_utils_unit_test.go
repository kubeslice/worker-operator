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

package serviceimport

import (
	"testing"

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestVirtualServiceFromAppPodName(t *testing.T) {
	tests := []struct {
		name          string
		serviceimport *kubeslicev1beta1.ServiceImport
		expected      string
	}{
		{
			name: "Simple name",
			serviceimport: &kubeslicev1beta1.ServiceImport{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-service",
				},
			},
			expected: "my-service",
		},
		{
			name: "Name with dashes",
			serviceimport: &kubeslicev1beta1.ServiceImport{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-test-service",
				},
			},
			expected: "my-test-service",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := virtualServiceFromAppPodName(tt.serviceimport)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestVirtualServiceFromEgressName(t *testing.T) {
	tests := []struct {
		name          string
		serviceimport *kubeslicev1beta1.ServiceImport
		expected      string
	}{
		{
			name: "Simple name and namespace",
			serviceimport: &kubeslicev1beta1.ServiceImport{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-service",
					Namespace: "my-namespace",
				},
			},
			expected: "my-service-my-namespace",
		},
		{
			name: "Name with dashes",
			serviceimport: &kubeslicev1beta1.ServiceImport{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-test-service",
					Namespace: "test-ns",
				},
			},
			expected: "my-test-service-test-ns",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := virtualServiceFromEgressName(tt.serviceimport)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCalculateInitialWeight(t *testing.T) {
	tests := []struct {
		name          string
		index         int
		serviceImport *kubeslicev1beta1.ServiceImport
		expected      int32
	}{
		{
			name:  "Equal distribution - 3 endpoints index 0",
			index: 0,
			serviceImport: &kubeslicev1beta1.ServiceImport{
				Status: kubeslicev1beta1.ServiceImportStatus{
					Endpoints: []kubeslicev1beta1.ServiceEndpoint{
						{DNSName: "endpoint1"},
						{DNSName: "endpoint2"},
						{DNSName: "endpoint3"},
					},
				},
			},
			expected: 34,
		},
		{
			name:  "Equal distribution - 3 endpoints index 1",
			index: 1,
			serviceImport: &kubeslicev1beta1.ServiceImport{
				Status: kubeslicev1beta1.ServiceImportStatus{
					Endpoints: []kubeslicev1beta1.ServiceEndpoint{
						{DNSName: "endpoint1"},
						{DNSName: "endpoint2"},
						{DNSName: "endpoint3"},
					},
				},
			},
			expected: 33,
		},
		{
			name:  "Equal distribution - 3 endpoints index 2",
			index: 2,
			serviceImport: &kubeslicev1beta1.ServiceImport{
				Status: kubeslicev1beta1.ServiceImportStatus{
					Endpoints: []kubeslicev1beta1.ServiceEndpoint{
						{DNSName: "endpoint1"},
						{DNSName: "endpoint2"},
						{DNSName: "endpoint3"},
					},
				},
			},
			expected: 33,
		},
		{
			name:  "Two endpoints - index 0",
			index: 0,
			serviceImport: &kubeslicev1beta1.ServiceImport{
				Status: kubeslicev1beta1.ServiceImportStatus{
					Endpoints: []kubeslicev1beta1.ServiceEndpoint{
						{DNSName: "endpoint1"},
						{DNSName: "endpoint2"},
					},
				},
			},
			expected: 50,
		},
		{
			name:  "Two endpoints - index 1",
			index: 1,
			serviceImport: &kubeslicev1beta1.ServiceImport{
				Status: kubeslicev1beta1.ServiceImportStatus{
					Endpoints: []kubeslicev1beta1.ServiceEndpoint{
						{DNSName: "endpoint1"},
						{DNSName: "endpoint2"},
					},
				},
			},
			expected: 50,
		},
		{
			name:  "Single endpoint",
			index: 0,
			serviceImport: &kubeslicev1beta1.ServiceImport{
				Status: kubeslicev1beta1.ServiceImportStatus{
					Endpoints: []kubeslicev1beta1.ServiceEndpoint{
						{DNSName: "endpoint1"},
					},
				},
			},
			expected: 100,
		},
		{
			name:  "Four endpoints - index 0",
			index: 0,
			serviceImport: &kubeslicev1beta1.ServiceImport{
				Status: kubeslicev1beta1.ServiceImportStatus{
					Endpoints: []kubeslicev1beta1.ServiceEndpoint{
						{DNSName: "endpoint1"},
						{DNSName: "endpoint2"},
						{DNSName: "endpoint3"},
						{DNSName: "endpoint4"},
					},
				},
			},
			expected: 25,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateInitialWeight(tt.index, tt.serviceImport)
			assert.Equal(t, tt.expected, result)
		})
	}
}
