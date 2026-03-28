package middleware

import (
	"crypto/subtle"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/backend/internal/types"
)

func parseIntegrationTokens(raw string) map[string]string {
	result := map[string]string{}
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}

		parts := strings.SplitN(item, "=", 2)
		if len(parts) != 2 {
			continue
		}

		source := strings.TrimSpace(parts[0])
		token := strings.TrimSpace(parts[1])
		if source == "" || token == "" {
			continue
		}

		result[source] = token
	}
	return result
}

func secureEquals(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// IntegrationTokenAuth 系统集成 token 认证
func IntegrationTokenAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		sourceSystem := strings.TrimSpace(c.GetHeader("X-Source-System"))
		if sourceSystem == "" {
			types.Unauthorized(c, "缺少 X-Source-System")
			c.Abort()
			return
		}

		authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			types.Unauthorized(c, "认证令牌格式错误")
			c.Abort()
			return
		}

		token := strings.TrimSpace(parts[1])
		if token == "" {
			types.Unauthorized(c, "缺少认证令牌")
			c.Abort()
			return
		}

		sharedToken := strings.TrimSpace(os.Getenv("INTEGRATION_SHARED_TOKEN"))
		if secureEquals(token, sharedToken) {
			c.Set("integration_source", sourceSystem)
			c.Set("integration_token_scope", "shared")
			c.Next()
			return
		}

		allowed := parseIntegrationTokens(os.Getenv("INTEGRATION_TOKENS"))
		expected, ok := allowed[sourceSystem]
		if !ok || !secureEquals(token, expected) {
			types.Unauthorized(c, "无效的集成令牌")
			c.Abort()
			return
		}

		c.Set("integration_source", sourceSystem)
		c.Set("integration_token_scope", "source")
		c.Next()
	}
}

// GetIntegrationSource 获取调用方系统名
func GetIntegrationSource(c *gin.Context) string {
	if source, exists := c.Get("integration_source"); exists {
		if s, ok := source.(string); ok {
			return s
		}
	}
	return ""
}
