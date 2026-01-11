package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/backend/internal/types"
	"github.com/jiangfire/cornerstone/backend/pkg/utils"
	"go.uber.org/zap"
)

// Auth JWT认证中间件
func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取Authorization头
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			types.Unauthorized(c, "缺少认证令牌")
			c.Abort()
			return
		}

		// 检查Bearer格式
		parts := strings.SplitN(authHeader, " ", 2)
		if !(len(parts) == 2 && parts[0] == "Bearer") {
			types.Unauthorized(c, "认证令牌格式错误")
			c.Abort()
			return
		}

		tokenString := parts[1]

		// 验证token
		claims, err := utils.ValidateJWT(tokenString)
		if err != nil {
			zap.L().Error("JWT验证失败", zap.Error(err))
			types.Unauthorized(c, "无效的认证令牌")
			c.Abort()
			return
		}

		// 检查token是否在黑名单中
		if utils.IsTokenBlacklisted(tokenString) {
			types.Unauthorized(c, "令牌已失效")
			c.Abort()
			return
		}

		// 将用户信息存储到上下文
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("user_role", claims.Role)

		c.Next()
	}
}

// GetUserID 从上下文中获取用户ID
func GetUserID(c *gin.Context) string {
	if userID, exists := c.Get("user_id"); exists {
		if id, ok := userID.(string); ok {
			return id
		}
	}
	return ""
}

// GetUserRole 从上下文中获取用户角色
func GetUserRole(c *gin.Context) string {
	if role, exists := c.Get("user_role"); exists {
		if r, ok := role.(string); ok {
			return r
		}
	}
	return ""
}
