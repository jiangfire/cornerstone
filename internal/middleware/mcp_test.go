package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestMCPOriginGuard_NoOrigin(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/mcp", nil)

	called := false
	MCPOriginGuard()(c)
	_ = called

	assert.False(t, c.IsAborted())
}

func TestMCPOriginGuard_SameOrigin(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/mcp", nil)
	c.Request.Header.Set("Origin", "http://localhost:8080")
	c.Request.Host = "localhost:8080"

	MCPOriginGuard()(c)

	assert.False(t, c.IsAborted())
}

func TestMCPOriginGuard_DifferentOrigin(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/mcp", nil)
	c.Request.Header.Set("Origin", "http://evil.com")
	c.Request.Host = "localhost:8080"

	MCPOriginGuard()(c)

	assert.True(t, c.IsAborted())
}

func TestMCPOriginGuard_DifferentPort(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/mcp", nil)
	c.Request.Header.Set("Origin", "http://localhost:9090")
	c.Request.Host = "localhost:8080"

	MCPOriginGuard()(c)

	assert.True(t, c.IsAborted())
}

func TestMCPOriginGuard_403Response(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/mcp", nil)
	c.Request.Header.Set("Origin", "http://evil.com")
	c.Request.Host = "localhost:8080"

	MCPOriginGuard()(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "Origin not allowed")
}

func TestMCPOriginGuard_AllowedOriginsEnv(t *testing.T) {
	os.Setenv("MCP_ALLOWED_ORIGINS", "http://allowed.com,http://other.com")
	defer os.Unsetenv("MCP_ALLOWED_ORIGINS")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/mcp", nil)
	c.Request.Header.Set("Origin", "http://allowed.com")
	c.Request.Host = "localhost:8080"

	MCPOriginGuard()(c)

	assert.False(t, c.IsAborted())
}

func TestMCPOriginGuard_AllowedOriginsEnvNotMatched(t *testing.T) {
	os.Setenv("MCP_ALLOWED_ORIGINS", "http://allowed.com")
	defer os.Unsetenv("MCP_ALLOWED_ORIGINS")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/mcp", nil)
	c.Request.Header.Set("Origin", "http://notallowed.com")
	c.Request.Host = "localhost:8080"

	MCPOriginGuard()(c)

	assert.True(t, c.IsAborted())
}

func TestMCPOriginGuard_AllowedOriginsCaseInsensitive(t *testing.T) {
	os.Setenv("MCP_ALLOWED_ORIGINS", "http://Allowed.Com")
	defer os.Unsetenv("MCP_ALLOWED_ORIGINS")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/mcp", nil)
	c.Request.Header.Set("Origin", "http://allowed.com")
	c.Request.Host = "localhost:8080"

	MCPOriginGuard()(c)

	assert.False(t, c.IsAborted())
}

func TestIsAllowedMCPOrigin_MalformedURL(t *testing.T) {
	assert.False(t, isAllowedMCPOrigin("://missing-scheme", "localhost:8080"))
}

func TestIsAllowedMCPOrigin_EmptyHost(t *testing.T) {
	assert.False(t, isAllowedMCPOrigin("http://", "localhost:8080"))
}

func TestIsAllowedMCPOrigin_SameHost(t *testing.T) {
	assert.True(t, isAllowedMCPOrigin("http://localhost:8080", "localhost:8080"))
}

func TestIsAllowedMCPOrigin_DifferentHost(t *testing.T) {
	assert.False(t, isAllowedMCPOrigin("http://evil.com", "localhost:8080"))
}

func TestSameHost_DifferentHostnames(t *testing.T) {
	assert.False(t, sameHost("evil.com", "localhost:8080", "http"))
}

func TestSameHost_SameHostDifferentPort(t *testing.T) {
	assert.False(t, sameHost("localhost:9090", "localhost:8080", "http"))
}

func TestSameHost_SameHostSamePort(t *testing.T) {
	assert.True(t, sameHost("localhost:8080", "localhost:8080", "http"))
}

func TestSameHost_EmptyStrings(t *testing.T) {
	assert.False(t, sameHost("", "localhost:8080", "http"))
	assert.False(t, sameHost("localhost:8080", "", "http"))
}

func TestSameHost_HTTPSDefaultPort(t *testing.T) {
	assert.True(t, sameHost("example.com", "example.com:443", "https"))
	assert.False(t, sameHost("example.com:80", "example.com:443", "https"))
}

func TestSameHost_HTTPDefaultPort(t *testing.T) {
	assert.True(t, sameHost("example.com", "example.com:80", "http"))
	assert.True(t, sameHost("example.com:80", "example.com", "http"))
}

func TestNormalizePort_HTTP(t *testing.T) {
	assert.Equal(t, "80", normalizePort("", "http"))
}

func TestNormalizePort_HTTPS(t *testing.T) {
	assert.Equal(t, "443", normalizePort("", "https"))
}

func TestNormalizePort_CustomPort(t *testing.T) {
	assert.Equal(t, "8080", normalizePort("8080", "http"))
}

func TestNormalizePort_Whitespace(t *testing.T) {
	assert.Equal(t, "80", normalizePort("  ", "http"))
}

func TestNormalizePort_UnknownScheme(t *testing.T) {
	assert.Equal(t, "", normalizePort("", "ftp"))
}

func TestMCPOriginGuard_SameHostHTTPSScheme(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/mcp", nil)
	c.Request.Header.Set("Origin", "https://example.com")
	c.Request.Host = "example.com:443"

	MCPOriginGuard()(c)

	assert.False(t, c.IsAborted())
}

func TestMCPOriginGuard_EmptyAllowedOrigins(t *testing.T) {
	os.Setenv("MCP_ALLOWED_ORIGINS", "")
	defer os.Unsetenv("MCP_ALLOWED_ORIGINS")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/mcp", nil)
	c.Request.Header.Set("Origin", "http://evil.com")
	c.Request.Host = "localhost:8080"

	MCPOriginGuard()(c)

	assert.True(t, c.IsAborted())
}

func TestMCPOriginGuard_WhitespaceOrigin(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/mcp", nil)
	c.Request.Header.Set("Origin", "   ")

	MCPOriginGuard()(c)

	assert.False(t, c.IsAborted())
}
