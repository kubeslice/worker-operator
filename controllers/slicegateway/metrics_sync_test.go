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
	"os"
	"testing"
	"time"

	"github.com/go-logr/logr"
	spokev1alpha1 "github.com/kubeslice/apis/pkg/worker/v1alpha1"
	kubeslicev1beta1 "github.com/kubeslice/worker-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestComputeAggregatedMetrics(t *testing.T) {
	tests := []struct {
		name     string
		gwPods   []*kubeslicev1beta1.GwPodInfo
		expected *GatewayMetrics
	}{
		{
			name:     "nil pods",
			gwPods:   nil,
			expected: nil,
		},
		{
			name:     "empty pods",
			gwPods:   []*kubeslicev1beta1.GwPodInfo{},
			expected: nil,
		},
		{
			name: "single active pod",
			gwPods: []*kubeslicev1beta1.GwPodInfo{
				{
					PodName: "gw-pod-1",
					TunnelStatus: kubeslicev1beta1.TunnelStatus{
						Status:     1, // UP
						Latency:    50,
						RxRate:     1000000,
						TxRate:     2000000,
						PacketLoss: 1,
					},
				},
			},
			expected: &GatewayMetrics{
				AvgLatency:  50,
				MinLatency:  50,
				MaxLatency:  50,
				AvgRxRate:   1000000,
				AvgTxRate:   2000000,
				PacketLoss:  1,
				ActivePods:  1,
			},
		},
		{
			name: "multiple active pods",
			gwPods: []*kubeslicev1beta1.GwPodInfo{
				{
					PodName: "gw-pod-1",
					TunnelStatus: kubeslicev1beta1.TunnelStatus{
						Status:     1, // UP
						Latency:    30,
						RxRate:     1000000,
						TxRate:     2000000,
						PacketLoss: 2,
					},
				},
				{
					PodName: "gw-pod-2",
					TunnelStatus: kubeslicev1beta1.TunnelStatus{
						Status:     1, // UP
						Latency:    70,
						RxRate:     3000000,
						TxRate:     4000000,
						PacketLoss: 4,
					},
				},
			},
			expected: &GatewayMetrics{
				AvgLatency:  50,      // (30 + 70) / 2
				MinLatency:  30,
				MaxLatency:  70,
				AvgRxRate:   2000000, // (1M + 3M) / 2
				AvgTxRate:   3000000, // (2M + 4M) / 2
				PacketLoss:  3,       // (2 + 4) / 2
				ActivePods:  2,
			},
		},
		{
			name: "pods with mixed status (some down)",
			gwPods: []*kubeslicev1beta1.GwPodInfo{
				{
					PodName: "gw-pod-1",
					TunnelStatus: kubeslicev1beta1.TunnelStatus{
						Status:     1, // UP
						Latency:    40,
						RxRate:     1000000,
						TxRate:     2000000,
						PacketLoss: 1,
					},
				},
				{
					PodName: "gw-pod-2",
					TunnelStatus: kubeslicev1beta1.TunnelStatus{
						Status:     0, // DOWN
						Latency:    999,
						RxRate:     999,
						TxRate:     999,
						PacketLoss: 999,
					},
				},
				{
					PodName: "gw-pod-3",
					TunnelStatus: kubeslicev1beta1.TunnelStatus{
						Status:     1, // UP
						Latency:    60,
						RxRate:     3000000,
						TxRate:     4000000,
						PacketLoss: 3,
					},
				},
			},
			expected: &GatewayMetrics{
				AvgLatency:  50,      // (40 + 60) / 2 (ignores pod-2)
				MinLatency:  40,
				MaxLatency:  60,
				AvgRxRate:   2000000,
				AvgTxRate:   3000000,
				PacketLoss:  2, // (1 + 3) / 2
				ActivePods:  2,
			},
		},
		{
			name: "all pods down",
			gwPods: []*kubeslicev1beta1.GwPodInfo{
				{
					PodName: "gw-pod-1",
					TunnelStatus: kubeslicev1beta1.TunnelStatus{
						Status: 0, // DOWN
					},
				},
				{
					PodName: "gw-pod-2",
					TunnelStatus: kubeslicev1beta1.TunnelStatus{
						Status: 0, // DOWN
					},
				},
			},
			expected: nil,
		},
		{
			name: "pods with nil entry",
			gwPods: []*kubeslicev1beta1.GwPodInfo{
				nil,
				{
					PodName: "gw-pod-1",
					TunnelStatus: kubeslicev1beta1.TunnelStatus{
						Status:     1,
						Latency:    50,
						RxRate:     1000000,
						TxRate:     2000000,
						PacketLoss: 1,
					},
				},
				nil,
			},
			expected: &GatewayMetrics{
				AvgLatency:  50,
				MinLatency:  50,
				MaxLatency:  50,
				AvgRxRate:   1000000,
				AvgTxRate:   2000000,
				PacketLoss:  1,
				ActivePods:  1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := computeAggregatedMetrics(tt.gwPods)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("expected nil, got %+v", result)
				}
				return
			}

			if result == nil {
				t.Errorf("expected %+v, got nil", tt.expected)
				return
			}

			if result.AvgLatency != tt.expected.AvgLatency {
				t.Errorf("AvgLatency: expected %d, got %d", tt.expected.AvgLatency, result.AvgLatency)
			}
			if result.MinLatency != tt.expected.MinLatency {
				t.Errorf("MinLatency: expected %d, got %d", tt.expected.MinLatency, result.MinLatency)
			}
			if result.MaxLatency != tt.expected.MaxLatency {
				t.Errorf("MaxLatency: expected %d, got %d", tt.expected.MaxLatency, result.MaxLatency)
			}
			if result.AvgRxRate != tt.expected.AvgRxRate {
				t.Errorf("AvgRxRate: expected %d, got %d", tt.expected.AvgRxRate, result.AvgRxRate)
			}
			if result.AvgTxRate != tt.expected.AvgTxRate {
				t.Errorf("AvgTxRate: expected %d, got %d", tt.expected.AvgTxRate, result.AvgTxRate)
			}
			if result.PacketLoss != tt.expected.PacketLoss {
				t.Errorf("PacketLoss: expected %d, got %d", tt.expected.PacketLoss, result.PacketLoss)
			}
			if result.ActivePods != tt.expected.ActivePods {
				t.Errorf("ActivePods: expected %d, got %d", tt.expected.ActivePods, result.ActivePods)
			}
		})
	}
}

func TestCheckCriticalThresholds(t *testing.T) {
	// Save original values
	origSevere := SevereLatencyThreshold
	origCritical := CriticalLatencyThreshold
	origPacketLoss := CriticalPacketLossThreshold
	origSpikeMin := LatencySpikeMinThreshold

	// Set test values
	SevereLatencyThreshold = 200
	CriticalLatencyThreshold = 100
	CriticalPacketLossThreshold = 5
	LatencySpikeMinThreshold = 50

	// Restore original values after test
	defer func() {
		SevereLatencyThreshold = origSevere
		CriticalLatencyThreshold = origCritical
		CriticalPacketLossThreshold = origPacketLoss
		LatencySpikeMinThreshold = origSpikeMin
	}()

	testLogger := logr.Discard() // Use a no-op logger for tests

	tests := []struct {
		name     string
		metrics  *GatewayMetrics
		expected bool
	}{
		{
			name:     "nil metrics",
			metrics:  nil,
			expected: false,
		},
		{
			name: "normal metrics (no threshold breach)",
			metrics: &GatewayMetrics{
				AvgLatency:  50,
				MinLatency:  40,
				MaxLatency:  60,
				PacketLoss:  1,
				ActivePods:  3,
			},
			expected: false,
		},
		{
			name: "severe latency breach",
			metrics: &GatewayMetrics{
				AvgLatency:  250,
				MinLatency:  240,
				MaxLatency:  260,
				PacketLoss:  1,
				ActivePods:  3,
			},
			expected: true,
		},
		{
			name: "critical latency breach",
			metrics: &GatewayMetrics{
				AvgLatency:  150,
				MinLatency:  140,
				MaxLatency:  160,
				PacketLoss:  1,
				ActivePods:  3,
			},
			expected: true,
		},
		{
			name: "critical packet loss",
			metrics: &GatewayMetrics{
				AvgLatency:  50,
				MinLatency:  40,
				MaxLatency:  60,
				PacketLoss:  10,
				ActivePods:  3,
			},
			expected: true,
		},
		{
			name: "all pods down",
			metrics: &GatewayMetrics{
				AvgLatency:  0,
				MinLatency:  0,
				MaxLatency:  0,
				PacketLoss:  0,
				ActivePods:  0,
			},
			expected: true,
		},
		{
			name: "high latency variance (unstable network)",
			metrics: &GatewayMetrics{
				AvgLatency:  50,
				MinLatency:  10,
				MaxLatency:  100, // spread = 90, minLatency*2 = 20, maxLatency > 50
				PacketLoss:  1,
				ActivePods:  3,
			},
			expected: true,
		},
		{
			name: "moderate variance (not critical)",
			metrics: &GatewayMetrics{
				AvgLatency:  50,
				MinLatency:  40,
				MaxLatency:  60, // spread = 20, minLatency*2 = 80 (not exceeded)
				PacketLoss:  1,
				ActivePods:  3,
			},
			expected: false,
		},
		{
			name: "at critical threshold (exactly 100ms)",
			metrics: &GatewayMetrics{
				AvgLatency:  100,
				MinLatency:  100,
				MaxLatency:  100,
				PacketLoss:  1,
				ActivePods:  3,
			},
			expected: true,
		},
		{
			name: "at packet loss threshold (exactly 5%)",
			metrics: &GatewayMetrics{
				AvgLatency:  50,
				MinLatency:  50,
				MaxLatency:  50,
				PacketLoss:  5,
				ActivePods:  3,
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checkCriticalThresholds(tt.metrics, testLogger)
			if result != tt.expected {
				t.Errorf("expected %v, got %v for metrics: %+v", tt.expected, result, tt.metrics)
			}
		})
	}
}

func TestShouldSyncMetrics(t *testing.T) {
	// Save original values
	origMinInterval := MinLatencySyncInterval
	origLatencyThreshold := LatencyChangeThreshold
	origThroughputThreshold := ThroughputChangeThreshold
	origMaxStaleness := MaxStalenessInterval

	// Set test values
	MinLatencySyncInterval = 5 * time.Minute
	LatencyChangeThreshold = 10
	ThroughputChangeThreshold = 1024 * 1024
	MaxStalenessInterval = 15 * time.Minute

	// Restore original values
	defer func() {
		MinLatencySyncInterval = origMinInterval
		LatencyChangeThreshold = origLatencyThreshold
		ThroughputChangeThreshold = origThroughputThreshold
		MaxStalenessInterval = origMaxStaleness
	}()

	testLogger := logr.Discard()

	tests := []struct {
		name           string
		hubGw          *spokev1alpha1.WorkerSliceGateway
		currentMetrics *GatewayMetrics
		expected       bool
	}{
		{
			name: "first sync - no previous metrics",
			hubGw: &spokev1alpha1.WorkerSliceGateway{
				Status: spokev1alpha1.WorkerSliceGatewayStatus{
					GatewayMetrics: nil,
				},
			},
			currentMetrics: &GatewayMetrics{AvgLatency: 50},
			expected:       true,
		},
		{
			name: "rate limited - synced recently",
			hubGw: &spokev1alpha1.WorkerSliceGateway{
				Status: spokev1alpha1.WorkerSliceGatewayStatus{
					GatewayMetrics: &spokev1alpha1.GatewayMetrics{
						AvgLatency:  50,
						AvgRxRate:   1000000,
						AvgTxRate:   2000000,
						LastUpdated: metav1.NewTime(time.Now().Add(-1 * time.Minute)),
					},
				},
			},
			currentMetrics: &GatewayMetrics{
				AvgLatency: 52, // Small change (2ms < 10ms threshold)
				AvgRxRate:  1000000,
				AvgTxRate:  2000000,
			},
			expected: false,
		},
		{
			name: "latency changed significantly (but rate limited)",
			hubGw: &spokev1alpha1.WorkerSliceGateway{
				Status: spokev1alpha1.WorkerSliceGatewayStatus{
					GatewayMetrics: &spokev1alpha1.GatewayMetrics{
						AvgLatency:  50,
						AvgRxRate:   1000000,
						AvgTxRate:   2000000,
						LastUpdated: metav1.NewTime(time.Now().Add(-1 * time.Minute)),
					},
				},
			},
			currentMetrics: &GatewayMetrics{
				AvgLatency: 65, // 15ms change > 10ms threshold
				AvgRxRate:  1000000,
				AvgTxRate:  2000000,
			},
			expected: false, // Rate limited - time interval not passed
		},
		{
			name: "latency changed significantly (time passed)",
			hubGw: &spokev1alpha1.WorkerSliceGateway{
				Status: spokev1alpha1.WorkerSliceGatewayStatus{
					GatewayMetrics: &spokev1alpha1.GatewayMetrics{
						AvgLatency:  50,
						AvgRxRate:   1000000,
						AvgTxRate:   2000000,
						LastUpdated: metav1.NewTime(time.Now().Add(-6 * time.Minute)),
					},
				},
			},
			currentMetrics: &GatewayMetrics{
				AvgLatency: 65, // 15ms change > 10ms threshold
				AvgRxRate:  1000000,
				AvgTxRate:  2000000,
			},
			expected: true, // Sync - time passed AND significant change
		},
		{
			name: "throughput changed significantly (RxRate, but rate limited)",
			hubGw: &spokev1alpha1.WorkerSliceGateway{
				Status: spokev1alpha1.WorkerSliceGatewayStatus{
					GatewayMetrics: &spokev1alpha1.GatewayMetrics{
						AvgLatency:  50,
						AvgRxRate:   1000000,
						AvgTxRate:   2000000,
						LastUpdated: metav1.NewTime(time.Now().Add(-1 * time.Minute)),
					},
				},
			},
			currentMetrics: &GatewayMetrics{
				AvgLatency: 50,
				AvgRxRate:  3000000, // 2MB/s change > 1MB/s threshold
				AvgTxRate:  2000000,
			},
			expected: false, // Rate limited
		},
		{
			name: "throughput changed significantly (RxRate, time passed)",
			hubGw: &spokev1alpha1.WorkerSliceGateway{
				Status: spokev1alpha1.WorkerSliceGatewayStatus{
					GatewayMetrics: &spokev1alpha1.GatewayMetrics{
						AvgLatency:  50,
						AvgRxRate:   1000000,
						AvgTxRate:   2000000,
						LastUpdated: metav1.NewTime(time.Now().Add(-6 * time.Minute)),
					},
				},
			},
			currentMetrics: &GatewayMetrics{
				AvgLatency: 50,
				AvgRxRate:  3000000, // 2MB/s change > 1MB/s threshold
				AvgTxRate:  2000000,
			},
			expected: true,
		},
		{
			name: "throughput changed significantly (TxRate, but rate limited)",
			hubGw: &spokev1alpha1.WorkerSliceGateway{
				Status: spokev1alpha1.WorkerSliceGatewayStatus{
					GatewayMetrics: &spokev1alpha1.GatewayMetrics{
						AvgLatency:  50,
						AvgRxRate:   1000000,
						AvgTxRate:   2000000,
						LastUpdated: metav1.NewTime(time.Now().Add(-1 * time.Minute)),
					},
				},
			},
			currentMetrics: &GatewayMetrics{
				AvgLatency: 50,
				AvgRxRate:  1000000,
				AvgTxRate:  4000000, // 2MB/s change > 1MB/s threshold
			},
			expected: false, // Rate limited
		},
		{
			name: "throughput changed significantly (TxRate, time passed)",
			hubGw: &spokev1alpha1.WorkerSliceGateway{
				Status: spokev1alpha1.WorkerSliceGatewayStatus{
					GatewayMetrics: &spokev1alpha1.GatewayMetrics{
						AvgLatency:  50,
						AvgRxRate:   1000000,
						AvgTxRate:   2000000,
						LastUpdated: metav1.NewTime(time.Now().Add(-6 * time.Minute)),
					},
				},
			},
			currentMetrics: &GatewayMetrics{
				AvgLatency: 50,
				AvgRxRate:  1000000,
				AvgTxRate:  4000000, // 2MB/s change > 1MB/s threshold
			},
			expected: true,
		},
		{
			name: "forced sync - data too stale",
			hubGw: &spokev1alpha1.WorkerSliceGateway{
				Status: spokev1alpha1.WorkerSliceGatewayStatus{
					GatewayMetrics: &spokev1alpha1.GatewayMetrics{
						AvgLatency:  50,
						AvgRxRate:   1000000,
						AvgTxRate:   2000000,
						LastUpdated: metav1.NewTime(time.Now().Add(-20 * time.Minute)), // > 15 min staleness
					},
				},
			},
			currentMetrics: &GatewayMetrics{
				AvgLatency: 50, // No change, but stale
				AvgRxRate:  1000000,
				AvgTxRate:  2000000,
			},
			expected: true,
		},
		{
			name: "synced 6 minutes ago, small change (no sync - no significant change)",
			hubGw: &spokev1alpha1.WorkerSliceGateway{
				Status: spokev1alpha1.WorkerSliceGatewayStatus{
					GatewayMetrics: &spokev1alpha1.GatewayMetrics{
						AvgLatency:  50,
						AvgRxRate:   1000000,
						AvgTxRate:   2000000,
						LastUpdated: metav1.NewTime(time.Now().Add(-6 * time.Minute)), // > 5 min
					},
				},
			},
			currentMetrics: &GatewayMetrics{
				AvgLatency: 52, // Small change (2ms < 10ms)
				AvgRxRate:  1000000,
				AvgTxRate:  2000000,
			},
			expected: false, // No sync - time passed but no significant change
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldSyncMetrics(tt.hubGw, tt.currentMetrics, testLogger)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestAbsUint64(t *testing.T) {
	tests := []struct {
		name     string
		a        uint64
		b        uint64
		expected uint64
	}{
		{
			name:     "a > b",
			a:        100,
			b:        50,
			expected: 50,
		},
		{
			name:     "b > a",
			a:        50,
			b:        100,
			expected: 50,
		},
		{
			name:     "a == b",
			a:        100,
			b:        100,
			expected: 0,
		},
		{
			name:     "zero values",
			a:        0,
			b:        0,
			expected: 0,
		},
		{
			name:     "one zero",
			a:        100,
			b:        0,
			expected: 100,
		},
		{
			name:     "large values",
			a:        1000000,
			b:        999999,
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := absUint64(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("absUint64(%d, %d): expected %d, got %d", tt.a, tt.b, tt.expected, result)
			}
		})
	}
}

func TestGetDurationFromEnv(t *testing.T) {
	tests := []struct {
		name         string
		envKey       string
		envValue     string
		defaultValue time.Duration
		expected     time.Duration
	}{
		{
			name:         "environment variable not set",
			envKey:       "TEST_DURATION_NOT_SET",
			envValue:     "",
			defaultValue: 5 * time.Minute,
			expected:     5 * time.Minute,
		},
		{
			name:         "valid environment variable",
			envKey:       "TEST_DURATION_VALID",
			envValue:     "300",
			defaultValue: 5 * time.Minute,
			expected:     300 * time.Second,
		},
		{
			name:         "invalid environment variable (non-numeric)",
			envKey:       "TEST_DURATION_INVALID",
			envValue:     "invalid",
			defaultValue: 10 * time.Minute,
			expected:     10 * time.Minute,
		},
		{
			name:         "zero value",
			envKey:       "TEST_DURATION_ZERO",
			envValue:     "0",
			defaultValue: 5 * time.Minute,
			expected:     0 * time.Second,
		},
		{
			name:         "negative value (treated as seconds)",
			envKey:       "TEST_DURATION_NEGATIVE",
			envValue:     "-10",
			defaultValue: 5 * time.Minute,
			expected:     -10 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up environment variable after test
			defer os.Unsetenv(tt.envKey)

			if tt.envValue != "" {
				os.Setenv(tt.envKey, tt.envValue)
			}

			result := getDurationFromEnv(tt.envKey, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGetUint64FromEnv(t *testing.T) {
	tests := []struct {
		name         string
		envKey       string
		envValue     string
		defaultValue uint64
		expected     uint64
	}{
		{
			name:         "environment variable not set",
			envKey:       "TEST_UINT64_NOT_SET",
			envValue:     "",
			defaultValue: 100,
			expected:     100,
		},
		{
			name:         "valid environment variable",
			envKey:       "TEST_UINT64_VALID",
			envValue:     "200",
			defaultValue: 100,
			expected:     200,
		},
		{
			name:         "invalid environment variable (non-numeric)",
			envKey:       "TEST_UINT64_INVALID",
			envValue:     "invalid",
			defaultValue: 150,
			expected:     150,
		},
		{
			name:         "zero value",
			envKey:       "TEST_UINT64_ZERO",
			envValue:     "0",
			defaultValue: 100,
			expected:     0,
		},
		{
			name:         "large value",
			envKey:       "TEST_UINT64_LARGE",
			envValue:     "9999999999",
			defaultValue: 100,
			expected:     9999999999,
		},
		{
			name:         "negative value (invalid for uint64)",
			envKey:       "TEST_UINT64_NEGATIVE",
			envValue:     "-10",
			defaultValue: 100,
			expected:     100, // Falls back to default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up environment variable after test
			defer os.Unsetenv(tt.envKey)

			if tt.envValue != "" {
				os.Setenv(tt.envKey, tt.envValue)
			}

			result := getUint64FromEnv(tt.envKey, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestGatewayMetrics_LastUpdated(t *testing.T) {
	// Test that LastUpdated is set when computing metrics
	gwPods := []*kubeslicev1beta1.GwPodInfo{
		{
			PodName: "gw-pod-1",
			TunnelStatus: kubeslicev1beta1.TunnelStatus{
				Status:  1,
				Latency: 50,
			},
		},
	}

	before := metav1.Now()
	result := computeAggregatedMetrics(gwPods)
	after := metav1.Now()

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// LastUpdated should be between before and after
	if result.LastUpdated.Time.Before(before.Time) {
		t.Errorf("LastUpdated (%v) is before test start (%v)", result.LastUpdated.Time, before.Time)
	}
	if result.LastUpdated.Time.After(after.Time) {
		t.Errorf("LastUpdated (%v) is after test end (%v)", result.LastUpdated.Time, after.Time)
	}
}

