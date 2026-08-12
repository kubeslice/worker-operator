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

package utils

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/kubeslice/kubeslice-monitoring/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestGetEnvOrDefault(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		def      string
		envValue string
		setEnv   bool
		expected string
	}{
		{
			name:     "environment variable set",
			key:      "TEST_VAR",
			def:      "default",
			envValue: "test_value",
			setEnv:   true,
			expected: "test_value",
		},
		{
			name:     "environment variable not set",
			key:      "NON_EXISTENT_VAR",
			def:      "default_value",
			envValue: "",
			setEnv:   false,
			expected: "default_value",
		},
		{
			name:     "empty string env value",
			key:      "EMPTY_VAR",
			def:      "default",
			envValue: "",
			setEnv:   true,
			expected: "",
		},
		{
			name:     "empty string default",
			key:      "TEST_VAR_2",
			def:      "",
			envValue: "",
			setEnv:   false,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setEnv {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			}

			result := GetEnvOrDefault(tt.key, tt.def)
			assert.Equal(t, tt.expected, result)
		})
	}
}

type MockEventRecorder struct {
	mock.Mock
}

func (m *MockEventRecorder) RecordEvent(ctx context.Context, event *events.Event) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func (m *MockEventRecorder) WithSlice(slice string) events.EventRecorder {
	args := m.Called(slice)
	return args.Get(0).(events.EventRecorder)
}

func (m *MockEventRecorder) WithNamespace(namespace string) events.EventRecorder {
	args := m.Called(namespace)
	return args.Get(0).(events.EventRecorder)
}

func (m *MockEventRecorder) WithProject(project string) events.EventRecorder {
	args := m.Called(project)
	return args.Get(0).(events.EventRecorder)
}

func (m *MockEventRecorder) WithCluster(cluster string) events.EventRecorder {
	args := m.Called(cluster)
	return args.Get(0).(events.EventRecorder)
}

func (m *MockEventRecorder) WithComponent(component string) events.EventRecorder {
	args := m.Called(component)
	return args.Get(0).(events.EventRecorder)
}

func TestRecordEvent(t *testing.T) {
	tests := []struct {
		name           string
		object         runtime.Object
		relatedObject  runtime.Object
		eventName      events.EventName
		controller     string
		recorderError  error
		expectLogError bool
	}{
		{
			name:           "successful event recording",
			object:         nil,
			relatedObject:  nil,
			eventName:      "TestEvent",
			controller:     "test-controller",
			recorderError:  nil,
			expectLogError: false,
		},
		{
			name:           "event recording with error",
			object:         nil,
			relatedObject:  nil,
			eventName:      "TestEvent",
			controller:     "test-controller",
			recorderError:  errors.New("failed to record event"),
			expectLogError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRecorder := new(MockEventRecorder)
			mockRecorder.On("RecordEvent", mock.Anything, mock.MatchedBy(func(e *events.Event) bool {
				return e.Object == tt.object &&
					e.RelatedObject == tt.relatedObject &&
					e.ReportingInstance == tt.controller &&
					e.Name == tt.eventName
			})).Return(tt.recorderError)

			ctx := context.Background()
			recorder := events.EventRecorder(mockRecorder)

			RecordEvent(ctx, &recorder, tt.object, tt.relatedObject, tt.eventName, tt.controller)

			mockRecorder.AssertExpectations(t)
		})
	}
}
