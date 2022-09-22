/*  Copyright (c) 2022 Avesha, Inc. All rights reserved.
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

package status

import "time"

// Config defines a health Check and it's scheduling timing requirements.
type Config struct {
	// Name of the status check
	Name string
	// Check is the status Check to be scheduled for execution.
	Checker Check
	// Interval is the time between successive executions.
	Interval time.Duration
	// InitialDelay is the time to delay first execution; defaults to zero.
	InitialDelay time.Duration
}

// TunnelInterfaceStatus represents Tunnel Interface Status
type TunnelInterfaceStatus struct {
	NetInterface string `json:"netInterface,omitempty"`
	LocalIP      string `json:"localIp,omitempty"`
	PeerIP       string `json:"peerIp,omitempty"`
	Latency      uint64 `json:"latency,omitempty"`
	TxRate       uint64 `json:"txRate,omitempty"`
	RxRate       uint64 `json:"rxRate,omitempty"`
	PacketLoss   uint64 `json:"packetLoss,omitempty"`
}
