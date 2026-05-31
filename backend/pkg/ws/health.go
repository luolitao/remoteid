// pkg/ws/health.go
package ws

// 需要导入的包
import (
	"runtime"
	"time"
)

type HealthMonitor struct {
	manager   *Manager
	lastCheck time.Time
}

func NewHealthMonitor(manager *Manager) *HealthMonitor {
	return &HealthMonitor{
		manager:   manager,
		lastCheck: time.Now(),
	}
}

// GetSystemHealth 获取系统健康状况
func (hm *HealthMonitor) GetSystemHealth() map[string]interface{} {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// 1. +++ 修复：获取客户端连接数 +++
	connCount := hm.manager.GetConnectionCount()

	health := map[string]interface{}{
		"timestamp":    time.Now().Format(time.RFC3339),
		"uptime":       time.Since(hm.lastCheck).String(),
		"connections":  connCount, // 使用正确的连接数
		"memory_usage": m.Sys,
		"goroutines":   runtime.NumGoroutine(),
		"cpu_count":    runtime.NumCPU(),
		"heap_alloc":   m.Alloc,
		"heap_sys":     m.Sys,
		"gc_cycles":    m.NumGC,
	}

	hm.lastCheck = time.Now()
	return health
}
