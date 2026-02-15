package log

import (
	"os"
	"path/filepath"

	"github.com/jiangfire/cornerstone/backend/internal/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var logger *zap.Logger

// Logger 返回全局logger实例
func Logger() *zap.Logger {
	if logger == nil {
		panic("zap logger not initialized")
	}
	return logger
}

// GetLogger 兼容函数，返回logger实例
func GetLogger() *zap.Logger {
	return Logger()
}

// InitLogger 初始化日志系统
func InitLogger(cfg config.LoggerConfig) error {
	// 创建日志目录
	if err := os.MkdirAll(filepath.Dir(cfg.OutputPath), 0750); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(cfg.ErrorPath), 0750); err != nil {
		return err
	}

	level, err := zapcore.ParseLevel(cfg.Level)
	if err != nil {
		return err
	}

	// 配置日志轮转
	appLogger := &lumberjack.Logger{
		Filename:   cfg.OutputPath,
		MaxSize:    cfg.MaxSize,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAge,
		Compress:   true,
	}

	errorLogger := &lumberjack.Logger{
		Filename:   cfg.ErrorPath,
		MaxSize:    cfg.MaxSize,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAge,
		Compress:   true,
	}

	// 编码器配置
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

	// 创建核心，同时输出到文件和控制台
	core := zapcore.NewTee(
		// 应用日志
		zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			zapcore.AddSync(appLogger),
			level,
		),
		// 错误日志（只记录Error及以上级别）
		zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			zapcore.AddSync(errorLogger),
			zapcore.ErrorLevel,
		),
		// 控制台输出
		zapcore.NewCore(
			zapcore.NewConsoleEncoder(encoderConfig),
			zapcore.AddSync(os.Stdout),
			level,
		),
	)

	// 创建logger
	logger = zap.New(core,
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)

	// 替换全局logger
	zap.ReplaceGlobals(logger)

	return nil
}

// Sync 同步日志缓冲区
func Sync() {
	if logger != nil {
		_ = logger.Sync()
	}
}

// Info 记录info级别日志
func Info(msg string, fields ...zap.Field) {
	Logger().Info(msg, fields...)
}

// Error 记录error级别日志
func Error(msg string, fields ...zap.Field) {
	Logger().Error(msg, fields...)
}

// Warn 记录warn级别日志
func Warn(msg string, fields ...zap.Field) {
	Logger().Warn(msg, fields...)
}

// Debug 记录debug级别日志
func Debug(msg string, fields ...zap.Field) {
	Logger().Debug(msg, fields...)
}

// Fatal 记录fatal级别日志并退出
func Fatal(msg string, fields ...zap.Field) {
	Logger().Fatal(msg, fields...)
}

// Infof 格式化info日志
func Infof(format string, args ...interface{}) {
	Logger().Sugar().Infof(format, args...)
}

// Errorf 格式化error日志
func Errorf(format string, args ...interface{}) {
	Logger().Sugar().Errorf(format, args...)
}

// Fatalf 格式化fatal日志并退出
func Fatalf(format string, args ...interface{}) {
	Logger().Sugar().Fatalf(format, args...)
}
