package logger

import (
	"context"
	"os"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	uzap "go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	logLevel string
)

var logLevelSeverity = map[string]zapcore.Level{
	"DEBUG":   zapcore.DebugLevel,
	"INFO":    zapcore.InfoLevel,
	"WARNING": zapcore.WarnLevel,
	"ERROR":   zapcore.ErrorLevel,
}

type loggerKey struct{}

func init() {
	logLevel = os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "INFO"
	}
}

//WithLogger takes in a context and returns a context with key as loggerKey{} and value as loggger(of type logr.Logger) passed
func WithLogger(ctx context.Context, logger logr.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}

//FromContext returns a logger from the context.
func FromContext(ctx context.Context) logr.Logger {

	if v, ok := ctx.Value(loggerKey{}).(logr.Logger); ok {
		return v
	}

	return NewLogger()
}

//NewLogger Creates a new logr.Logger
func NewLogger() logr.Logger {

	// info and debug level enabler
	debugInfoLevel := uzap.LevelEnablerFunc(func(level zapcore.Level) bool {
		return level >= logLevelSeverity[logLevel] && level < zapcore.ErrorLevel
	})

	// error and fatal level enabler
	errorFatalLevel := uzap.LevelEnablerFunc(func(level zapcore.Level) bool {
		return level >= zapcore.ErrorLevel
	})

	// write syncers
	stdoutSyncer := zapcore.Lock(os.Stdout)
	stderrSyncer := zapcore.Lock(os.Stderr)

	configLog := uzap.NewProductionEncoderConfig()
	configLog.EncodeTime = func(ts time.Time, encoder zapcore.PrimitiveArrayEncoder) {
		encoder.AppendString(ts.Format(time.RFC3339Nano))
	}

	// tee core
	core := zapcore.NewTee(
		zapcore.NewCore(
			zapcore.NewJSONEncoder(configLog),
			stdoutSyncer,
			debugInfoLevel,
		),
		zapcore.NewCore(
			zapcore.NewJSONEncoder(configLog),
			stderrSyncer,
			errorFatalLevel,
		),
	)

	// finally construct the logger with the tee core
	logger := uzap.New(core)

	return zapr.NewLogger(logger)

}
