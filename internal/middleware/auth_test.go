package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jiangfire/cornerstone/internal/config"
	"github.com/jiangfire/cornerstone/internal/models"
	pkgdb "github.com/jiangfire/cornerstone/pkg/db"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupAuthDB(t *testing.T) (*gin.Engine, *models.Token, *models.Token) {
	t.Helper()

	dbType := os.Getenv("DB_TYPE")
	databaseURL := os.Getenv("DATABASE_URL")
	if dbType == "" {
		dbType = "sqlite"
		databaseURL = ":memory:"
	}

	err := pkgdb.InitDB(config.DatabaseConfig{Type: dbType, URL: databaseURL})
	require.NoError(t, err)

	d := pkgdb.DB()
	err = d.AutoMigrate(&models.Token{}, &models.Database{}, &models.Table{}, &models.Field{}, &models.Record{}, &models.RecordFieldIndex{}, &models.File{})
	require.NoError(t, err)

	master := &models.Token{Name: "master", IsMaster: true, Scopes: "{}", CreatedAt: time.Now()}
	require.NoError(t, d.Create(master).Error)

	worker := &models.Token{Name: "worker", IsMaster: false, Scopes: `{"databases":{},"tables":{}}`, CreatedAt: time.Now()}
	require.NoError(t, d.Create(worker).Error)

	pkgdb.SetDB(d)

	// Cleanup function: hard-delete all test data
	t.Cleanup(func() {
		d.Unscoped().Where("1 = 1").Delete(&models.File{})
		d.Unscoped().Where("1 = 1").Delete(&models.RecordFieldIndex{})
		d.Unscoped().Where("1 = 1").Delete(&models.Record{})
		d.Unscoped().Where("1 = 1").Delete(&models.Field{})
		d.Unscoped().Where("1 = 1").Delete(&models.Table{})
		d.Unscoped().Where("1 = 1").Delete(&models.Database{})
		d.Unscoped().Where("1 = 1").Delete(&models.Token{})
		_ = pkgdb.CloseDB()
	})

	r := gin.New()
	return r, master, worker
}

func testHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"token_id":  GetTokenID(c),
		"is_master": IsMasterToken(c),
	})
}

func TestAuth_MissingToken(t *testing.T) {
	r, _, _ := setupAuthDB(t)
	r.Use(Auth())
	r.GET("/", testHandler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuth_BearerToken(t *testing.T) {
	r, master, _ := setupAuthDB(t)
	r.Use(Auth())
	r.GET("/", testHandler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+master.Token)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuth_XAPIKey(t *testing.T) {
	r, _, worker := setupAuthDB(t)
	r.Use(Auth())
	r.GET("/", testHandler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-API-Key", worker.Token)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuth_InvalidToken(t *testing.T) {
	r, _, _ := setupAuthDB(t)
	r.Use(Auth())
	r.GET("/", testHandler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer invalid_token_value")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuth_ExpiredToken(t *testing.T) {
	r, _, _ := setupAuthDB(t)

	past := time.Now().Add(-24 * time.Hour)
	expired := &models.Token{
		Name:      "expired",
		IsMaster:  false,
		Scopes:    "{}",
		ExpiresAt: &past,
	}
	require.NoError(t, pkgdb.DB().Create(expired).Error)

	r.Use(Auth())
	r.GET("/", testHandler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+expired.Token)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuth_MasterTokenEnv(t *testing.T) {
	r, _, _ := setupAuthDB(t)
	masterVal := "env_master_secret_12345"
	os.Setenv("MASTER_TOKEN", masterVal)
	defer os.Unsetenv("MASTER_TOKEN")

	r.Use(Auth())
	r.GET("/", testHandler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+masterVal)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuth_MasterTokenEnv_XAPIKey(t *testing.T) {
	r, _, _ := setupAuthDB(t)
	masterVal := "env_master_xapi"
	os.Setenv("MASTER_TOKEN", masterVal)
	defer os.Unsetenv("MASTER_TOKEN")

	r.Use(Auth())
	r.GET("/", testHandler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-API-Key", masterVal)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetTokenID(t *testing.T) {
	r, master, _ := setupAuthDB(t)
	r.Use(Auth())
	r.GET("/", func(c *gin.Context) {
		id := GetTokenID(c)
		c.JSON(http.StatusOK, gin.H{"token_id": id})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+master.Token)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestIsMasterToken(t *testing.T) {
	r, master, _ := setupAuthDB(t)
	r.Use(Auth())
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"is_master": IsMasterToken(c)})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+master.Token)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestIsMasterToken_NonMaster(t *testing.T) {
	r, _, worker := setupAuthDB(t)
	r.Use(Auth())
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"is_master": IsMasterToken(c)})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+worker.Token)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetTokenScopes(t *testing.T) {
	r, _, worker := setupAuthDB(t)
	r.Use(Auth())
	r.GET("/", func(c *gin.Context) {
		scopes := GetTokenScopes(c)
		c.JSON(http.StatusOK, gin.H{"scopes": scopes})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+worker.Token)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetTokenScopes_Empty(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	scopes := GetTokenScopes(c)
	assert.NotNil(t, scopes)
	assert.Len(t, scopes, 0)
}

func TestRequireMaster_BlocksNonMaster(t *testing.T) {
	r, _, worker := setupAuthDB(t)
	r.Use(Auth())
	r.Use(RequireMaster())
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+worker.Token)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRequireMaster_PassesMaster(t *testing.T) {
	r, _, _ := setupAuthDB(t)
	masterVal := "env_master_pass"
	os.Setenv("MASTER_TOKEN", masterVal)
	defer os.Unsetenv("MASTER_TOKEN")

	r.Use(Auth())
	r.Use(RequireMaster())
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+masterVal)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireMaster_MasterEnv(t *testing.T) {
	r, _, _ := setupAuthDB(t)
	masterVal := "env_master_rm"
	os.Setenv("MASTER_TOKEN", masterVal)
	defer os.Unsetenv("MASTER_TOKEN")

	r.Use(Auth())
	r.Use(RequireMaster())
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+masterVal)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestExtractToken_XAPIKey(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Request.Header.Set("X-API-Key", "my-key")

	token := extractToken(c)
	assert.Equal(t, "my-key", token)
}

func TestExtractToken_Bearer(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Request.Header.Set("Authorization", "Bearer my-bearer")

	token := extractToken(c)
	assert.Equal(t, "my-bearer", token)
}

func TestExtractToken_BearerCaseInsensitive(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Request.Header.Set("Authorization", "bearer my-bearer")

	token := extractToken(c)
	assert.Equal(t, "my-bearer", token)
}

func TestExtractToken_XAPIKeyPriority(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Request.Header.Set("X-API-Key", "api-key")
	c.Request.Header.Set("Authorization", "Bearer bearer-token")

	token := extractToken(c)
	assert.Equal(t, "api-key", token)
}

func TestExtractToken_NoHeaders(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	token := extractToken(c)
	assert.Equal(t, "", token)
}

func TestExtractToken_MalformedAuth(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Request.Header.Set("Authorization", "Basic abc123")

	token := extractToken(c)
	assert.Equal(t, "", token)
}

func TestAuth_XAPIKeyTakesPrecedence(t *testing.T) {
	r, master, _ := setupAuthDB(t)
	r.Use(Auth())
	r.GET("/", func(c *gin.Context) {
		id := GetTokenID(c)
		masterFlag := IsMasterToken(c)
		c.JSON(http.StatusOK, gin.H{"token_id": id, "is_master": masterFlag})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-API-Key", master.Token)
	req.Header.Set("Authorization", "Bearer some_other_token")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuth_MultipleRequestsIsolation(t *testing.T) {
	r, master, worker := setupAuthDB(t)
	r.Use(Auth())
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"token_id":  GetTokenID(c),
			"is_master": IsMasterToken(c),
		})
	})

	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest("GET", "/", nil)
	req1.Header.Set("Authorization", "Bearer "+master.Token)
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("GET", "/", nil)
	req2.Header.Set("Authorization", "Bearer "+worker.Token)
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
}

func TestAuth_ValidNotExpiredToken(t *testing.T) {
	r, _, _ := setupAuthDB(t)
	future := time.Now().Add(24 * time.Hour)
	valid := &models.Token{
		Name:      "valid_future",
		IsMaster:  false,
		Scopes:    "{}",
		ExpiresAt: &future,
	}
	require.NoError(t, pkgdb.DB().Create(valid).Error)

	r.Use(Auth())
	r.GET("/", testHandler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+valid.Token)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestValidateToken_NotFound(t *testing.T) {
	_, _, _ = setupAuthDB(t)

	_, err := validateToken("nonexistent_token_value")
	assert.Error(t, err)
}

func TestAllAuthErrorResponses(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() string
		wantMsg string
	}{
		{
			name:    "missing token",
			setup:   func() string { return "" },
			wantMsg: "missing API Key",
		},
		{
			name:    "invalid token",
			setup:   func() string { return "Bearer totally_invalid_token" },
			wantMsg: "invalid API Key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, _, _ := setupAuthDB(t)
			r.Use(Auth())
			r.GET("/", testHandler)

			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)
			tokenVal := tt.setup()
			if tokenVal != "" {
				req.Header.Set("Authorization", tokenVal)
			}
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)
			assert.Contains(t, w.Body.String(), tt.wantMsg)
		})
	}
}

func TestGetTokenID_NoContext(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	assert.Equal(t, "", GetTokenID(c))
}

func TestIsMasterToken_NoContext(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	assert.False(t, IsMasterToken(c))
}

func TestGetTokenScopes_WithString(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Set("token_scopes", `{"read":true}`)
	scopes := GetTokenScopes(c)
	assert.NotNil(t, scopes)
}

func TestAuth_MasterEnvNotSet(t *testing.T) {
	os.Unsetenv("MASTER_TOKEN")
	r, _, _ := setupAuthDB(t)
	r.Use(Auth())
	r.GET("/", testHandler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer env_master_rm")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
