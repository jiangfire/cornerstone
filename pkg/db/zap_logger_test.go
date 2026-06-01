package db

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gorm.io/gorm/logger"
)

func newTestZapLogger(buf *bytes.Buffer) (*zap.Logger, *ZapLogger) {
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewDevelopmentEncoderConfig()),
		zapcore.AddSync(buf),
		zap.DebugLevel,
	)
	zl := zap.New(core)
	return zl, NewZapLogger(zl).(*ZapLogger)
}

func TestNewZapLogger_Defaults(t *testing.T) {
	buf := &bytes.Buffer{}
	_, zl := newTestZapLogger(buf)

	assert.Equal(t, 200*time.Millisecond, zl.SlowThreshold)
	assert.Equal(t, logger.Info, zl.LogLevel)
	assert.True(t, zl.IgnoreRecordNotFoundError)
	assert.False(t, zl.Colorful)
}

func TestZapLogger_LogMode(t *testing.T) {
	buf := &bytes.Buffer{}
	_, zl := newTestZapLogger(buf)

	newLog := zl.LogMode(logger.Warn)
	newZl, ok := newLog.(*ZapLogger)
	require.True(t, ok)
	assert.Equal(t, logger.Warn, newZl.LogLevel)
	assert.Equal(t, logger.Info, zl.LogLevel)
}

func TestZapLogger_Info(t *testing.T) {
	buf := &bytes.Buffer{}
	_, zl := newTestZapLogger(buf)

	zl.Info(context.Background(), "test info message %s", "arg")

	bufStr := buf.String()
	assert.Contains(t, bufStr, "test info message arg")
}

func TestZapLogger_Info_Silent(t *testing.T) {
	buf := &bytes.Buffer{}
	_, zl := newTestZapLogger(buf)
	zl.LogLevel = logger.Silent

	zl.Info(context.Background(), "should not appear")

	assert.Empty(t, buf.String())
}

func TestZapLogger_Warn(t *testing.T) {
	buf := &bytes.Buffer{}
	_, zl := newTestZapLogger(buf)
	zl.LogLevel = logger.Warn

	zl.Warn(context.Background(), "test warn %d", 1)

	bufStr := buf.String()
	assert.Contains(t, bufStr, "test warn 1")
}

func TestZapLogger_Warn_Silent(t *testing.T) {
	buf := &bytes.Buffer{}
	_, zl := newTestZapLogger(buf)
	zl.LogLevel = logger.Silent

	zl.Warn(context.Background(), "should not appear")

	assert.Empty(t, buf.String())
}

func TestZapLogger_Error(t *testing.T) {
	buf := &bytes.Buffer{}
	_, zl := newTestZapLogger(buf)
	zl.LogLevel = logger.Error

	zl.Error(context.Background(), "test error %s", "fail")

	bufStr := buf.String()
	assert.Contains(t, bufStr, "test error fail")
}

func TestZapLogger_Error_Silent(t *testing.T) {
	buf := &bytes.Buffer{}
	_, zl := newTestZapLogger(buf)
	zl.LogLevel = logger.Silent

	zl.Error(context.Background(), "should not appear")

	assert.Empty(t, buf.String())
}

func TestZapLogger_Trace_WithError(t *testing.T) {
	buf := &bytes.Buffer{}
	_, zl := newTestZapLogger(buf)

	begin := time.Now()
	zl.Trace(context.Background(), begin, func() (string, int64) {
		return "SELECT * FROM users", 0
	}, assert.AnError)

	bufStr := buf.String()
	assert.Contains(t, bufStr, "sql error")
	assert.Contains(t, bufStr, "SELECT * FROM users")
}

func TestZapLogger_Trace_WithSlowQuery(t *testing.T) {
	buf := &bytes.Buffer{}
	_, zl := newTestZapLogger(buf)
	zl.SlowThreshold = 1 * time.Nanosecond

	begin := time.Now().Add(-100 * time.Millisecond)
	zl.Trace(context.Background(), begin, func() (string, int64) {
		return "SELECT SLEEP(1)", 1
	}, nil)

	bufStr := buf.String()
	assert.Contains(t, bufStr, "slow sql")
}

func TestZapLogger_Trace_SilentLevel(t *testing.T) {
	buf := &bytes.Buffer{}
	_, zl := newTestZapLogger(buf)
	zl.LogLevel = logger.Silent

	begin := time.Now()
	zl.Trace(context.Background(), begin, func() (string, int64) {
		return "SELECT 1", 1
	}, nil)

	assert.Empty(t, buf.String())
}

func TestZapLogger_Trace_Normal(t *testing.T) {
	buf := &bytes.Buffer{}
	_, zl := newTestZapLogger(buf)

	begin := time.Now()
	zl.Trace(context.Background(), begin, func() (string, int64) {
		return "SELECT 1", 1
	}, nil)

	bufStr := buf.String()
	assert.Contains(t, bufStr, `"sql"`)
	assert.Contains(t, bufStr, "SELECT 1")
}

func TestZapLogger_Trace_RecordNotFound(t *testing.T) {
	buf := &bytes.Buffer{}
	_, zl := newTestZapLogger(buf)
	zl.IgnoreRecordNotFoundError = true

	begin := time.Now()
	zl.Trace(context.Background(), begin, func() (string, int64) {
		return "SELECT * FROM users WHERE id=999", 0
	}, logger.ErrRecordNotFound)

	bufStr := buf.String()
	assert.Contains(t, bufStr, "record not found")
}

func TestZapLogger_Trace_RecordNotFound_NotIgnored(t *testing.T) {
	buf := &bytes.Buffer{}
	_, zl := newTestZapLogger(buf)
	zl.IgnoreRecordNotFoundError = false

	begin := time.Now()
	zl.Trace(context.Background(), begin, func() (string, int64) {
		return "SELECT * FROM users WHERE id=999", 0
	}, logger.ErrRecordNotFound)

	bufStr := buf.String()
	assert.Contains(t, bufStr, "sql error")
}

func TestZapLevelToGormLevel(t *testing.T) {
	tests := []struct {
		zapLevel   zapcore.Level
		gormLevel  logger.LogLevel
	}{
		{zapcore.DebugLevel, logger.Info},
		{zapcore.InfoLevel, logger.Info},
		{zapcore.WarnLevel, logger.Warn},
		{zapcore.ErrorLevel, logger.Error},
		{zapcore.DPanicLevel, logger.Error},
		{zapcore.PanicLevel, logger.Error},
		{zapcore.FatalLevel, logger.Error},
	}
	for _, tc := range tests {
		result := zapLevelToGormLevel(tc.zapLevel)
		assert.Equal(t, tc.gormLevel, result, "zapLevel=%v", tc.zapLevel)
	}
}
