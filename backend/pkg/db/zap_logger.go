package db

import (
	"context"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gorm.io/gorm/logger"
)

// ZapLogger implements gorm logger.Interface using zap
type ZapLogger struct {
	logger                    *zap.Logger
	SlowThreshold             time.Duration
	LogLevel                  logger.LogLevel
	IgnoreRecordNotFoundError bool
	Colorful                  bool
}

// zapLevelToGormLevel converts a zap level to a gorm level
func zapLevelToGormLevel(level zapcore.Level) logger.LogLevel {
	switch level {
	case zapcore.DebugLevel:
		return logger.Info
	case zapcore.InfoLevel:
		return logger.Info
	case zapcore.WarnLevel:
		return logger.Warn
	case zapcore.ErrorLevel:
		return logger.Error
	case zapcore.DPanicLevel:
		return logger.Error
	case zapcore.PanicLevel:
		return logger.Error
	case zapcore.FatalLevel:
		return logger.Error
	default:
		return logger.Info
	}

}

// NewZapLogger creates a new ZapLogger instance
func NewZapLogger(logger *zap.Logger) logger.Interface {
	return &ZapLogger{
		logger:                    logger,
		SlowThreshold:             200 * time.Millisecond,
		LogLevel:                  zapLevelToGormLevel(zap.InfoLevel),
		IgnoreRecordNotFoundError: true,
		Colorful:                  false,
	}
}

// LogMode sets the log level
func (l *ZapLogger) LogMode(level logger.LogLevel) logger.Interface {
	newLogger := *l
	newLogger.LogLevel = level
	return &newLogger
}

// Info logs info messages
func (l *ZapLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Info {
		l.logger.Sugar().Infof(msg, data...)
	}
}

// Warn logs warning messages
func (l *ZapLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Warn {
		l.logger.Sugar().Warnf(msg, data...)
	}
}

// Error logs error messages
func (l *ZapLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Error {
		l.logger.Sugar().Errorf(msg, data...)
	}
}

// Trace logs SQL queries and their execution time
func (l *ZapLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.LogLevel <= logger.Silent {
		return
	}

	elapsed := time.Since(begin)
	sql, rows := fc()

	fields := []zap.Field{
		zap.String("sql", sql),
		zap.Duration("duration", elapsed),
		zap.Int64("rows", rows),
	}

	switch {
	case err != nil && l.LogLevel >= logger.Error:
		// SQL error
		fields = append(fields, zap.Error(err))
		if l.IgnoreRecordNotFoundError && err == logger.ErrRecordNotFound {
			l.logger.Debug("record not found", fields...)
		} else {
			l.logger.Error("sql error", fields...)
		}

	case elapsed > l.SlowThreshold && l.SlowThreshold != 0 && l.LogLevel >= logger.Warn:
		// Slow SQL
		fields = append(fields, zap.Duration("slow_threshold", l.SlowThreshold))
		l.logger.Warn("slow sql", fields...)

	case l.LogLevel == logger.Info:
		l.logger.Info("sql", fields...)
	}
}
