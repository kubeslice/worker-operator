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

package featureflag

import (
	"os"
	"strings"
)

// Check if a feature flag is enabled
// for a feature xxx to be enabled, an environment variable FEATURE_XXX=true is expected
func IsEnabled(feature string) bool {
	f := os.Getenv("FEATURE_" + strings.ToUpper(feature))
	return strings.ToLower(f) == "true"
}
