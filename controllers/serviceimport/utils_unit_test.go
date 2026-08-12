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
	corev1 "k8s.io/api/core/v1"
)

func TestPortListToDisplayString(t *testing.T) {
	tests := []struct {
		name         string
		servicePorts []kubeslicev1beta1.ServicePort
		expected     string
	}{
		{
			name: "Single TCP port",
			servicePorts: []kubeslicev1beta1.ServicePort{
				{
					ContainerPort: 8080,
					Protocol:      corev1.ProtocolTCP,
				},
			},
			expected: "8080/TCP",
		},
		{
			name: "Multiple ports with different protocols",
			servicePorts: []kubeslicev1beta1.ServicePort{
				{
					ContainerPort: 8080,
					Protocol:      corev1.ProtocolTCP,
				},
				{
					ContainerPort: 9090,
					Protocol:      corev1.ProtocolUDP,
				},
			},
			expected: "8080/TCP,9090/UDP",
		},
		{
			name: "Port without protocol defaults to TCP",
			servicePorts: []kubeslicev1beta1.ServicePort{
				{
					ContainerPort: 3000,
				},
			},
			expected: "3000/TCP",
		},
		{
			name:         "Empty port list",
			servicePorts: []kubeslicev1beta1.ServicePort{},
			expected:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := portListToDisplayString(tt.servicePorts)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetServiceProtocol(t *testing.T) {
	tests := []struct {
		name     string
		si       *kubeslicev1beta1.ServiceImport
		expected kubeslicev1beta1.ServiceProtocol
	}{
		{
			name: "HTTP port",
			si: &kubeslicev1beta1.ServiceImport{
				Spec: kubeslicev1beta1.ServiceImportSpec{
					Ports: []kubeslicev1beta1.ServicePort{
						{
							Name:          "http",
							ContainerPort: 8080,
						},
					},
				},
			},
			expected: kubeslicev1beta1.ServiceProtocolHTTP,
		},
		{
			name: "HTTP2 port",
			si: &kubeslicev1beta1.ServiceImport{
				Spec: kubeslicev1beta1.ServiceImportSpec{
					Ports: []kubeslicev1beta1.ServicePort{
						{
							Name:          "http2",
							ContainerPort: 8080,
						},
					},
				},
			},
			expected: kubeslicev1beta1.ServiceProtocolHTTP,
		},
		{
			name: "HTTPS port",
			si: &kubeslicev1beta1.ServiceImport{
				Spec: kubeslicev1beta1.ServiceImportSpec{
					Ports: []kubeslicev1beta1.ServicePort{
						{
							Name:          "https",
							ContainerPort: 443,
						},
					},
				},
			},
			expected: kubeslicev1beta1.ServiceProtocolHTTP,
		},
		{
			name: "TCP port",
			si: &kubeslicev1beta1.ServiceImport{
				Spec: kubeslicev1beta1.ServiceImportSpec{
					Ports: []kubeslicev1beta1.ServicePort{
						{
							Name:          "tcp",
							ContainerPort: 3306,
						},
					},
				},
			},
			expected: kubeslicev1beta1.ServiceProtocolTCP,
		},
		{
			name: "Multiple ports defaults to TCP",
			si: &kubeslicev1beta1.ServiceImport{
				Spec: kubeslicev1beta1.ServiceImportSpec{
					Ports: []kubeslicev1beta1.ServicePort{
						{
							Name:          "http",
							ContainerPort: 8080,
						},
						{
							Name:          "grpc",
							ContainerPort: 9090,
						},
					},
				},
			},
			expected: kubeslicev1beta1.ServiceProtocolTCP,
		},
		{
			name: "No ports defaults to TCP",
			si: &kubeslicev1beta1.ServiceImport{
				Spec: kubeslicev1beta1.ServiceImportSpec{
					Ports: []kubeslicev1beta1.ServicePort{},
				},
			},
			expected: kubeslicev1beta1.ServiceProtocolTCP,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getServiceProtocol(tt.si)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContainsString(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		element  string
		expected bool
	}{
		{
			name:     "String exists",
			slice:    []string{"one", "two", "three"},
			element:  "two",
			expected: true,
		},
		{
			name:     "String does not exist",
			slice:    []string{"one", "two", "three"},
			element:  "four",
			expected: false,
		},
		{
			name:     "Empty slice",
			slice:    []string{},
			element:  "one",
			expected: false,
		},
		{
			name:     "Empty string in slice",
			slice:    []string{"", "a", "b"},
			element:  "",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsString(tt.slice, tt.element)
			assert.Equal(t, tt.expected, result)
		})
	}
}
