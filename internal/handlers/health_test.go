package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/internal/config"
	pkgdb "github.com/jiangfire/cornerstone/pkg/db"
	"github.com/stretchr/testify/require"
)

func setupHealthHandlerTestDB(t *testing.T) {
	t.Helper()

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

	// Close the underlying connection pool so subsequent PingContext calls will definitely fail.
	// pkgdb.DB() still returns a *gorm.DB (internal db variable is not set to nil), so it takes the Ping path instead of panicking.
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
