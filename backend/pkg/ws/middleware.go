package ws

import (
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

// ConnectionLimiter 连接限制中间件
func ConnectionLimiter(maxConns int, manager *Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		currentConns := manager.GetConnectionCount()
		if currentConns >= maxConns {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":   "Too many WebSocket connections",
				"max":     maxConns,
				"current": currentConns,
			})
			return
		}
		c.Next()
	}
}

// RateLimiter 请求速率限制中间件
func RateLimiter(requests int, window time.Duration) gin.HandlerFunc {
	limit := requests
	windowSec := int(window.Seconds())
	counts := make(map[string]int)
	timestamps := make(map[string]int64)

	return func(c *gin.Context) {
		ip := GetRealIP(c.Request)
		now := time.Now().Unix()

		// 清理旧数据
		if oldTime, ok := timestamps[ip]; ok && now-oldTime > int64(windowSec) {
			counts[ip] = 0
			timestamps[ip] = now
		}

		// 检查速率
		counts[ip]++
		if counts[ip] > limit {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":  "Too many requests",
				"limit":  limit,
				"window": windowSec,
			})
			return
		}

		// 设置清理定时器
		if _, ok := timestamps[ip]; !ok {
			timestamps[ip] = now
		}

		c.Next()
	}
}

// AuthMiddleware 认证中间件
func AuthMiddleware(allowedOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin == "" {
			origin = c.Request.Header.Get("Referer")
		}

		if !isOriginAllowed(origin, allowedOrigins) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":  "Forbidden origin",
				"origin": origin,
			})
			return
		}

		// 可选：检查 token
		token := c.Query("token")
		if token != "" && !validateToken(token) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid token",
			})
			return
		}

		c.Next()
	}
}

func validateToken(token string) bool {
	// 简单 token 验证，支持环境变量 WS_TOKEN 或默认值
	expected := os.Getenv("WS_TOKEN")
	if expected == "" {
		expected = "remoteid-ws-token" // 非敏感默认值，生产环境应设置环境变量
	}
	return token == expected
}
