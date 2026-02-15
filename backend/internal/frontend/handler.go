package frontend

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers the frontend static file serving routes.
// If no frontend dist is embedded (empty dist directory), it returns silently
// so the server can run as an API-only backend during development.
func RegisterRoutes(r *gin.Engine) {
	subFS, err := fs.Sub(distFS, "dist")
	if err != nil {
		return
	}

	// Check if index.html exists — if not, no frontend is embedded
	_, err = subFS.Open("index.html")
	if err != nil {
		return
	}

	fileServer := http.FileServer(http.FS(subFS))

	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// API routes that don't match → JSON 404
		if strings.HasPrefix(path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "not found",
				"message": "The requested API endpoint does not exist",
			})
			return
		}

		// Try to serve a static file from the embedded filesystem
		f, err := subFS.Open(strings.TrimPrefix(path, "/"))
		if err == nil {
			// 关闭文件，忽略错误（文件已经成功打开）
			_ = f.Close()

			// Cache hashed assets aggressively, no-cache for everything else
			if strings.HasPrefix(path, "/assets/") {
				c.Header("Cache-Control", "public, max-age=31536000, immutable")
			}

			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}

		// SPA fallback: serve index.html for all other routes
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Request.URL.Path = "/"
		fileServer.ServeHTTP(c.Writer, c.Request)
	})
}
