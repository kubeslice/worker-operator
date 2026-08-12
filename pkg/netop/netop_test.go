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

package netop

import (
	"reflect"
	"testing"
)

func TestGetRemoteSliceGwNodeIP(t *testing.T) {
	tests := []struct {
		name    string
		nodeIPs []string
		want    string
	}{
		{
			name:    "single IP",
			nodeIPs: []string{"192.168.1.1"},
			want:    "192.168.1.1",
		},
		{
			name:    "multiple IPs returns first",
			nodeIPs: []string{"192.168.1.1", "192.168.1.2", "192.168.1.3"},
			want:    "192.168.1.1",
		},
		{
			name:    "nil slice",
			nodeIPs: nil,
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getRemoteSliceGwNodeIP(tt.nodeIPs); got != tt.want {
				t.Errorf("getRemoteSliceGwNodeIP() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConvertIntSliceToStringSlice(t *testing.T) {
	tests := []struct {
		name     string
		intSlice []int
		want     []string
	}{
		{
			name:     "single element",
			intSlice: []int{8080},
			want:     []string{"8080"},
		},
		{
			name:     "multiple elements",
			intSlice: []int{8080, 9090, 3000},
			want:     []string{"8080", "9090", "3000"},
		},
		{
			name:     "empty slice",
			intSlice: []int{},
			want:     []string{},
		},
		{
			name:     "zero values",
			intSlice: []int{0, 0},
			want:     []string{"0", "0"},
		},
		{
			name:     "negative values",
			intSlice: []int{-1, 100},
			want:     []string{"-1", "100"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertIntSliceToStringSlice(tt.intSlice)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("convertIntSliceToStringSlice() = %v, want %v", got, tt.want)
			}
		})
	}
}
