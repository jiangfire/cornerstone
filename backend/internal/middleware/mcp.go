package middleware

import (
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// MCPOriginGuard 校验 HTTP MCP 的 Origin，降低 DNS rebinding 风险。
// 若请求未携带 Origin，则默认放行，兼容非浏览器 MCP 客户端。
func MCPOriginGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := strings.TrimSpace(c.GetHeader("Origin"))
		if origin == "" {
			c.Next()
			return
		}

		if isAllowedMCPOrigin(origin, c.Request.Host) {
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"jsonrpc": "2.0",
			"error": gin.H{
				"code":    -32003,
				"message": "Origin not allowed for MCP endpoint",
			},
		})
	}
}

func isAllowedMCPOrigin(origin, requestHost string) bool {
	originURL, err := url.Parse(origin)
	if err != nil || originURL.Host == "" {
		return false
	}

	if sameHost(originURL.Host, requestHost, originURL.Scheme) {
		return true
	}

	for _, item := range strings.Split(os.Getenv("MCP_ALLOWED_ORIGINS"), ",") {
		allowed := strings.TrimSpace(item)
		if allowed == "" {
			continue
		}
		if strings.EqualFold(allowed, origin) {
			return true
		}
	}

	return false
}

func sameHost(originHost, requestHost, scheme string) bool {
	originHost = strings.TrimSpace(strings.ToLower(originHost))
	requestHost = strings.TrimSpace(strings.ToLower(requestHost))
	scheme = strings.TrimSpace(strings.ToLower(scheme))
	if originHost == "" || requestHost == "" {
		return false
	}

	originURL := &url.URL{Scheme: scheme, Host: originHost}
	requestURL := &url.URL{Scheme: scheme, Host: requestHost}
	if originURL.Hostname() == "" || requestURL.Hostname() == "" {
		return false
	}
	if originURL.Hostname() != requestURL.Hostname() {
		return false
	}

	originPort := normalizePort(originURL.Port(), scheme)
	requestPort := normalizePort(requestURL.Port(), scheme)
	return originPort == requestPort
}

func normalizePort(port, scheme string) string {
	if strings.TrimSpace(port) != "" {
		return port
	}
	switch scheme {
	case "http":
		return "80"
	case "https":
		return "443"
	default:
		return ""
	}
}
