/*
 *  Copyright (c) 2026 Avesha, Inc. All rights reserved.
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

package slicegateway

// remoteNsmSubnetForRoute decides which subnet the slice router should route via
// a gateway. Normally it is the peer gateway's subnet. For a spoke's gateway to
// the hub (routeEntireSliceSubnet), it is the entire slice subnet, so the spoke
// forwards all slice-internal traffic (including traffic for other spokes) to the
// hub. ready is false when the entire-slice route is requested but the slice
// subnet is not known yet, signalling the caller to requeue.
func remoteNsmSubnetForRoute(routeEntireSliceSubnet bool, gatewayRemoteSubnet, sliceSubnet string) (subnet string, ready bool) {
	if !routeEntireSliceSubnet {
		return gatewayRemoteSubnet, true
	}
	if sliceSubnet == "" {
		return "", false
	}
	return sliceSubnet, true
}
