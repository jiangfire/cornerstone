package middleware

import (
	"crypto/subtle"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/backend/pkg/dto"
)

// IntegrationAuthConfig 入站集成 token 配置；由 main.go 在路由注册阶段从 cfg.Integrations 注入，
// 避免中间件在请求路径上反复读取 os.Getenv，也保证 config 与 middleware 走同一份变量来源。
type IntegrationAuthConfig struct {
	InboundTokens string // INTEGRATION_TOKENS：按来源系统区分，格式 sys=tok,sys2=tok2
	SharedToken   string // INTEGRATION_SHARED_TOKEN：跨系统共享回写 token
}

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

// IntegrationTokenAuth 系统集成 token 认证。
// cfg 在启动阶段一次性解析，运行时不再访问环境变量。
func IntegrationTokenAuth(cfg IntegrationAuthConfig) gin.HandlerFunc {
	allowed := parseIntegrationTokens(cfg.InboundTokens)
	sharedToken := strings.TrimSpace(cfg.SharedToken)

	return func(c *gin.Context) {
		sourceSystem := strings.TrimSpace(c.GetHeader("X-Source-System"))
		if sourceSystem == "" {
			dto.Unauthorized(c, "缺少 X-Source-System")
			c.Abort()
			return
		}

		authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			dto.Unauthorized(c, "认证令牌格式错误")
			c.Abort()
			return
		}

		token := strings.TrimSpace(parts[1])
		if token == "" {
			dto.Unauthorized(c, "缺少认证令牌")
			c.Abort()
			return
		}

		if secureEquals(token, sharedToken) {
			c.Set("integration_source", sourceSystem)
			c.Set("integration_token_scope", "shared")
			c.Next()
			return
		}

		expected, ok := allowed[sourceSystem]
		if !ok || !secureEquals(token, expected) {
			dto.Unauthorized(c, "无效的集成令牌")
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
