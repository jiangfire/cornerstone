package handlers

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/backend/pkg/db"
)

// version 由 main 包通过 SetVersion 注入,避免 handlers 反向依赖 main 包。
var version = "dev"

// SetVersion 由 main 在路由注册前调用,把构建版本号注入到 /health /ready 响应。
func SetVersion(v string) {
	if v != "" {
		version = v
	}
}

// readinessProbeTimeout 是 /ready 内部 PingContext 的上限。
// 短于 docker HEALTHCHECK 的 timeout(3s),避免连接挂死时把整个探针拖到 docker 端超时。
const readinessProbeTimeout = 2 * time.Second

// Health 是 liveness 探针:进程在跑就返回 200。不查任何外部依赖。
// 用途:k8s livenessProbe / docker HEALTHCHECK 中的最低门槛。
func Health(c *gin.Context) {
	c.JSON(200, gin.H{
		"status":  "healthy",
		"service": "cornerstone-backend",
		"version": version,
		"time":    time.Now().Format(time.RFC3339),
	})
}

// Ready 是 readiness 探针:进程在跑 + 关键依赖可用(目前是 DB)才返回 200。
// DB ping 失败返回 503,部署工具/LB 据此把流量摘掉。
// 用途:k8s readinessProbe / docker compose healthcheck。
func Ready(c *gin.Context) {
	gormDB := db.DB()
	sqlDB, err := gormDB.DB()
	if err != nil {
		c.JSON(503, gin.H{
			"status":  "unready",
			"service": "cornerstone-backend",
			"version": version,
			"time":    time.Now().Format(time.RFC3339),
			"reason":  "database handle unavailable: " + err.Error(),
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), readinessProbeTimeout)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		c.JSON(503, gin.H{
			"status":  "unready",
			"service": "cornerstone-backend",
			"version": version,
			"time":    time.Now().Format(time.RFC3339),
			"reason":  "database ping failed: " + err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"status":  "ready",
		"service": "cornerstone-backend",
		"version": version,
		"time":    time.Now().Format(time.RFC3339),
	})
}
