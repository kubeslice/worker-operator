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

package monitoring

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	zap "go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestEventRecorderCopy(t *testing.T) {
	tests := []struct {
		name string
		er   *EventRecorder
	}{
		{
			name: "copy event recorder",
			er: &EventRecorder{
				Version:   "v1.0.0",
				Cluster:   "test-cluster",
				Tenant:    "test-tenant",
				Slice:     "test-slice",
				Namespace: "test-namespace",
				Component: "test-component",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			copy := tt.er.Copy()
			assert.NotNil(t, copy)
			assert.Equal(t, tt.er.Version, copy.Version)
			assert.Equal(t, tt.er.Cluster, copy.Cluster)
			assert.Equal(t, tt.er.Tenant, copy.Tenant)
			assert.Equal(t, tt.er.Slice, copy.Slice)
			assert.Equal(t, tt.er.Namespace, copy.Namespace)
			assert.Equal(t, tt.er.Component, copy.Component)
		})
	}
}

func TestEventRecorderWithSlice(t *testing.T) {
	tests := []struct {
		name      string
		sliceName string
	}{
		{
			name:      "add slice to event recorder",
			sliceName: "test-slice",
		},
		{
			name:      "add different slice",
			sliceName: "another-slice",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			er := &EventRecorder{
				Component: "test-component",
			}

			result := er.WithSlice(tt.sliceName)
			assert.NotNil(t, result)
			assert.Equal(t, tt.sliceName, result.Slice)
			assert.Equal(t, er.Component, result.Component)
		})
	}
}

func TestEventRecorderWithNamespace(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
	}{
		{
			name:      "add namespace to event recorder",
			namespace: "test-namespace",
		},
		{
			name:      "add different namespace",
			namespace: "another-namespace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			er := &EventRecorder{
				Component: "test-component",
			}

			result := er.WithNamespace(tt.namespace)
			assert.NotNil(t, result)
			assert.Equal(t, tt.namespace, result.Namespace)
			assert.Equal(t, er.Component, result.Component)
		})
	}
}

func TestEventTypes(t *testing.T) {
	tests := []struct {
		name      string
		eventType EventType
		expected  string
	}{
		{
			name:      "warning event type",
			eventType: EventTypeWarning,
			expected:  "Warning",
		},
		{
			name:      "normal event type",
			eventType: EventTypeNormal,
			expected:  "Normal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.eventType))
		})
	}
}

func TestEventReasonConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{
			name:     "node IP update reason",
			constant: EventReasonNodeIpUpdate,
			expected: "NodeIpUpdate",
		},
		{
			name:     "node port update reason",
			constant: EventReasonNodePortUpdate,
			expected: "NodePortUpdate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.constant)
		})
	}
}

func TestEventRecorderRecordEvent(t *testing.T) {
	tests := []struct {
		name          string
		event         *Event
		expectedError bool
	}{
		{
			name: "record event successfully",
			event: &Event{
				Object: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "test-namespace",
					},
				},
				EventType:         EventTypeNormal,
				Reason:            "TestReason",
				Message:           "Test message",
				ReportingInstance: "test-instance",
			},
			expectedError: false,
		},
		{
			name: "record event with related object",
			event: &Event{
				Object: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "test-namespace",
					},
				},
				RelatedObject: &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-service",
						Namespace: "test-namespace",
					},
				},
				EventType:         EventTypeWarning,
				Reason:            "WarningReason",
				Message:           "Warning message",
				ReportingInstance: "test-instance",
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = corev1.AddToScheme(scheme)
			
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

			logger, _ := zap.NewDevelopment()
			sugarLogger := logger.Sugar()

			er := &EventRecorder{
				Client:    fakeClient,
				Logger:    sugarLogger,
				Scheme:    scheme,
				Version:   "v1.0.0",
				Cluster:   "test-cluster",
				Tenant:    "test-tenant",
				Slice:     "test-slice",
				Namespace: "test-namespace",
				Component: "test-component",
			}

			err := er.RecordEvent(context.Background(), tt.event)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestEventRecorderRecordEventWithCustomNamespace(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	logger, _ := zap.NewDevelopment()
	sugarLogger := logger.Sugar()

	er := &EventRecorder{
		Client:    fakeClient,
		Logger:    sugarLogger,
		Scheme:    scheme,
		Version:   "v1.0.0",
		Cluster:   "test-cluster",
		Tenant:    "test-tenant",
		Slice:     "test-slice",
		Namespace: "custom-namespace",
		Component: "test-component",
	}

	event := &Event{
		Object: &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pod",
				Namespace: "pod-namespace",
			},
		},
		EventType:         EventTypeNormal,
		Reason:            "TestReason",
		Message:           "Test message",
		ReportingInstance: "test-instance",
	}

	err := er.RecordEvent(context.Background(), event)
	assert.NoError(t, err)
}
