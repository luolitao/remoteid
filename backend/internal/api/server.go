package api

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"remoteid-monitor/internal/config"
	"remoteid-monitor/internal/drone"
	"remoteid-monitor/pkg/ws"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// StatusProvider 抽象出健康检查所需的接口
type StatusProvider interface {
	GetLastPacketTime() time.Time
}

var defaultAllowedOrigins = []string{
	"http://localhost:8080",
	"http://127.0.0.1:8080",
	"http://localhost:3000",
	"http://127.0.0.1:3000",
	"http://192.168.6.30:8080",
	"http://192.168.6.30",
}

type Server struct {
	engine         *gin.Engine
	httpSrv        *http.Server
	processor      *drone.Processor
	wsManager      *ws.Manager
	statusProv     StatusProvider
	config         *config.Config
	ctx            context.Context
	cancel         context.CancelFunc
	startTime      time.Time
	upgrader       *websocket.Upgrader
	allowedOrigins []string
}

func NewServer(processor *drone.Processor, wsManager *ws.Manager, statusProv StatusProvider) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	r := gin.New()
	r.Use(gin.Recovery())

	cfg := config.Get()
	allowedOrigins := cfg.API.CORSAllowOrigins
	if len(allowedOrigins) == 0 {
		allowedOrigins = defaultAllowedOrigins
	}

	r.Use(cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// 1. 先初始化基础结构体，确保所有变量名绝对正确，无空格
	s := &Server{
		engine:         r,
		processor:      processor,
		wsManager:      wsManager,
		statusProv:     statusProv,
		config:         cfg,
		ctx:            ctx,
		cancel:         cancel,
		startTime:      time.Now(),
		allowedOrigins: allowedOrigins,
	}

	// 2. 分步初始化 upgrader，这是 Go 中最安全、最无争议的写法
	s.upgrader = &websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return s.isOriginAllowed(r.Header.Get("Origin"))
		},
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	s.registerRoutes()

	addr := ":" + cfg.API.Port
	if cfg.API.Host != "" {
		addr = cfg.API.Host + ":" + cfg.API.Port
	}

	s.httpSrv = &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go s.monitorConnections()

	return s
}

func (s *Server) Run(port string) error {
	slog.Info("API 服务启动", "addr", s.httpSrv.Addr)
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

func (s *Server) isOriginAllowed(origin string) bool {
	if len(s.allowedOrigins) == 0 {
		return true
	}
	for _, allowed := range s.allowedOrigins {
		if origin == allowed {
			return true
		}
		if strings.HasPrefix(allowed, "*.") && strings.HasSuffix(origin, allowed[1:]) {
			return true
		}
	}
	return false
}
