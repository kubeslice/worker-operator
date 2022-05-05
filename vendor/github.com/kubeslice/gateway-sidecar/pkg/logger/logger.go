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

package logger

import (
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var GlobalLogger *Logger

// Logger : Logger type
type Logger struct {
	handle *zap.SugaredLogger
}

// Debugf : Log level type Debugf
func (logger *Logger) Debugf(format string, args ...interface{}) {
	logger.handle.Debugf(format, args...)
}

// Infof : Log level type Infof
func (logger *Logger) Infof(format string, args ...interface{}) {
	logger.handle.Infof(format, args...)
}

// Errorf : Log level type Errorf
func (logger *Logger) Errorf(format string, args ...interface{}) {
	logger.handle.Errorf(format, args...)
}

// Fatalf : Log level type Fatalf
func (logger *Logger) Fatalf(format string, args ...interface{}) {
	logger.handle.Fatalf(format, args...)
}

// Panicf : Log level type Panicf
func (logger *Logger) Panicf(format string, args ...interface{}) {
	logger.handle.Panicf(format, args...)
}

// Debug : Log level type Debug
func (logger *Logger) Debug(args ...interface{}) {
	logger.handle.Debug(args...)
}

// Info : Log level type Info
func (logger *Logger) Info(args ...interface{}) {
	logger.handle.Info(args...)
}

// Warn : Log level type Warn
func (logger *Logger) Warn(args ...interface{}) {
	logger.handle.Warn(args...)
}

// Error : Log level type Error
func (logger *Logger) Error(args ...interface{}) {
	logger.handle.Error(args...)
}

// Fatal : Log level type Fatal
func (logger *Logger) Fatal(args ...interface{}) {
	logger.handle.Fatal(args...)
}

// Panic : Log level type Panic
func (logger *Logger) Panic(args ...interface{}) {
	logger.handle.Panic(args...)
}

// NewLogger creates the new logger object.
func NewLogger() *Logger {
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "INFO"
	}
	logPath := "avesha-sidecar.log"

	logLevelMap := map[string]zapcore.Level{
		"DEBUG": zapcore.DebugLevel,
		"INFO":  zapcore.InfoLevel,
		"ERROR": zapcore.ErrorLevel,
		"WARN":  zapcore.WarnLevel,
		"FATAL": zapcore.FatalLevel,
		"PANIC": zapcore.PanicLevel}

	// Open the Log file
	logFileHandle, err := os.OpenFile(filepath.Clean(logPath), os.O_CREATE|os.O_APPEND|os.O_RDWR, 0600)
	if err != nil {
		fmt.Println("Failed to open the %s File with error %v", logPath, err)
	}

	logLvl := logLevelMap[logLevel]

	// Get the Write Syncer
	writerSyncer := zapcore.AddSync(logFileHandle)

	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder

	logEncoder := zapcore.NewConsoleEncoder(encoderConfig)
	consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)
	core := zapcore.NewTee(
		zapcore.NewCore(logEncoder, writerSyncer, logLvl),
		zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), logLvl),
	)
	logger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1)).Sugar()

	defer logger.Sync()

	return &Logger{logger}
}
