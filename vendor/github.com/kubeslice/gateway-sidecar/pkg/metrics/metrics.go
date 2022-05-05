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

package metrics

// Records latency for this sidecar
func RecordLatencyMetric(latency float64) {
	// Set Latency Gauge in prometheus
	LatencyMetrics.Set(latency)
}

// Records rx bytes for this sidecar
func RecordRxRateMetric(rxRate float64) {
	// Set Rx bytes in prometheus
	RxRateMetrics.Set(rxRate)
}

// Records tx bytes for this sidecar
func RecordTxRateMetric(txRate float64) {
	// Set Tx bytes in prometheus
	TxRateMetrics.Set(txRate)
}
