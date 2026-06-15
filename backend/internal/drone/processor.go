package drone

import (
	"log/slog"
	"remoteid-monitor/internal/db"
	"remoteid-monitor/pkg/types"
	"remoteid-monitor/pkg/ws"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const activeDroneTimeout = 30 * time.Second

type Processor struct {
	wsManager *ws.Manager
	mutex     sync.RWMutex

	drones     map[string]*types.DroneData
	positions  map[string][]*types.Position
	lastUpdate map[string]time.Time
	alerts     map[string]*types.Alert

	stats struct {
		totalDrones   int
		totalMessages int
		lastUpdate    time.Time
	}
}

func NewProcessor(wsManager *ws.Manager) *Processor {
	return &Processor{

		wsManager:  wsManager,
		drones:     make(map[string]*types.DroneData),
		positions:  make(map[string][]*types.Position),
		lastUpdate: make(map[string]time.Time),
		alerts:     make(map[string]*types.Alert),
	}
}

func (p *Processor) ProcessDroneData(mac string, messages []types.DroneMessage) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	drone, exists := p.drones[mac]
	if !exists {
		drone = &types.DroneData{MAC: mac, FirstSeen: time.Now()}
		p.stats.totalDrones++
	}

	for _, msg := range messages {
		MapMessageToDrone(drone, msg, &p.positions)
	}

	drone.LastSeen = time.Now()
	p.drones[mac] = drone
	p.lastUpdate[mac] = time.Now()
	p.stats.totalMessages += len(messages)
	p.stats.lastUpdate = time.Now()

	if err := db.SaveDrone(drone); err != nil {
		slog.Warn("保存无人机数据失败", "mac", mac, "error", err)
	}

	if p.wsManager != nil {
		p.wsManager.BroadcastDroneUpdate(drone)
	}
}

// ================= 查询与导出方法 (API 依赖) =================

func (p *Processor) GetAllDrones() []*types.DroneData {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	result := make([]*types.DroneData, 0, len(p.drones))
	for _, d := range p.drones {
		result = append(result, d)
	}
	return result
}

func (p *Processor) GetDroneByMAC(mac string) *types.DroneData {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.drones[mac]
}

func (p *Processor) ListActiveDrones() []*types.DroneData {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	var active []*types.DroneData
	now := time.Now()
	for _, d := range p.drones {
		if now.Sub(d.LastSeen) < activeDroneTimeout {
			active = append(active, d)
		}
	}
	return active
}

// GetDroneTrajectory 获取无人机轨迹 (优先查数据库，降级查内存)
func (p *Processor) GetDroneTrajectory(mac string, hours int) *types.Trajectory {
	// 1. 优先从数据库获取历史轨迹
	dbPositions, err := db.GetTrajectory(mac, hours)
	if err == nil && len(dbPositions) > 0 {
		// 将 []types.Position 转换为 []*types.Position 以匹配结构体定义
		points := make([]*types.Position, len(dbPositions))
		for i := range dbPositions {
			points[i] = &dbPositions[i]
		}
		return &types.Trajectory{
			MAC:     mac,
			Points:  points,
			Count:   len(points),
			Updated: time.Now(),
		}
	}

	// 2. 数据库无数据时，降级查询内存 (应对服务刚启动或极短期的轨迹)
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	if memPositions, exists := p.positions[mac]; exists && len(memPositions) > 0 {
		return &types.Trajectory{
			MAC:     mac,
			Points:  memPositions,
			Count:   len(memPositions),
			Updated: time.Now(),
		}
	}

	return nil
}

func (p *Processor) ExportDroneData(mac string) *types.ExportData {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	drone, exists := p.drones[mac]
	if !exists {
		return nil
	}
	return &types.ExportData{
		Drone:     drone,
		Positions: p.positions[mac],
		Exported:  time.Now(),
	}
}

func (p *Processor) SearchDrones(query string) []*types.DroneData {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	q := strings.ToLower(query)
	var results []*types.DroneData
	for _, d := range p.drones {
		if strings.Contains(strings.ToLower(d.MAC), q) ||
			strings.Contains(strings.ToLower(d.UASID), q) ||
			strings.Contains(strings.ToLower(d.UAType), q) {
			results = append(results, d)
		}
	}
	return results
}

func (p *Processor) GetDroneStatistics() *types.DroneStatistics {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	stats := &types.DroneStatistics{
		TotalDrones: len(p.drones),
		LastUpdate:  time.Now(),
	}
	now := time.Now()
	for _, drone := range p.drones {
		if now.Sub(drone.LastSeen) < activeDroneTimeout {
			stats.ActiveDrones++
		} else {
			stats.InactiveDrones++
		}
	}
	return stats
}

// ================= 告警管理方法 (API 依赖) =================

func (p *Processor) CreateAlert(alert *types.Alert) *types.Alert {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if alert.ID == "" {
		alert.ID = generateAlertID()
	}
	alert.CreatedAt = time.Now()
	alert.UpdatedAt = time.Now()
	alert.Resolved = false
	p.alerts[alert.ID] = alert

	if _, err := db.CreateAlert(alert.Type, alert.Message, alert.TargetMAC); err != nil {
		slog.Warn("保存告警到数据库失败", "error", err)
	}
	if p.wsManager != nil {
		p.wsManager.BroadcastAlert(alert)
	}
	return alert
}

func (p *Processor) GetAlertByID(id string) *types.Alert {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.alerts[id]
}

func (p *Processor) ResolveAlert(id string) bool {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if alert, exists := p.alerts[id]; exists {
		alert.Resolved = true
		resolvedAt := time.Now()
		alert.ResolvedAt = &resolvedAt
		alert.UpdatedAt = time.Now()
		if p.wsManager != nil {
			p.wsManager.BroadcastAlert(alert)
		}
		return true
	}
	return false
}

func (p *Processor) GetAlerts(limit string) []*types.Alert {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	alerts := make([]*types.Alert, 0, len(p.alerts))
	for _, a := range p.alerts {
		alerts = append(alerts, a)
	}
	sort.Slice(alerts, func(i, j int) bool {
		return alerts[i].CreatedAt.After(alerts[j].CreatedAt)
	})
	if limit != "" {
		if n, err := strconv.Atoi(limit); err == nil && n > 0 && n < len(alerts) {
			alerts = alerts[:n]
		}
	}
	return alerts
}

func (p *Processor) GetAlertStatistics() *types.AlertStatistics {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	stats := &types.AlertStatistics{
		TotalAlerts: len(p.alerts),
		LastUpdate:  time.Now(),
	}
	for _, alert := range p.alerts {
		if alert.Resolved {
			stats.ResolvedAlerts++
		} else {
			stats.PendingAlerts++
		}
	}
	return stats
}

func (p *Processor) ClearAllAlerts() int {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	count := len(p.alerts)
	p.alerts = make(map[string]*types.Alert)

	// 如果 db 包中有此方法，可取消注释
	// if _, err := db.ClearOldAlerts(1); err != nil {
	// 	slog.Warn("清除旧警报失败", "error", err)
	// }
	return count
}

func (p *Processor) SearchAlerts(query string) []*types.Alert {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	q := strings.ToLower(query)
	var results []*types.Alert
	for _, alert := range p.alerts {
		if strings.Contains(strings.ToLower(alert.ID), q) ||
			strings.Contains(strings.ToLower(alert.Message), q) ||
			strings.Contains(strings.ToLower(alert.Category), q) {
			results = append(results, alert)
		}
	}
	return results
}

func generateAlertID() string {
	return "alert_" + strconv.FormatInt(time.Now().UnixNano(), 10)
}

// 在 processor.go 文件末尾添加
func (p *Processor) GetProcessorStats() types.ProcessorStats {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return types.ProcessorStats{
		TotalDrones:   p.stats.totalDrones,
		TotalMessages: p.stats.totalMessages,
		LastUpdate:    p.stats.lastUpdate,
	}
}
