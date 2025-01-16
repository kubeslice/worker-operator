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

func ListContains[T comparable](li []T, v T) bool {
	for _, val := range li {
		if val == v {
			return true
		}
	}
	return false
}

func ListEqual[T comparable](l1, l2 []T) bool {
	if len(l1) != len(l2) {
		return false
	}
	for _, val := range l1 {
		if !ListContains(l2, val) {
			return false
		}
	}

	return true
}
