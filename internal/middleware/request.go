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

// RequestID injects request_id/trace_id for each request for traceability
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

// RequestLogger is a middleware that logs requests
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start time
		start := time.Now()

		// Process request
		c.Next()

		// Calculate duration
		duration := time.Since(start)
		requestID := GetRequestID(c)
		traceID := GetTraceID(c)

		// Log the request
		zap.L().Info("HTTP request",
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

// GetRequestID gets request_id from context
func GetRequestID(c *gin.Context) string {
	if id, exists := c.Get(requestIDKey); exists {
		if s, ok := id.(string); ok {
			return s
		}
	}
	return ""
}

// GetTraceID gets trace_id from context
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

// CORS is a cross-origin middleware
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")
		c.Header("Access-Control-Allow-Credentials", "false")

		// Handle preflight request
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
