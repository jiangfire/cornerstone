package handlers

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/internal/middleware"
	"github.com/jiangfire/cornerstone/internal/models"
	"github.com/jiangfire/cornerstone/internal/testutil"
	pkgdb "github.com/jiangfire/cornerstone/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChatWithAI_NotConfiguredReturns503(t *testing.T) {
	db := testutil.SetupTestDB(t)
	master := &models.Token{Name: "master", IsMaster: true, Scopes: "{}"}
	require.NoError(t, db.Create(master).Error)
	pkgdb.SetDB(db)

	previousAgent := aiAgent
	aiAgent = nil
	t.Cleanup(func() { aiAgent = previousAgent })

	router := gin.New()
	router.Use(middleware.Auth())
	router.POST("/api/v1/ai/chat", ChatWithAI)

	rec := testutil.DoRequest(t, router, "POST", "/api/v1/ai/chat", master.Token, map[string]interface{}{
		"message": "List databases",
	})

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	resp := testutil.DecodeJSONResponseRaw(t, rec)
	assert.Equal(t, float64(http.StatusServiceUnavailable), resp["code"])
	assert.Contains(t, resp["message"], "LLM_API_KEY")
}
