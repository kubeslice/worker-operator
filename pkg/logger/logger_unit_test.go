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

package logger

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewLogger(t *testing.T) {
	tests := []struct {
		name     string
		logLevel string
	}{
		{
			name:     "create logger with DEBUG level",
			logLevel: "DEBUG",
		},
		{
			name:     "create logger with INFO level",
			logLevel: "INFO",
		},
		{
			name:     "create logger with WARNING level",
			logLevel: "WARNING",
		},
		{
			name:     "create logger with ERROR level",
			logLevel: "ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.logLevel != "" {
				os.Setenv("LOG_LEVEL", tt.logLevel)
				defer os.Unsetenv("LOG_LEVEL")
			}

			logger := NewLogger()
			assert.NotNil(t, logger)
		})
	}
}

func TestNewWrappedLogger(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "create wrapped logger",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := NewWrappedLogger()
			assert.NotNil(t, logger)
		})
	}
}

func TestWithLogger(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "add logger to context",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := NewWrappedLogger()
			ctx := context.Background()

			newCtx := WithLogger(ctx, logger)
			assert.NotNil(t, newCtx)

			retrievedLogger := FromContext(newCtx)
			assert.NotNil(t, retrievedLogger)
		})
	}
}

func TestFromContext(t *testing.T) {
	tests := []struct {
		name             string
		setupContext     func() context.Context
		expectDefaultLog bool
	}{
		{
			name: "get logger from context",
			setupContext: func() context.Context {
				logger := NewWrappedLogger()
				return WithLogger(context.Background(), logger)
			},
			expectDefaultLog: false,
		},
		{
			name: "get default logger when not in context",
			setupContext: func() context.Context {
				return context.Background()
			},
			expectDefaultLog: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setupContext()
			logger := FromContext(ctx)
			assert.NotNil(t, logger)
		})
	}
}

func TestLogLevelSeverity(t *testing.T) {
	tests := []struct {
		name     string
		logLevel string
		setup    func()
	}{
		{
			name:     "default log level INFO when not set",
			logLevel: "",
			setup: func() {
				os.Unsetenv("LOG_LEVEL")
			},
		},
		{
			name:     "use DEBUG log level",
			logLevel: "DEBUG",
			setup: func() {
				os.Setenv("LOG_LEVEL", "DEBUG")
			},
		},
		{
			name:     "use ERROR log level",
			logLevel: "ERROR",
			setup: func() {
				os.Setenv("LOG_LEVEL", "ERROR")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			defer os.Unsetenv("LOG_LEVEL")

			logger := NewLogger()
			assert.NotNil(t, logger)
		})
	}
}

func TestLoggerWithClusterName(t *testing.T) {
	tests := []struct {
		name        string
		clusterName string
	}{
		{
			name:        "logger with cluster name set",
			clusterName: "test-cluster",
		},
		{
			name:        "logger without cluster name",
			clusterName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.clusterName != "" {
				os.Setenv("CLUSTER_NAME", tt.clusterName)
				defer os.Unsetenv("CLUSTER_NAME")
			} else {
				os.Unsetenv("CLUSTER_NAME")
			}

			logger := NewLogger()
			assert.NotNil(t, logger)
		})
	}
}
