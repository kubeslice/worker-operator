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

package featureflag_test

import (
	"github.com/kubeslice/worker-operator/pkg/featureflag"
	"os"
	"testing"
)

func TestFeatureFlag(t *testing.T) {
	cases := []struct {
		desc           string
		feature        string
		featureEnv     string
		featureEnabled string
		expected       bool
	}{
		{"TestNoEnv", "abc", "", "", false},
		{"TestEnv", "abc", "FEATURE_ABC", "true", true},
		{"TestUppercase", "ABC", "FEATURE_ABC", "true", true},
		{"TestInvalidFormat", "abc", "ABC", "true", false},
		{"TestFalse", "abc", "FEATURE_ABC", "false", false},
	}

	for _, tc := range cases {
		os.Setenv(tc.featureEnv, tc.featureEnabled)
		actual := featureflag.IsEnabled(tc.feature)
		if actual != tc.expected {
			t.Fatalf("%s: expected: %t got: %t for feature: %s env: %s", tc.desc, tc.expected, actual, tc.feature, tc.featureEnv)
		}
		os.Unsetenv(tc.featureEnv)
	}

}
