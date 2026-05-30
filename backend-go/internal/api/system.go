// internal/api/system.go
package api

import (
	"net/http"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
)

func (s *Server) getSystemInfo(c *gin.Context) {
	// 计算运行时间
	uptime := time.Since(s.startTime).String()

	// 获取运行时内存统计
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	info := gin.H{
		"version":       "1.0.0",
		"build_time":    time.Now().Format(time.RFC3339),
		"go_version":    runtime.Version(),
		"go_os":         runtime.GOOS,
		"go_arch":       runtime.GOARCH,
		"num_goroutine": runtime.NumGoroutine(),
		"num_cpu":       runtime.NumCPU(),
		"timestamp":     time.Now().Format(time.RFC3339),
		"uptime":        uptime,
		"memory": gin.H{
			"alloc_mb":    formatMB(m.Alloc),
			"sys_mb":      formatMB(m.Sys),
			"num_gc":      m.NumGC,
			"gc_pause_ns": m.PauseNs[(m.NumGC+255)%256],
		},
		"network": gin.H{
			"interface": s.config.Network.Interface,
			"channel":   s.config.Network.Channel,
			"mode":      s.config.Network.MonitorMode,
		},
		"database": gin.H{
			"path": s.config.Database.Path,
		},
	}

	c.JSON(http.StatusOK, info)
}

// formatMB 将字节转换为 MB，保留 2 位小数
func formatMB(bytes uint64) float64 {
	return float64(bytes) / 1024 / 1024
}

func (s *Server) getConfig(c *gin.Context) {
	config := gin.H{
		"database": gin.H{
			"path":            s.config.Database.Path,
			"max_connections": s.config.Database.MaxConnections,
			"cache_size":      s.config.Database.CacheSize,
		},
		"network": gin.H{
			"interface":    s.config.Network.Interface,
			"channel":      s.config.Network.Channel,
			"monitor_mode": s.config.Network.MonitorMode,
		},
		"api": gin.H{
			"port": s.config.API.Port,
			"cors": s.config.API.CORS,
		},
		"logging": gin.H{
			"level": s.config.Logging.Level,
			"file":  s.config.Logging.File,
		},
		"debug": s.config.Debug,
	}

	c.JSON(http.StatusOK, config)
}
