package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func newIntegrationTestRouter(cfg IntegrationAuthConfig) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(IntegrationTokenAuth(cfg))
	r.POST("/integrations/events", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"source": GetIntegrationSource(c),
		})
	})
	return r
}

func doAuthRequest(r *gin.Engine, source, authHeader string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/integrations/events", nil)
	if source != "" {
		req.Header.Set("X-Source-System", source)
	}
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	r.ServeHTTP(w, req)
	return w
}

func TestIntegrationTokenAuth_PerSourceToken(t *testing.T) {
	cfg := IntegrationAuthConfig{InboundTokens: "fuckcmdb=tokA, other=tokB"}
	r := newIntegrationTestRouter(cfg)

	w := doAuthRequest(r, "fuckcmdb", "Bearer tokA")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid per-source token, got %d body=%s", w.Code, w.Body.String())
	}

	w = doAuthRequest(r, "fuckcmdb", "Bearer wrong")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong token, got %d", w.Code)
	}

	w = doAuthRequest(r, "unknown-system", "Bearer tokA")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unknown source, got %d", w.Code)
	}
}

func TestIntegrationTokenAuth_SharedToken(t *testing.T) {
	cfg := IntegrationAuthConfig{SharedToken: "global-shared"}
	r := newIntegrationTestRouter(cfg)

	w := doAuthRequest(r, "anything", "Bearer global-shared")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for shared token, got %d body=%s", w.Code, w.Body.String())
	}

	w = doAuthRequest(r, "anything", "Bearer not-shared")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong shared token, got %d", w.Code)
	}
}

func TestIntegrationTokenAuth_HeaderValidation(t *testing.T) {
	cfg := IntegrationAuthConfig{InboundTokens: "x=y"}
	r := newIntegrationTestRouter(cfg)

	w := doAuthRequest(r, "", "Bearer y")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when X-Source-System missing, got %d", w.Code)
	}

	w = doAuthRequest(r, "x", "y")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when scheme missing, got %d", w.Code)
	}

	w = doAuthRequest(r, "x", "Bearer ")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when token empty, got %d", w.Code)
	}
}

func TestIntegrationTokenAuth_EmptyConfigAlwaysRejects(t *testing.T) {
	r := newIntegrationTestRouter(IntegrationAuthConfig{})

	w := doAuthRequest(r, "fuckcmdb", "Bearer anything")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with empty config, got %d", w.Code)
	}
}
