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

package serviceexport

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
			name: "Multiple ports with protocols",
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

func TestContainsString(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		element  string
		expected bool
	}{
		{
			name:     "String exists",
			slice:    []string{"foo", "bar", "baz"},
			element:  "bar",
			expected: true,
		},
		{
			name:     "String does not exist",
			slice:    []string{"foo", "bar", "baz"},
			element:  "qux",
			expected: false,
		},
		{
			name:     "Empty slice",
			slice:    []string{},
			element:  "foo",
			expected: false,
		},
		{
			name:     "Empty string search",
			slice:    []string{"foo", "", "bar"},
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

func TestGetServiceProtocol(t *testing.T) {
	tests := []struct {
		name     string
		se       *kubeslicev1beta1.ServiceExport
		expected kubeslicev1beta1.ServiceProtocol
	}{
		{
			name: "HTTP port",
			se: &kubeslicev1beta1.ServiceExport{
				Spec: kubeslicev1beta1.ServiceExportSpec{
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
			se: &kubeslicev1beta1.ServiceExport{
				Spec: kubeslicev1beta1.ServiceExportSpec{
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
			name: "TCP port",
			se: &kubeslicev1beta1.ServiceExport{
				Spec: kubeslicev1beta1.ServiceExportSpec{
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
			se: &kubeslicev1beta1.ServiceExport{
				Spec: kubeslicev1beta1.ServiceExportSpec{
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
			se: &kubeslicev1beta1.ServiceExport{
				Spec: kubeslicev1beta1.ServiceExportSpec{
					Ports: []kubeslicev1beta1.ServicePort{},
				},
			},
			expected: kubeslicev1beta1.ServiceProtocolTCP,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getServiceProtocol(tt.se)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestArrayContainsString(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		element  string
		expected bool
	}{
		{
			name:     "String exists",
			slice:    []string{"alpha", "beta", "gamma"},
			element:  "beta",
			expected: true,
		},
		{
			name:     "String does not exist",
			slice:    []string{"alpha", "beta", "gamma"},
			element:  "delta",
			expected: false,
		},
		{
			name:     "Empty slice",
			slice:    []string{},
			element:  "alpha",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := arrayContainsString(tt.slice, tt.element)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsServiceAppPodChanged(t *testing.T) {
	tests := []struct {
		name     string
		current  []kubeslicev1beta1.ServicePod
		old      []kubeslicev1beta1.ServicePod
		expected bool
	}{
		{
			name: "No change",
			current: []kubeslicev1beta1.ServicePod{
				{Name: "pod1", NsmIP: "10.0.0.1", PodIp: "192.168.0.1"},
				{Name: "pod2", NsmIP: "10.0.0.2", PodIp: "192.168.0.2"},
			},
			old: []kubeslicev1beta1.ServicePod{
				{Name: "pod1", NsmIP: "10.0.0.1", PodIp: "192.168.0.1"},
				{Name: "pod2", NsmIP: "10.0.0.2", PodIp: "192.168.0.2"},
			},
			expected: false,
		},
		{
			name: "NSM IP changed",
			current: []kubeslicev1beta1.ServicePod{
				{Name: "pod1", NsmIP: "10.0.0.10", PodIp: "192.168.0.1"},
			},
			old: []kubeslicev1beta1.ServicePod{
				{Name: "pod1", NsmIP: "10.0.0.1", PodIp: "192.168.0.1"},
			},
			expected: true,
		},
		{
			name: "Pod IP changed",
			current: []kubeslicev1beta1.ServicePod{
				{Name: "pod1", NsmIP: "10.0.0.1", PodIp: "192.168.0.10"},
			},
			old: []kubeslicev1beta1.ServicePod{
				{Name: "pod1", NsmIP: "10.0.0.1", PodIp: "192.168.0.1"},
			},
			expected: true,
		},
		{
			name: "Different number of pods",
			current: []kubeslicev1beta1.ServicePod{
				{Name: "pod1", NsmIP: "10.0.0.1", PodIp: "192.168.0.1"},
				{Name: "pod2", NsmIP: "10.0.0.2", PodIp: "192.168.0.2"},
			},
			old: []kubeslicev1beta1.ServicePod{
				{Name: "pod1", NsmIP: "10.0.0.1", PodIp: "192.168.0.1"},
			},
			expected: true,
		},
		{
			name:     "Both empty",
			current:  []kubeslicev1beta1.ServicePod{},
			old:      []kubeslicev1beta1.ServicePod{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isServiceAppPodChanged(tt.current, tt.old)
			assert.Equal(t, tt.expected, result)
		})
	}
}
