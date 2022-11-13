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

	"github.com/bombsimon/logrusr/v3"
	"github.com/go-logr/logr"
	"github.com/sirupsen/logrus"
)

var (
	logLevel logrus.Level = logrus.InfoLevel
)

const (
	LogLevelEnv = "LOG_LEVEL"
)

type loggerKey struct{}

func init() {
	if lvl, err := logrus.ParseLevel(os.Getenv(LogLevelEnv)); err == nil {
		logLevel = lvl
	}
}

// WithLogger takes in a context and returns a context with key as loggerKey{} and value as loggger(of type logr.Logger) passed
func WithLogger(ctx context.Context, logger logr.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}

// FromContext returns a logger from the context.
func FromContext(ctx context.Context) logr.Logger {

	if v, ok := ctx.Value(loggerKey{}).(logr.Logger); ok {
		return v
	}

	return NewLogger()
}

// NewLogger Creates a new logr.Logger
func NewLogger() logr.Logger {
	logger := logrus.New()

	logger.SetFormatter(&logrus.JSONFormatter{
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyLevel: "severity",
			logrus.FieldKeyMsg:   "message",
			logrus.FieldKeyTime:  "time",
		},
	})

	logger.SetLevel(logLevel)

	return logrusr.New(logger)
}
