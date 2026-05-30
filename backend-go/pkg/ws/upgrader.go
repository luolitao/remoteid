package ws

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type Upgrader struct {
	upgrader *websocket.Upgrader
	config   UpgraderConfig
}

type UpgraderConfig struct {
	// 读写缓冲区大小
	ReadBufferSize  int
	WriteBufferSize int

	// 超时设置
	HandshakeTimeout time.Duration
	ReadTimeout      time.Duration
	WriteTimeout     time.Duration
	PongTimeout      time.Duration
	PingPeriod       time.Duration

	// 压缩
	EnableCompression bool

	// 安全设置
	CheckOrigin    bool
	OriginPatterns []string
}

func DefaultConfig() UpgraderConfig {
	return UpgraderConfig{
		ReadBufferSize:    1024,
		WriteBufferSize:   1024,
		HandshakeTimeout:  10 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		PongTimeout:       60 * time.Second,
		PingPeriod:        54 * time.Second, // 90% of PongTimeout
		EnableCompression: true,
		CheckOrigin:       true,
		OriginPatterns:    []string{"*"},
	}
}

func NewUpgrader() *Upgrader {
	config := DefaultConfig()

	upgrader := &websocket.Upgrader{
		ReadBufferSize:    config.ReadBufferSize,
		WriteBufferSize:   config.WriteBufferSize,
		HandshakeTimeout:  config.HandshakeTimeout,
		EnableCompression: config.EnableCompression,
	}

	// 设置跨域检查
	if config.CheckOrigin {
		upgrader.CheckOrigin = func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				origin = r.Header.Get("Referer")
			}

			// 允许空 origin
			if origin == "" {
				return true
			}

			// 检查 origin 模式
			return isOriginAllowed(origin, config.OriginPatterns)
		}
	}

	return &Upgrader{
		upgrader: upgrader,
		config:   config,
	}
}

func (u *Upgrader) Upgrade(w http.ResponseWriter, r *http.Request, responseHeader http.Header) (*websocket.Conn, error) {
	conn, err := u.upgrader.Upgrade(w, r, responseHeader)
	if err != nil {
		return nil, err
	}

	// 设置读写超时
	conn.SetReadDeadline(time.Now().Add(u.config.ReadTimeout))
	conn.SetWriteDeadline(time.Now().Add(u.config.WriteTimeout))

	return conn, nil
}

func (u *Upgrader) WithConfig(config UpgraderConfig) *Upgrader {
	u.config = config
	u.upgrader.ReadBufferSize = config.ReadBufferSize
	u.upgrader.WriteBufferSize = config.WriteBufferSize
	u.upgrader.HandshakeTimeout = config.HandshakeTimeout
	u.upgrader.EnableCompression = config.EnableCompression

	if config.CheckOrigin {
		u.upgrader.CheckOrigin = func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				origin = r.Header.Get("Referer")
			}
			return origin == "" || isOriginAllowed(origin, config.OriginPatterns)
		}
	}

	return u
}

func (u *Upgrader) WithOrigins(origins []string) *Upgrader {
	u.config.OriginPatterns = origins
	return u.WithConfig(u.config)
}

func (u *Upgrader) WithCompression(enable bool) *Upgrader {
	u.config.EnableCompression = enable
	return u.WithConfig(u.config)
}

func (u *Upgrader) GetConfig() UpgraderConfig {
	return u.config
}

// isOriginAllowed 检查 origin 是否允许
func isOriginAllowed(origin string, patterns []string) bool {
	for _, pattern := range patterns {
		if pattern == "*" {
			return true
		}
		if pattern == origin {
			return true
		}
		// 简单通配符匹配
		if len(pattern) > 0 && pattern[0] == '*' && strings.HasSuffix(origin, pattern[1:]) {
			return true
		}
	}
	slog.Warn("跨域请求被拒绝", "origin", origin)
	return false
}

// GetRealIP 获取真实 IP
func GetRealIP(r *http.Request) string {
	// 检查 X-Forwarded-For
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}

	// 检查 X-Real-IP
	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		return xrip
	}

	// 获取远程地址
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
