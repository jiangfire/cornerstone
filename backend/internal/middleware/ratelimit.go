package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/backend/pkg/dto"
)

// rateLimitEntry 记录单个客户端的请求计数和窗口起始时间
type rateLimitEntry struct {
	count       int
	windowStart time.Time
}

// RateLimiter 基于固定窗口的简单限流器
type RateLimiter struct {
	mu      sync.RWMutex
	clients map[string]*rateLimitEntry
	limit   int
	window  time.Duration
}

// NewRateLimiter 创建限流器
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		clients: make(map[string]*rateLimitEntry),
		limit:   limit,
		window:  window,
	}
}

// Allow 检查客户端 key 是否允许通过
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	entry, exists := rl.clients[key]
	if !exists || now.Sub(entry.windowStart) >= rl.window {
		rl.clients[key] = &rateLimitEntry{count: 1, windowStart: now}
		return true
	}

	if entry.count >= rl.limit {
		return false
	}

	entry.count++
	return true
}

// cleanup 清理过期的限流记录（建议定期调用）
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for key, entry := range rl.clients {
		if now.Sub(entry.windowStart) >= rl.window {
			delete(rl.clients, key)
		}
	}
}

var (
	// 通用 API 限流：每 IP 每分钟 60 次
	apiRateLimiter = NewRateLimiter(60, time.Minute)
	// 认证端点限流：每 IP 每分钟 10 次（防止暴力破解）
	authRateLimiter = NewRateLimiter(10, time.Minute)
)

func init() {
	// 每 5 分钟清理一次过期记录，防止内存泄漏
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			apiRateLimiter.cleanup()
			authRateLimiter.cleanup()
		}
	}()
}

// RateLimit 通用 API 限流中间件
func RateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !apiRateLimiter.Allow(getClientKey(c)) {
			c.Header("Retry-After", "60")
			dto.Error(c, http.StatusTooManyRequests, "请求过于频繁，请稍后再试")
			c.Abort()
			return
		}
		c.Next()
	}
}

// AuthRateLimit 认证端点限流中间件（更严格）
func AuthRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !authRateLimiter.Allow(getClientKey(c)) {
			c.Header("Retry-After", "60")
			dto.Error(c, http.StatusTooManyRequests, "认证请求过于频繁，请稍后再试")
			c.Abort()
			return
		}
		c.Next()
	}
}

// getClientKey 生成限流 key，优先使用 IP，若配置了代理则取 X-Forwarded-For
func getClientKey(c *gin.Context) string {
	// 若部署在代理后，可优先使用 X-Forwarded-For
	xff := c.GetHeader("X-Forwarded-For")
	if xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	return c.ClientIP()
}
