package middleware

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	requestIDKey = "request_id"
	traceIDKey   = "trace_id"
)

// RequestID 为每个请求注入 request_id/trace_id，便于链路追踪
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := strings.TrimSpace(c.GetHeader("X-Request-ID"))
		if requestID == "" {
			requestID = uuid.NewString()
		}

		traceID := parseTraceID(c.GetHeader("traceparent"))
		if traceID == "" {
			traceID = strings.TrimSpace(c.GetHeader("X-Trace-ID"))
		}
		if traceID == "" {
			traceID = requestID
		}

		c.Set(requestIDKey, requestID)
		c.Set(traceIDKey, traceID)
		c.Writer.Header().Set("X-Request-ID", requestID)
		c.Writer.Header().Set("X-Trace-ID", traceID)

		c.Next()
	}
}

// RequestLogger 记录请求日志的中间件
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 开始时间
		start := time.Now()

		// 处理请求
		c.Next()

		// 计算耗时
		duration := time.Since(start)
		requestID := GetRequestID(c)
		traceID := GetTraceID(c)

		// 记录日志
		zap.L().Info("HTTP请求",
			zap.String("request_id", requestID),
			zap.String("trace_id", traceID),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.String("duration", duration.String()),
			zap.String("client_ip", c.ClientIP()),
			zap.String("user_agent", c.Request.UserAgent()),
		)
	}
}

// GetRequestID 从上下文获取 request_id
func GetRequestID(c *gin.Context) string {
	if id, exists := c.Get(requestIDKey); exists {
		if s, ok := id.(string); ok {
			return s
		}
	}
	return ""
}

// GetTraceID 从上下文获取 trace_id
func GetTraceID(c *gin.Context) string {
	if id, exists := c.Get(traceIDKey); exists {
		if s, ok := id.(string); ok {
			return s
		}
	}
	return ""
}

func parseTraceID(traceparent string) string {
	parts := strings.Split(traceparent, "-")
	if len(parts) >= 4 && len(parts[1]) == 32 {
		return parts[1]
	}
	return ""
}

// CORS 跨域处理中间件
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")
		c.Header("Access-Control-Allow-Credentials", "false")

		// 处理预检请求
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
