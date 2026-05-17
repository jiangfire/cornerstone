package log

import (
	"os"

	"github.com/jiangfire/cornerstone/backend/internal/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logger *zap.Logger

// Logger 返回全局 logger 实例
func Logger() *zap.Logger {
	if logger == nil {
		panic("zap logger not initialized")
	}
	return logger
}

// GetLogger 兼容函数, 返回 logger 实例
func GetLogger() *zap.Logger {
	return Logger()
}

// InitLogger 初始化日志系统。
//
// 设计取舍 (2026-05 P3-6):
//   - 容器化部署的标准做法是结构化日志走 stdout, 由编排层 (docker/k8s/journald) 接管轮转与聚合;
//     应用自身写文件 + lumberjack 轮转会与外部 sidecar 重复, 还要求挂卷可写。
//   - 因此这里只输出一份 JSON 到 stdout, 不再分文件、不再分级旁路 (error 仍可由采集端过滤 level=error)。
//   - LoggerConfig 上保留 Level 字段, 其余字段已从配置层移除。
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

// Sync 同步日志缓冲区
func Sync() {
	if logger != nil {
		_ = logger.Sync()
	}
}

// Info 记录 info 级别日志
func Info(msg string, fields ...zap.Field) {
	Logger().Info(msg, fields...)
}

// Error 记录 error 级别日志
func Error(msg string, fields ...zap.Field) {
	Logger().Error(msg, fields...)
}

// Warn 记录 warn 级别日志
func Warn(msg string, fields ...zap.Field) {
	Logger().Warn(msg, fields...)
}

// Debug 记录 debug 级别日志
func Debug(msg string, fields ...zap.Field) {
	Logger().Debug(msg, fields...)
}

// Fatal 记录 fatal 级别日志并退出
func Fatal(msg string, fields ...zap.Field) {
	Logger().Fatal(msg, fields...)
}

// Infof 格式化 info 日志
func Infof(format string, args ...any) {
	Logger().Sugar().Infof(format, args...)
}

// Errorf 格式化 error 日志
func Errorf(format string, args ...any) {
	Logger().Sugar().Errorf(format, args...)
}

// Fatalf 格式化 fatal 日志并退出
func Fatalf(format string, args ...any) {
	Logger().Sugar().Fatalf(format, args...)
}
