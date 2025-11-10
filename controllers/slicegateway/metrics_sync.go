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

package slicegateway

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	spokev1alpha1 "github.com/kubeslice/apis/pkg/worker/v1alpha1"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	hub "github.com/kubeslice/worker-operator/pkg/hub/hubclient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
)

// Configuration for metrics sync (can be overridden via environment variables)
var (
	// Normal sync thresholds (rate limited)
	MinLatencySyncInterval    = getDurationFromEnv("LATENCY_SYNC_MIN_INTERVAL_SECONDS", 5*time.Minute)
	LatencyChangeThreshold    = getUint64FromEnv("LATENCY_CHANGE_THRESHOLD_MS", 10)
	ThroughputChangeThreshold = getUint64FromEnv("THROUGHPUT_CHANGE_THRESHOLD_BYTES", 1024*1024)
	MaxStalenessInterval      = getDurationFromEnv("LATENCY_SYNC_MAX_STALENESS_SECONDS", 15*time.Minute)

	// Critical thresholds (bypass rate limiting)
	CriticalLatencyThreshold    = getUint64FromEnv("CRITICAL_LATENCY_THRESHOLD_MS", 100)
	SevereLatencyThreshold      = getUint64FromEnv("SEVERE_LATENCY_THRESHOLD_MS", 200)
	CriticalPacketLossThreshold = getUint64FromEnv("CRITICAL_PACKET_LOSS_PERCENT", 5)
	LatencySpikeMinThreshold    = getUint64FromEnv("LATENCY_SPIKE_MIN_MS", 50)
)

// GatewayMetrics represents aggregated metrics from gateway pods
type GatewayMetrics struct {
	AvgLatency  uint64
	MinLatency  uint64
	MaxLatency  uint64
	AvgRxRate   uint64
	AvgTxRate   uint64
	PacketLoss  uint64
	ActivePods  int32
	LastUpdated metav1.Time
}

// syncMetricsToHub syncs gateway metrics to hub cluster with threshold-based rate limiting
// This is called immediately after updating the worker SliceGateway CR
func (r *SliceGwReconciler) syncMetricsToHub(
	ctx context.Context,
	sliceGw *kubeslicev1beta1.SliceGateway,
	log logr.Logger,
) {
	// Compute current metrics from worker CR
	currentMetrics := computeAggregatedMetrics(sliceGw.Status.GatewayPodStatus)
	if currentMetrics == nil {
		return // No active pods
	}

	// Check if this is a critical threshold breach
	isCritical := checkCriticalThresholds(currentMetrics, log)

	// Find corresponding WorkerSliceGateway in hub
	hubGw := &spokev1alpha1.WorkerSliceGateway{}
	hubGwKey := types.NamespacedName{
		Name:      sliceGw.Name,
		Namespace: hub.ProjectNamespace,  // Use hub project namespace, not worker namespace
	}

	err := r.RawHubClient.Get(ctx, hubGwKey, hubGw)
	if err != nil {
		log.Error(err, "Failed to get WorkerSliceGateway from hub")
		return // Don't fail worker reconciliation
	}

	// Check if we should sync (rate limiting unless critical)
	if !isCritical && !shouldSyncMetrics(hubGw, currentMetrics, log) {
		log.V(1).Info("Skipping metrics sync (rate limited)")
		return
	}

	if isCritical {
		log.Info("⚡ CRITICAL threshold breach - forcing immediate sync",
			"avgLatency", currentMetrics.AvgLatency,
			"packetLoss", currentMetrics.PacketLoss,
		)
	}

	// Update hub WorkerSliceGateway status
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Re-fetch to get latest version
		if err := r.RawHubClient.Get(ctx, hubGwKey, hubGw); err != nil {
			return err
		}

		// Update gateway metrics in hub status
		hubGw.Status.GatewayMetrics = &spokev1alpha1.GatewayMetrics{
			AvgLatency:  currentMetrics.AvgLatency,
			MinLatency:  currentMetrics.MinLatency,
			MaxLatency:  currentMetrics.MaxLatency,
			AvgRxRate:   currentMetrics.AvgRxRate,
			AvgTxRate:   currentMetrics.AvgTxRate,
			PacketLoss:  currentMetrics.PacketLoss,
			ActivePods:  currentMetrics.ActivePods,
			LastUpdated: currentMetrics.LastUpdated,
		}

		// Update hub status
		return r.RawHubClient.Status().Update(ctx, hubGw)
	})

	if err != nil {
		log.Error(err, "Failed to sync metrics to hub")
	} else {
		log.V(1).Info("Successfully synced metrics to hub",
			"avgLatency", currentMetrics.AvgLatency,
			"activePods", currentMetrics.ActivePods,
		)
	}
}

// computeAggregatedMetrics aggregates metrics from all gateway pods
func computeAggregatedMetrics(gwPods []*kubeslicev1beta1.GwPodInfo) *GatewayMetrics {
	if len(gwPods) == 0 {
		return nil
	}

	var totalLatency, totalRxRate, totalTxRate, totalPacketLoss uint64
	var minLatency, maxLatency uint64
	var activePods int32

	minLatency = ^uint64(0) // max uint64

	for _, pod := range gwPods {
		if pod == nil {
			continue
		}

		// Only include pods with active tunnels
		// Check TunnelState instead of Status because sidecar sets TunnelState, not Status
		if pod.TunnelStatus.TunnelState == "UP" {
			activePods++
			latency := pod.TunnelStatus.Latency
			rxRate := pod.TunnelStatus.RxRate
			txRate := pod.TunnelStatus.TxRate
			packetLoss := pod.TunnelStatus.PacketLoss

			totalLatency += latency
			totalRxRate += rxRate
			totalTxRate += txRate
			totalPacketLoss += packetLoss

			if latency < minLatency {
				minLatency = latency
			}
			if latency > maxLatency {
				maxLatency = latency
			}
		}
	}

	if activePods == 0 {
		return nil
	}

	return &GatewayMetrics{
		AvgLatency:  totalLatency / uint64(activePods),
		MinLatency:  minLatency,
		MaxLatency:  maxLatency,
		AvgRxRate:   totalRxRate / uint64(activePods),
		AvgTxRate:   totalTxRate / uint64(activePods),
		PacketLoss:  totalPacketLoss / uint64(activePods),
		ActivePods:  activePods,
		LastUpdated: metav1.Now(),
	}
}

// checkCriticalThresholds determines if metrics breach critical thresholds
func checkCriticalThresholds(metrics *GatewayMetrics, log logr.Logger) bool {
	if metrics == nil {
		return false
	}

	// Severe latency
	if metrics.AvgLatency >= SevereLatencyThreshold {
		log.Info("🔴 SEVERE latency threshold breached",
			"current", metrics.AvgLatency,
			"threshold", SevereLatencyThreshold,
		)
		return true
	}

	// Critical latency
	if metrics.AvgLatency >= CriticalLatencyThreshold {
		log.Info("🟠 CRITICAL latency threshold breached",
			"current", metrics.AvgLatency,
			"threshold", CriticalLatencyThreshold,
		)
		return true
	}

	// Critical packet loss
	if metrics.PacketLoss >= CriticalPacketLossThreshold {
		log.Info("🔴 CRITICAL packet loss threshold breached",
			"current", metrics.PacketLoss,
			"threshold", CriticalPacketLossThreshold,
		)
		return true
	}

	// All pods down
	if metrics.ActivePods == 0 {
		log.Info("🔴 CRITICAL: All gateway pods DOWN")
		return true
	}

	// High latency variance (network instability)
	if metrics.MaxLatency > 0 && metrics.MinLatency > 0 {
		spread := metrics.MaxLatency - metrics.MinLatency
		if spread > metrics.MinLatency*2 && metrics.MaxLatency > LatencySpikeMinThreshold {
			log.Info("🟠 WARNING: High latency variance (unstable network)",
				"min", metrics.MinLatency,
				"max", metrics.MaxLatency,
				"spread", spread,
			)
			return true
		}
	}

	return false
}

// shouldSyncMetrics checks if metrics should be synced based on rate limiting
func shouldSyncMetrics(
	hubGw *spokev1alpha1.WorkerSliceGateway,
	currentMetrics *GatewayMetrics,
	log logr.Logger,
) bool {
	// First sync - no previous metrics
	if hubGw.Status.GatewayMetrics == nil {
		return true
	}

	lastSynced := hubGw.Status.GatewayMetrics.LastUpdated.Time

	// Check time-based rate limit
	if time.Since(lastSynced) < MinLatencySyncInterval {
		return false
	}

	// Check if latency changed significantly
	latencyDelta := absUint64(currentMetrics.AvgLatency, hubGw.Status.GatewayMetrics.AvgLatency)
	if latencyDelta > LatencyChangeThreshold {
		return true
	}

	// Check if throughput changed significantly
	rxDelta := absUint64(currentMetrics.AvgRxRate, hubGw.Status.GatewayMetrics.AvgRxRate)
	txDelta := absUint64(currentMetrics.AvgTxRate, hubGw.Status.GatewayMetrics.AvgTxRate)
	if rxDelta > ThroughputChangeThreshold || txDelta > ThroughputChangeThreshold {
		return true
	}

	// Force sync if too stale
	if time.Since(lastSynced) > MaxStalenessInterval {
		return true
	}

	return false
}

// Helper functions

func absUint64(a, b uint64) uint64 {
	if a > b {
		return a - b
	}
	return b - a
}

func getDurationFromEnv(key string, defaultValue time.Duration) time.Duration {
	val := os.Getenv(key)
	if val == "" {
		return defaultValue
	}
	seconds, err := strconv.Atoi(val)
	if err != nil {
		return defaultValue
	}
	return time.Duration(seconds) * time.Second
}

func getUint64FromEnv(key string, defaultValue uint64) uint64 {
	val := os.Getenv(key)
	if val == "" {
		return defaultValue
	}
	intVal, err := strconv.ParseUint(val, 10, 64)
	if err != nil {
		return defaultValue
	}
	return intVal
}
