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

package slice

import (
	"testing"

	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	"github.com/stretchr/testify/assert"
)

func TestIndexOf(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		element  string
		expected int
	}{
		{
			name:     "Element found at index 0",
			slice:    []string{"a", "b", "c"},
			element:  "a",
			expected: 0,
		},
		{
			name:     "Element found at index 2",
			slice:    []string{"a", "b", "c"},
			element:  "c",
			expected: 2,
		},
		{
			name:     "Element not found",
			slice:    []string{"a", "b", "c"},
			element:  "d",
			expected: -1,
		},
		{
			name:     "Empty slice",
			slice:    []string{},
			element:  "a",
			expected: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := indexOf(tt.slice, tt.element)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExists(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		element  string
		expected bool
	}{
		{
			name:     "Element exists",
			slice:    []string{"apple", "banana", "cherry"},
			element:  "banana",
			expected: true,
		},
		{
			name:     "Element does not exist",
			slice:    []string{"apple", "banana", "cherry"},
			element:  "grape",
			expected: false,
		},
		{
			name:     "Empty slice",
			slice:    []string{},
			element:  "apple",
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
			result := exists(tt.slice, tt.element)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildAppNamespacesList(t *testing.T) {
	tests := []struct {
		name     string
		slice    *kubeslicev1beta1.Slice
		expected []string
	}{
		{
			name: "Multiple namespaces excluding control plane",
			slice: &kubeslicev1beta1.Slice{
				Status: kubeslicev1beta1.SliceStatus{
					SliceConfig: &kubeslicev1beta1.SliceConfig{
						NamespaceIsolationProfile: &kubeslicev1beta1.NamespaceIsolationProfile{
							ApplicationNamespaces: []string{"ns1", "ns2", ControlPlaneNamespace, "ns3"},
						},
					},
				},
			},
			expected: []string{"ns1", "ns2", "ns3"},
		},
		{
			name: "No control plane namespace in list",
			slice: &kubeslicev1beta1.Slice{
				Status: kubeslicev1beta1.SliceStatus{
					SliceConfig: &kubeslicev1beta1.SliceConfig{
						NamespaceIsolationProfile: &kubeslicev1beta1.NamespaceIsolationProfile{
							ApplicationNamespaces: []string{"ns1", "ns2"},
						},
					},
				},
			},
			expected: []string{"ns1", "ns2"},
		},
		{
			name: "Empty namespaces list",
			slice: &kubeslicev1beta1.Slice{
				Status: kubeslicev1beta1.SliceStatus{
					SliceConfig: &kubeslicev1beta1.SliceConfig{
						NamespaceIsolationProfile: &kubeslicev1beta1.NamespaceIsolationProfile{
							ApplicationNamespaces: []string{},
						},
					},
				},
			},
			expected: nil,
		},
		{
			name: "Only control plane namespace",
			slice: &kubeslicev1beta1.Slice{
				Status: kubeslicev1beta1.SliceStatus{
					SliceConfig: &kubeslicev1beta1.SliceConfig{
						NamespaceIsolationProfile: &kubeslicev1beta1.NamespaceIsolationProfile{
							ApplicationNamespaces: []string{ControlPlaneNamespace},
						},
					},
				},
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildAppNamespacesList(tt.slice)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMergeMaps(t *testing.T) {
	tests := []struct {
		name        string
		baseMap     map[string]string
		overrideMap map[string]string
		expected    map[string]string
	}{
		{
			name:        "Both maps have values",
			baseMap:     map[string]string{"a": "1", "b": "2"},
			overrideMap: map[string]string{"b": "3", "c": "4"},
			expected:    map[string]string{"a": "1", "b": "3", "c": "4"},
		},
		{
			name:        "Empty base map",
			baseMap:     map[string]string{},
			overrideMap: map[string]string{"a": "1"},
			expected:    map[string]string{"a": "1"},
		},
		{
			name:        "Empty override map",
			baseMap:     map[string]string{"a": "1"},
			overrideMap: map[string]string{},
			expected:    map[string]string{"a": "1"},
		},
		{
			name:        "Both maps empty",
			baseMap:     map[string]string{},
			overrideMap: map[string]string{},
			expected:    map[string]string{},
		},
		{
			name:        "Override replaces base values",
			baseMap:     map[string]string{"key1": "value1", "key2": "value2"},
			overrideMap: map[string]string{"key1": "newValue1"},
			expected:    map[string]string{"key1": "newValue1", "key2": "value2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeMaps(tt.baseMap, tt.overrideMap)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRemoveEntries(t *testing.T) {
	tests := []struct {
		name     string
		map1     map[string]string
		map2     map[string]string
		expected map[string]string
	}{
		{
			name:     "Remove matching entries",
			map1:     map[string]string{"a": "1", "b": "2", "c": "3"},
			map2:     map[string]string{"a": "1", "c": "3"},
			expected: map[string]string{"b": "2"},
		},
		{
			name:     "No matching entries",
			map1:     map[string]string{"a": "1", "b": "2"},
			map2:     map[string]string{"c": "3", "d": "4"},
			expected: map[string]string{"a": "1", "b": "2"},
		},
		{
			name:     "Key exists but value different",
			map1:     map[string]string{"a": "1", "b": "2"},
			map2:     map[string]string{"a": "2"},
			expected: map[string]string{"a": "1", "b": "2"},
		},
		{
			name:     "Empty map2",
			map1:     map[string]string{"a": "1", "b": "2"},
			map2:     map[string]string{},
			expected: map[string]string{"a": "1", "b": "2"},
		},
		{
			name:     "Remove all entries",
			map1:     map[string]string{"a": "1", "b": "2"},
			map2:     map[string]string{"a": "1", "b": "2"},
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			removeEntries(tt.map1, tt.map2)
			assert.Equal(t, tt.expected, tt.map1)
		})
	}
}
