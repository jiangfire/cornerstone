package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestIsAllowedMCPOrigin(t *testing.T) {
	t.Run("same host is allowed case-insensitively", func(t *testing.T) {
		require.True(t, isAllowedMCPOrigin("https://LOCALHOST:8080", "localhost:8080"))
	})

	t.Run("same host allows default http port normalization", func(t *testing.T) {
		require.True(t, isAllowedMCPOrigin("http://localhost", "localhost:80"))
	})

	t.Run("same host allows default https port normalization", func(t *testing.T) {
		require.True(t, isAllowedMCPOrigin("https://localhost", "localhost:443"))
	})

	t.Run("configured allowlist is honored", func(t *testing.T) {
		t.Setenv("MCP_ALLOWED_ORIGINS", "https://app.example.com, https://other.example.com")
		require.True(t, isAllowedMCPOrigin("https://app.example.com", "localhost:8080"))
	})

	t.Run("malformed origin is rejected", func(t *testing.T) {
		require.False(t, isAllowedMCPOrigin("://bad-origin", "localhost:8080"))
	})

	t.Run("unknown origin is rejected", func(t *testing.T) {
		t.Setenv("MCP_ALLOWED_ORIGINS", "https://app.example.com")
		require.False(t, isAllowedMCPOrigin("https://evil.example.com", "localhost:8080"))
	})
}

func TestMCPOriginGuardAllowsMissingOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(MCPOriginGuard())
	r.GET("/mcp", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNoContent, w.Code)
}

func TestMCPOriginGuardRejectsDisallowedOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	_ = os.Setenv("MCP_ALLOWED_ORIGINS", "")
	t.Cleanup(func() {
		_ = os.Unsetenv("MCP_ALLOWED_ORIGINS")
	})

	r := gin.New()
	r.Use(MCPOriginGuard())
	r.GET("/mcp", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "http://localhost/mcp", nil)
	req.Host = "localhost"
	req.Header.Set("Origin", "https://evil.example.com")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "Origin not allowed for MCP endpoint")
}
