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

package manager

import (
	"testing"

	hubv1alpha1 "github.com/kubeslice/apis/pkg/controller/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestShouldProcessVpnKeyRotation(t *testing.T) {
	originalClusterName := ClusterName
	defer func() { ClusterName = originalClusterName }()
	ClusterName = "test-cluster"

	tests := []struct {
		name     string
		vpn      *hubv1alpha1.VpnKeyRotation
		expected bool
	}{
		{
			name: "cluster in rotation list",
			vpn: &hubv1alpha1.VpnKeyRotation{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vpn-rotation-1",
				},
				Spec: hubv1alpha1.VpnKeyRotationSpec{
					Clusters: []string{"test-cluster", "other-cluster"},
				},
			},
			expected: true,
		},
		{
			name: "cluster not in rotation list",
			vpn: &hubv1alpha1.VpnKeyRotation{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vpn-rotation-2",
				},
				Spec: hubv1alpha1.VpnKeyRotationSpec{
					Clusters: []string{"other-cluster", "another-cluster"},
				},
			},
			expected: false,
		},
		{
			name: "empty cluster list",
			vpn: &hubv1alpha1.VpnKeyRotation{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vpn-rotation-3",
				},
				Spec: hubv1alpha1.VpnKeyRotationSpec{
					Clusters: []string{},
				},
			},
			expected: false,
		},
		{
			name: "nil cluster list",
			vpn: &hubv1alpha1.VpnKeyRotation{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vpn-rotation-4",
				},
				Spec: hubv1alpha1.VpnKeyRotationSpec{
					Clusters: nil,
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldProcessVpnKeyRotation(tt.vpn)
			if result != tt.expected {
				t.Errorf("shouldProcessVpnKeyRotation() = %v, want %v", result, tt.expected)
			}
		})
	}
}
