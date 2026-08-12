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

package events

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
)

type MockK8sEventRecorder struct {
	mock.Mock
}

func (m *MockK8sEventRecorder) Event(object runtime.Object, eventtype, reason, message string) {
	m.Called(object, eventtype, reason, message)
}

func (m *MockK8sEventRecorder) Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{}) {
	m.Called(object, eventtype, reason, messageFmt, args)
}

func (m *MockK8sEventRecorder) AnnotatedEventf(object runtime.Object, annotations map[string]string, eventtype, reason, messageFmt string, args ...interface{}) {
	m.Called(object, annotations, eventtype, reason, messageFmt, args)
}

var _ record.EventRecorder = &MockK8sEventRecorder{}

func TestNewEventRecorder(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "create new event recorder",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRecorder := new(MockK8sEventRecorder)
			result := NewEventRecorder(mockRecorder)

			assert.NotNil(t, result)
			assert.Equal(t, mockRecorder, result.Recorder)
		})
	}
}

func TestEventRecorderRecord(t *testing.T) {
	tests := []struct {
		name      string
		event     *Event
		eventType EventType
		reason    string
		message   string
	}{
		{
			name: "record warning event",
			event: &Event{
				Object:    nil,
				EventType: EventTypeWarning,
				Reason:    "TestReason",
				Message:   "Test message",
			},
			eventType: EventTypeWarning,
			reason:    "TestReason",
			message:   "Test message",
		},
		{
			name: "record normal event",
			event: &Event{
				Object:    nil,
				EventType: EventTypeNormal,
				Reason:    "NormalReason",
				Message:   "Normal message",
			},
			eventType: EventTypeNormal,
			reason:    "NormalReason",
			message:   "Normal message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRecorder := new(MockK8sEventRecorder)
			mockRecorder.On("Event", tt.event.Object, string(tt.eventType), tt.reason, tt.message).Return()

			recorder := &EventRecorder{
				Recorder: mockRecorder,
			}

			recorder.Record(tt.event)

			mockRecorder.AssertExpectations(t)
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
