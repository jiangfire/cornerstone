package handlers

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/pkg/db"
)

// version is injected by the main package via SetVersion to avoid a circular dependency from handlers back to main.
var version = "dev"

// SetVersion is called by main before route registration to inject the build version into /health and /ready responses.
func SetVersion(v string) {
	if v != "" {
		version = v
	}
}

// readinessProbeTimeout is the upper bound for PingContext inside /ready.
// Shorter than the docker HEALTHCHECK timeout (3s) to prevent a hung connection from dragging the probe past the docker-side timeout.
const readinessProbeTimeout = 2 * time.Second

// Health is the liveness probe: returns 200 as long as the process is running. Does not check any external dependencies.
// Used for: k8s livenessProbe / docker HEALTHCHECK minimum threshold.
//
// @Summary      Health check
// @Description  Liveness probe. Returns 200 if the process is running.
//
//	Does not check external dependencies. Use /ready for a full readiness check.
//	No authentication required.
//
// @Tags         health
// @Produce      json
// @Success      200  {object}  object  "{"status":"healthy","service":"cornerstone-backend","version":"...","time":"..."}"
// @Router       /health [get]
func Health(c *gin.Context) {
	c.JSON(200, gin.H{
		"status":  "healthy",
		"service": "cornerstone-backend",
		"version": version,
		"time":    time.Now().Format(time.RFC3339),
	})
}

// Ready is the readiness probe: returns 200 only when the process is running and critical dependencies (currently DB) are available.
// DB ping failure returns 503 so deployment tools/LB can drain traffic.
// Used for: k8s readinessProbe / docker compose healthcheck.
//
// @Summary      Readiness check
// @Description  Readiness probe. Returns 200 if the process is running and the database is reachable.
//
//	Returns 503 if the database is unreachable. No authentication required.
//	Use this endpoint for load balancer health checks and orchestration readiness probes.
//
// @Tags         health
// @Produce      json
// @Success      200  {object}  object  "{"status":"ready","service":"cornerstone-backend","version":"...","time":"..."}"
// @Failure      503  {object}  object  "{"status":"unready","reason":"..."}"
// @Router       /ready [get]
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
