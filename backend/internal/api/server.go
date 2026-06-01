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
	sniffer   *drone.Sniffer // sniffer 引用，用于健康检查
	config    *config.Config
	ctx       context.Context
	cancel    context.CancelFunc
	// 2. +++ 添加：启动时间字段 +++
	startTime time.Time
}

// WebSocket Upgrader
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")

		allowedOrigins := []string{
			"http://localhost:8080",
			"http://127.0.0.1:8080",
			"http://192.168.6.30:8080",
			"http://192.168.6.30",
		}

		for _, allowed := range allowedOrigins {
			if origin == allowed {
				return true
			}
		}

		return true
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func NewServer(processor *drone.Processor, wsManager *ws.Manager, sniffer *drone.Sniffer) *Server {
	ctx, cancel := context.WithCancel(context.Background())

	r := gin.New()

	r.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return ""
	}))
	r.Use(gin.Recovery())

	// 使用 Gin 官方 CORS 中间件
	r.Use(cors.New(cors.Config{
		AllowOrigins: []string{
			"http://localhost:8080", // Vite 开发服务器
			"http://192.168.6.30:8080",
			"http://127.0.0.1:8080",
			"http://localhost",
			"http://127.0.0.1",
			"http://localhost:3000", // React 开发服务器
			"http://127.0.0.1:3000",
			// 生产环境域名可以在这里添加

			"http://192.168.6.30",
		},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// 注册路由
	s := &Server{
		engine:    r,
		processor: processor,
		wsManager: wsManager,
		sniffer:   sniffer,
		config:    config.Get(), // 确保 config 已正确初始化
		ctx:       ctx,
		cancel:    cancel,
		startTime: time.Now(),
	}

	s.registerRoutes()

	// 创建 http.Server
	httpSrv := &http.Server{
		Addr:    ":" + config.Get().API.Port,
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
				slog.Debug("WebSocket 连接数", "count", count)
			}
		case <-s.ctx.Done():
			return
		}
	}
}
