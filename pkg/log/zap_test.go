package log

import (
	"testing"

	"github.com/jiangfire/cornerstone/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func resetLogger() {
	logger = nil
}

func TestInitLogger_Success(t *testing.T) {
	resetLogger()
	err := InitLogger(config.LoggerConfig{Level: "info"})
	require.NoError(t, err)
	assert.NotNil(t, logger)
	resetLogger()
}

func TestInitLogger_InvalidLevel(t *testing.T) {
	resetLogger()
	err := InitLogger(config.LoggerConfig{Level: "bogus"})
	assert.Error(t, err)
	resetLogger()
}

func TestLogger_PanicWhenNotInitialized(t *testing.T) {
	resetLogger()
	assert.Panics(t, func() {
		Logger()
	})
}

func TestGetLogger(t *testing.T) {
	resetLogger()
	err := InitLogger(config.LoggerConfig{Level: "info"})
	require.NoError(t, err)
	l := GetLogger()
	assert.NotNil(t, l)
	resetLogger()
}

func TestSync(t *testing.T) {
	resetLogger()
	err := InitLogger(config.LoggerConfig{Level: "info"})
	require.NoError(t, err)
	assert.NotPanics(t, func() {
		Sync()
	})
	resetLogger()
}

func TestSync_NilLogger(t *testing.T) {
	resetLogger()
	assert.NotPanics(t, func() {
		Sync()
	})
}

func TestInfo_WritesLog(t *testing.T) {
	resetLogger()
	err := InitLogger(config.LoggerConfig{Level: "info"})
	require.NoError(t, err)
	assert.NotPanics(t, func() {
		Info("test info message", zap.String("key", "value"))
	})
	resetLogger()
}

func TestError_WritesLog(t *testing.T) {
	resetLogger()
	err := InitLogger(config.LoggerConfig{Level: "info"})
	require.NoError(t, err)
	assert.NotPanics(t, func() {
		Error("test error message", zap.String("key", "value"))
	})
	resetLogger()
}

func TestWarn_WritesLog(t *testing.T) {
	resetLogger()
	err := InitLogger(config.LoggerConfig{Level: "info"})
	require.NoError(t, err)
	assert.NotPanics(t, func() {
		Warn("test warn message", zap.String("key", "value"))
	})
	resetLogger()
}

func TestDebug_WritesLog(t *testing.T) {
	resetLogger()
	err := InitLogger(config.LoggerConfig{Level: "debug"})
	require.NoError(t, err)
	assert.NotPanics(t, func() {
		Debug("test debug message", zap.String("key", "value"))
	})
	resetLogger()
}

func TestInfof_WritesLog(t *testing.T) {
	resetLogger()
	err := InitLogger(config.LoggerConfig{Level: "info"})
	require.NoError(t, err)
	assert.NotPanics(t, func() {
		Infof("formatted %s %d", "msg", 42)
	})
	resetLogger()
}
