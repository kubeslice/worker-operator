/*
 *  Copyright (c) 2025 Avesha, Inc. All rights reserved.
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

package hubutils

import "testing"

func TestListContains(t *testing.T) {
	tests := []struct {
		name string
		list []string
		val  string
		want bool
	}{
		{
			name: "element exists",
			list: []string{"a", "b", "c"},
			val:  "b",
			want: true,
		},
		{
			name: "element does not exist",
			list: []string{"a", "b", "c"},
			val:  "d",
			want: false,
		},
		{
			name: "empty list",
			list: []string{},
			val:  "a",
			want: false,
		},
		{
			name: "nil list",
			list: nil,
			val:  "a",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ListContains(tt.list, tt.val); got != tt.want {
				t.Errorf("ListContains() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestListContains_Int(t *testing.T) {
	tests := []struct {
		name string
		list []int
		val  int
		want bool
	}{
		{
			name: "element exists",
			list: []int{1, 2, 3},
			val:  2,
			want: true,
		},
		{
			name: "element does not exist",
			list: []int{1, 2, 3},
			val:  4,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ListContains(tt.list, tt.val); got != tt.want {
				t.Errorf("ListContains() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestListEqual(t *testing.T) {
	tests := []struct {
		name string
		l1   []string
		l2   []string
		want bool
	}{
		{
			name: "equal lists",
			l1:   []string{"a", "b", "c"},
			l2:   []string{"a", "b", "c"},
			want: true,
		},
		{
			name: "equal lists different order",
			l1:   []string{"a", "b", "c"},
			l2:   []string{"c", "a", "b"},
			want: true,
		},
		{
			name: "different lengths",
			l1:   []string{"a", "b"},
			l2:   []string{"a", "b", "c"},
			want: false,
		},
		{
			name: "different elements",
			l1:   []string{"a", "b", "c"},
			l2:   []string{"a", "b", "d"},
			want: false,
		},
		{
			name: "both empty",
			l1:   []string{},
			l2:   []string{},
			want: true,
		},
		{
			name: "both nil",
			l1:   nil,
			l2:   nil,
			want: true,
		},
		{
			name: "one empty one nil",
			l1:   []string{},
			l2:   nil,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ListEqual(tt.l1, tt.l2); got != tt.want {
				t.Errorf("ListEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}
