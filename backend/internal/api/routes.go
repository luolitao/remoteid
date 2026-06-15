package api

import (
	"log/slog"
	"net/http"
	"time"

	"remoteid-monitor/internal/drone"
	"remoteid-monitor/pkg/ws"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func (s *Server) registerRoutes() {
	// 1. 健康检查（含抓包状态）
	s.engine.GET("/health", func(c *gin.Context) {
		health := gin.H{
			"status":    "healthy",
			"timestamp": time.Now().Format(time.RFC3339),
			"version":   "1.0.0",
		}

		// 使用 statusProv 接口获取状态，兼容 Sniffer 或未来的 Manager/BLE
		if s.statusProv != nil {
			lastPacket := s.statusProv.GetLastPacketTime()
			health["sniffer"] = gin.H{ // Key 保持 "sniffer" 以兼容前端
				"active":        !lastPacket.IsZero(),
				"last_packet":   lastPacket.Format(time.RFC3339),
				"seconds_since": time.Since(lastPacket).Seconds(),
				"stale":         !lastPacket.IsZero() && time.Since(lastPacket) > 60*time.Second,
			}
		}

		c.JSON(http.StatusOK, health)
	})

	// 2. API 路由组
	api := s.engine.Group("/api")
	{
		// 实时统计路由
		api.GET("/stats", s.getRealtimeStats)

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

	// 3. WebSocket 路由
	s.engine.GET("/ws", s.websocketHandler)
	slog.Info("所有 API 路由已注册")
}

// getRealtimeStats 返回实时的抓包与解析统计数据
func (s *Server) getRealtimeStats(c *gin.Context) {
	stats := gin.H{}

	// 尝试将 statusProv 断言为 drone.Manager 以获取详细统计
	if manager, ok := s.statusProv.(*drone.Manager); ok {
		stats["capture"] = manager.GetCaptureStats()
		stats["processor"] = manager.GetProcessor().GetProcessorStats()
	} else {
		// 降级处理
		stats["capture"] = gin.H{"status": "unavailable"}
		if s.processor != nil {
			stats["processor"] = s.processor.GetProcessorStats()
		}
	}

	c.JSON(http.StatusOK, stats)
}

// websocketHandler 处理 WebSocket 连接
func (s *Server) websocketHandler(c *gin.Context) {
	// 直接使用 s.upgrader (指针类型) 调用 Upgrade 方法，Gin 的 CORS 中间件已处理预检请求
	conn, err := s.upgrader.Upgrade(c.Writer, c.Request, nil)
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
