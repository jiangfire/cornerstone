package middleware

import (
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/internal/authz"
	"github.com/jiangfire/cornerstone/internal/models"
	"github.com/jiangfire/cornerstone/pkg/db"
	"github.com/jiangfire/cornerstone/pkg/dto"
)

func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractToken(c)
		if token == "" {
			dto.Unauthorized(c, "missing API Key")
			c.Abort()
			return
		}

		masterToken := os.Getenv("MASTER_TOKEN")
		if masterToken != "" && token == masterToken {
			c.Set("token_id", "")
			c.Set("token_is_master", true)
			c.Set("token_scopes", "{}")
			c.Next()
			return
		}

		tokenRecord, err := validateToken(token)
		if err != nil {
			dto.Unauthorized(c, "invalid API Key")
			c.Abort()
			return
		}

		if tokenRecord.ExpiresAt != nil && tokenRecord.ExpiresAt.Before(time.Now()) {
			dto.Unauthorized(c, "API Key expired")
			c.Abort()
			return
		}

		c.Set("token_id", tokenRecord.ID)
		c.Set("token_is_master", false)
		c.Set("token_scopes", tokenRecord.Scopes)

		c.Next()
	}
}

func GetTokenID(c *gin.Context) string {
	if id, exists := c.Get("token_id"); exists {
		if s, ok := id.(string); ok {
			return s
		}
	}
	return ""
}

func IsMasterToken(c *gin.Context) bool {
	if v, exists := c.Get("token_is_master"); exists {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

func GetTokenScopes(c *gin.Context) map[string]bool {
	if v, exists := c.Get("token_scopes"); exists {
		if scopes, ok := v.(map[string]bool); ok {
			return scopes
		}
		if s, ok := v.(string); ok && s != "" {
			var result map[string]bool
			if err := json.Unmarshal([]byte(s), &result); err == nil {
				return result
			}
		}
	}
	return make(map[string]bool)
}

func RequireMaster() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !IsMasterToken(c) {
			dto.Forbidden(c, "master token required for this operation")
			c.Abort()
			return
		}
		c.Next()
	}
}

func extractToken(c *gin.Context) string {
	if key := c.GetHeader("X-API-Key"); key != "" {
		return strings.TrimSpace(key)
	}
	auth := c.GetHeader("Authorization")
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return strings.TrimSpace(parts[1])
	}
	return ""
}

func validateToken(token string) (*models.Token, error) {
	return authz.FindTokenByValue(db.DB(), token)
}
