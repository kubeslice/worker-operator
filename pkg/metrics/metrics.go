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
package metrics

import (
	"github.com/kubeslice/worker-operator/pkg/logger"
	"github.com/kubeslice/worker-operator/pkg/monitoring"
)

// Declare labels
var (
	ClusterName = monitoring.MustCreateLabel("clusterName")
	Namespace   = monitoring.MustCreateLabel("namespace")
	Slice       = monitoring.MustCreateLabel("slice")
	Service     = monitoring.MustCreateLabel("service")
)

/*
Declaration of each metric
*/
var (
	metricLog = logger.NewWrappedLogger().WithValues("type", "metrics")
	// appPodsGauge is a prometheus metric which is a gauge of no. of app pods.
	appPodsGauge = monitoring.NewGauge(
		"kubeslice_slice_app_pods",
		"No. of app pods in slice",
		monitoring.WithLabels(ClusterName, Slice, Namespace),
	)
	serviceExportAvailableEndpointsGauge = monitoring.NewGauge(
		"kubeslice_service_export_available_endpoints",
		"No. of service exports avaialble endpoints in slice",
		monitoring.WithLabels(ClusterName, Slice, Namespace, Service),
	)
)

/*
Helper methods to update metrics from reconcilers
*/

// RecordServicecExportAvailableEndpointsCount records currently active serviceexports endpoints
func RecordServicecExportAvailableEndpointsCount(count int, clusterName, slice, ns, svc string) {
	metricLog.Info("Recording serviceexport available endpoint", "count", count, "clusterName", clusterName, "slice", slice, "ns", ns, "svc", svc)
	serviceExportAvailableEndpointsGauge.
		With(ClusterName.Value(clusterName), Slice.Value(slice), Namespace.Value(ns), Service.Value(svc)).
		Record(float64(count))
}

// method to register metrics to prometheus
func init() {
	// Register custom metrics with the global prometheus registry
	monitoring.MustRegister(
		appPodsGauge,
		serviceExportAvailableEndpointsGauge,
	)
}
