// internal/api/server.go
package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"remoteid-monitor/internal/config"
	"remoteid-monitor/internal/drone"
	"remoteid-monitor/pkg/ws"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type Server struct {
	engine    *gin.Engine
	httpSrv   *http.Server
	processor *drone.Processor
	wsManager *ws.Manager
	sniffer   *drone.Sniffer
	config    *config.Config
	ctx       context.Context
	cancel    context.CancelFunc
	startTime time.Time
}

// WebSocket Upgrader
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// 💡 优化 1：修复形同虚设的校验。
		// 如果开发环境允许所有跨域，直接 return true 并加注释；
		// 如果需要严格校验，请删除最后的 return true。
		origin := r.Header.Get("Origin")

		// 简单校验：如果是本地或局域网 IP，直接放行
		if origin == "" ||
			origin == "http://localhost:8080" ||
			origin == "http://127.0.0.1:8080" ||
			origin == "http://192.168.6.30:8080" {
			return true
		}

		// 生产环境建议严格校验，这里暂时放行所有以保证前端能连上
		slog.Warn("WebSocket 允许了未知的 Origin 连接", "origin", origin)
		return true
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func NewServer(processor *drone.Processor, wsManager *ws.Manager, sniffer *drone.Sniffer) *Server {
	// 💡 优化 2：设置 Gin 模式，生产环境自动关闭调试日志
	gin.SetMode(gin.ReleaseMode)

	ctx, cancel := context.WithCancel(context.Background())
	r := gin.New()

	// 💡 优化 3：恢复 Gin 的标准请求日志，方便排查 API 问题
	// 如果实在不想看，可以改回 return ""，但强烈建议保留
	r.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return param.Method + " " + param.Path + " " + param.StatusCodeColor() + param.Latency.String() + "\n"
	}))
	r.Use(gin.Recovery())

	// 使用 Gin 官方 CORS 中间件
	r.Use(cors.New(cors.Config{
		AllowOrigins: []string{
			"http://localhost:8080",
			"http://192.168.6.30:8080",
			"http://127.0.0.1:8080",
			"http://localhost",
			"http://127.0.0.1",
			"http://localhost:3000",
			"http://127.0.0.1:3000",
			"http://192.168.6.30",
		},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	cfg := config.Get()

	s := &Server{
		engine:    r,
		processor: processor,
		wsManager: wsManager,
		sniffer:   sniffer,
		config:    cfg,
		ctx:       ctx,
		cancel:    cancel,
		startTime: time.Now(),
	}

	s.registerRoutes()

	httpSrv := &http.Server{
		Addr:    ":" + cfg.API.Port,
		Handler: r,
	}
	s.httpSrv = httpSrv

	go s.monitorConnections()

	return s
}

func (s *Server) Run(port string) error {
	slog.Info("API 服务启动", "addr", "0.0.0.0"+port)
	return s.httpSrv.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.cancel()
	return s.httpSrv.Shutdown(ctx)
}

func (s *Server) monitorConnections() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			count := s.wsManager.GetConnectionCount()
			if count > 0 {
				slog.Debug("WebSocket 活跃连接数", "count", count)
			}
		case <-s.ctx.Done():
			return
		}
	}
}
