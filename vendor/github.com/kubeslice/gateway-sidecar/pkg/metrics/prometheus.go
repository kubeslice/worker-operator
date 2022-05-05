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

import (
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/kubeslice/gateway-sidecar/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

//create latency metrics which has to be populated when we receive latency from tunnel
var (
	sourceClusterId = os.Getenv("CLUSTER_ID")
	remoteClusterId = os.Getenv("REMOTE_CLUSTER_ID")
	remoteGatewayId = os.Getenv("REMOTE_GATEWAY_ID")
	sourceGatewayId = os.Getenv("GATEWAY_ID")
	sliceName       = os.Getenv("SLICE_NAME")
	namespace       = "kubeslice_system"
	constlabels     = prometheus.Labels{
		"slice_name":              sliceName,
		"source_slice_cluster_id": sourceClusterId,
		"remote_slice_cluster_id": remoteClusterId,
		"source_gateway_id":       sourceGatewayId,
		"remote_gateway_id":       remoteGatewayId,
	}
	LatencyMetrics                = getGaugeMetrics("slicegw_latency", "latency Metrics From Slice Gateway")
	RxRateMetrics                 = getGaugeMetrics("rx_rate", "Rx rate from Slice Gateway.")
	TxRateMetrics                 = getGaugeMetrics("tx_rate", "Tx rate from Slice Gateway.")
	log            *logger.Logger = logger.NewLogger()
)

//common method get gauge metrics alongwith labels
func getGaugeMetrics(name string, help string) prometheus.Gauge {
	return prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace:   namespace,
			Name:        name,
			Help:        help,
			ConstLabels: constlabels,
		})
}

//method to register metrics to prometheus
func StartMetricsCollector(metricCollectorPort string) {
	metricCollectorPort = ":" + metricCollectorPort
	log.Infof("Starting metric collector @ %s", metricCollectorPort)
	rand.Seed(time.Now().Unix())
	histogramVec := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "prom_request_time",
		Help: "Time it has taken to retrieve the metrics",
	}, []string{"time"})

	prometheus.Register(histogramVec)

	prometheus.MustRegister(LatencyMetrics)
	prometheus.MustRegister(RxRateMetrics)
	prometheus.MustRegister(TxRateMetrics)

	http.Handle("/metrics", newHandlerWithHistogram(promhttp.Handler(), histogramVec))

	err := http.ListenAndServe(metricCollectorPort, nil)
	if err != nil {
		log.Errorf("Failed to start metric collector @ %s", metricCollectorPort)
	}
	log.Info("Started Prometheus server at", metricCollectorPort)
}

//send http request
func newHandlerWithHistogram(handler http.Handler, histogram *prometheus.HistogramVec) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()
		status := http.StatusOK

		defer func() {
			histogram.WithLabelValues(fmt.Sprintf("%d", status)).Observe(time.Since(start).Seconds())
		}()

		if req.Method == http.MethodGet {
			handler.ServeHTTP(w, req)
			return
		}
		status = http.StatusBadRequest

		w.WriteHeader(status)
	})
}
