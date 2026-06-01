// internal/api/routes.go
package api

import (
	"log/slog"
	"net/http"
	"time"

	"remoteid-monitor/pkg/ws"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func (s *Server) registerRoutes() {
	// 健康检查（含 sniffer 状态）
	s.engine.GET("/health", func(c *gin.Context) {
		health := gin.H{
			"status":    "healthy",
			"timestamp": time.Now().Format(time.RFC3339),
			"version":   "1.0.0",
		}

		// 添加抓包状态信息
		if s.sniffer != nil {
			lastPacket := s.sniffer.GetLastPacketTime()
			health["sniffer"] = gin.H{
				"active":          !lastPacket.IsZero(),
				"last_packet":     lastPacket.Format(time.RFC3339),
				"seconds_since":   time.Since(lastPacket).Seconds(),
				"stale":           !lastPacket.IsZero() && time.Since(lastPacket) > 60*time.Second,
			}
		}

		c.JSON(http.StatusOK, health)
	})

	// API 路由组
	api := s.engine.Group("/api")
	{
		// 无人机路由
		drones := api.Group("/drones")
		{
			drones.GET("/", s.listDrones)
			drones.GET("/:mac", s.getDroneDetails)
			drones.GET("/:mac/trajectory", s.getTrajectory)
			drones.GET("/:mac/export", s.exportDroneData)
			drones.GET("/search", s.searchDrones)
			drones.GET("/statistics", s.getDroneStatistics)
		}

		// 警报路由
		alerts := api.Group("/alerts")
		{
			alerts.GET("/", s.listAlerts)
			alerts.POST("/", s.createAlert)
			alerts.GET("/:id", s.getAlertDetails)
			alerts.POST("/:id/resolve", s.resolveAlert)
			alerts.GET("/statistics", s.getAlertStatistics)
			alerts.DELETE("/clear", s.clearAlerts)
			alerts.GET("/search", s.searchAlerts)
		}

		// 系统路由
		system := api.Group("/system")
		{
			system.GET("/info", s.getSystemInfo)
			system.GET("/config", s.getConfig)
		}
	}

	// WebSocket 路由
	s.engine.GET("/ws", s.websocketHandler)

	slog.Info("所有 API 路由已注册")
}

// WebSocket 处理器
func (s *Server) websocketHandler(c *gin.Context) {
	origin := c.GetHeader("Origin")

	// 检查是否为允许的来源
	allowedOrigins := []string{
		"http://localhost:8080",
		"http://127.0.0.1:8080",
		"http://192.168.6.30:8080",
		"http://192.168.6.30",
		"http://rpi5.lan",
		"http://rpi5.local",
	}

	isAllowed := false
	for _, allowed := range allowedOrigins {
		if origin == allowed {
			isAllowed = true
			break
		}
	}

	if isAllowed {
		c.Header("Access-Control-Allow-Origin", origin)
	} else {
		c.Header("Access-Control-Allow-Origin", "*")
	}

	c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	c.Header("Access-Control-Allow-Headers", "*")
	c.Header("Access-Control-Allow-Credentials", "true")

	if c.Request.Method == "OPTIONS" {
		c.Status(http.StatusOK)
		return
	}

	// 正常的 WebSocket 连接处理
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Error("WebSocket 升级失败", "error", err)
		return
	}

	client := &ws.Client{
		Conn:    conn,
		Send:    make(chan []byte, 256),
		IP:      c.ClientIP(),
		Created: time.Now(),
	}

	s.wsManager.Register(client)
	defer s.wsManager.Unregister(client)

	go client.WritePump()

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Warn("WebSocket 读取错误", "error", err)
			}
			break
		}
	}
}
