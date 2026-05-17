package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/backend/internal/config"
	pkgdb "github.com/jiangfire/cornerstone/backend/pkg/db"
	"github.com/stretchr/testify/require"
)

func setupHealthHandlerTestDB(t *testing.T) {
	t.Helper()

	_ = os.Setenv("JWT_SECRET", "health-handler-test-secret")

	dbFile := t.TempDir() + "/health-handler.db"
	require.NoError(t, pkgdb.InitDB(config.DatabaseConfig{
		Type:        "sqlite",
		URL:         dbFile,
		MaxOpen:     1,
		MaxIdle:     1,
		MaxLifetime: 60,
	}))
	t.Cleanup(func() {
		_ = pkgdb.CloseDB()
	})
}

func TestHealthAlwaysReturns200(t *testing.T) {
	gin.SetMode(gin.TestMode)
	SetVersion("test-version")

	r := gin.New()
	r.GET("/health", Health)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, "healthy", body["status"])
	require.Equal(t, "cornerstone-backend", body["service"])
	require.Equal(t, "test-version", body["version"])
	require.NotEmpty(t, body["time"])
}

func TestReadyReturns200WhenDatabaseReachable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupHealthHandlerTestDB(t)
	SetVersion("test-version")

	r := gin.New()
	r.GET("/ready", Ready)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, "ready", body["status"])
	require.Equal(t, "cornerstone-backend", body["service"])
	require.Equal(t, "test-version", body["version"])
}

func TestReadyReturns503WhenDatabasePingFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupHealthHandlerTestDB(t)
	SetVersion("test-version")

	// 主动关闭底层连接池, 让后续的 PingContext 必定失败。
	// pkgdb.DB() 仍能拿到 *gorm.DB(内部 db 变量未置 nil), 走的就是 Ping 路径而不是 panic。
	require.NoError(t, pkgdb.CloseDB())

	r := gin.New()
	r.GET("/ready", Ready)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusServiceUnavailable, w.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, "unready", body["status"])
	require.Contains(t, body["reason"], "database ping failed")
}
