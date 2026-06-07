package log

import (
	"os"

	"github.com/jiangfire/cornerstone/internal/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logger *zap.Logger

// Logger returns the global logger instance
func Logger() *zap.Logger {
	if logger == nil {
		panic("zap logger not initialized")
	}
	return logger
}

// GetLogger compatibility function, returns the logger instance
func GetLogger() *zap.Logger {
	return Logger()
}

// InitLogger initializes the logging system.
//
// Design tradeoffs (2026-05 P3-6):
//   - The standard practice for containerized deployments is structured logs to stdout, with the orchestration layer (docker/k8s/journald) handling rotation and aggregation;
//     Application file writes + lumberjack rotation would duplicate external sidecar functionality and require writable volumes.
//   - Therefore only output one JSON stream to stdout, no separate files, no level bypass (errors can still be filtered by collector with level=error).
//   - Level field is retained on LoggerConfig, other fields have been removed from the config layer.
func InitLogger(cfg config.LoggerConfig) error {
	level, err := zapcore.ParseLevel(cfg.Level)
	if err != nil {
		return err
	}

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(os.Stdout),
		level,
	)

	logger = zap.New(core,
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)

	zap.ReplaceGlobals(logger)

	return nil
}

// Sync flushes the log buffer
func Sync() {
	if logger != nil {
		_ = logger.Sync()
	}
}

// Info logs an info-level message
func Info(msg string, fields ...zap.Field) {
	Logger().Info(msg, fields...)
}

// Error logs an error-level message
func Error(msg string, fields ...zap.Field) {
	Logger().Error(msg, fields...)
}

// Warn logs a warn-level message
func Warn(msg string, fields ...zap.Field) {
	Logger().Warn(msg, fields...)
}

// Debug logs a debug-level message
func Debug(msg string, fields ...zap.Field) {
	Logger().Debug(msg, fields...)
}

// Fatal logs a fatal-level message and exits
func Fatal(msg string, fields ...zap.Field) {
	Logger().Fatal(msg, fields...)
}

// Infof formats and logs an info message
func Infof(format string, args ...any) {
	Logger().Sugar().Infof(format, args...)
}

// Errorf formats and logs an error message
func Errorf(format string, args ...any) {
	Logger().Sugar().Errorf(format, args...)
}

// Fatalf formats and logs a fatal message and exits
func Fatalf(format string, args ...any) {
	Logger().Sugar().Fatalf(format, args...)
}
