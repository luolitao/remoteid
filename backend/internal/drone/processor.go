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
	parser     *RemoteIDParser
	wsManager  *ws.Manager
	mutex      sync.RWMutex
	drones     map[string]*types.DroneData
	alerts     map[string]*types.Alert
	positions  map[string][]*types.Position
	lastUpdate map[string]time.Time

	macMsgCounts map[string]int

	stats struct {
		totalDrones   int
		totalMessages int
		lastUpdate    time.Time
	}
}

func NewProcessor(wsManager *ws.Manager) *Processor {
	p := &Processor{
		parser:       NewParser(),
		wsManager:    wsManager,
		drones:       make(map[string]*types.DroneData),
		alerts:       make(map[string]*types.Alert),
		positions:    make(map[string][]*types.Position),
		lastUpdate:   make(map[string]time.Time),
		macMsgCounts: make(map[string]int),
	}
	return p
}

// ProcessDroneData 处理无人机数据
func (p *Processor) ProcessDroneData(mac string, messages []types.DroneMessage) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	drone, exists := p.drones[mac]
	if !exists {
		drone = &types.DroneData{
			MAC:       mac,
			FirstSeen: time.Now(),
		}
		p.drones[mac] = drone
		p.stats.totalDrones++
		slog.Info("🛸 发现新无人机", "mac", mac, "first_seen", drone.FirstSeen.Format(time.RFC3339))
	}

	p.macMsgCounts[mac]++

	rawHex := ""
	if len(messages) > 0 {
		rawHex = messages[0].RawHex
	}

	// 协议识别（仅一次）
	for _, msg := range messages {
		msgStandard := strings.TrimSpace(msg.Standard)
		if msgStandard != "" && drone.Standard == "" {
			slog.Info("📡 成功解析 Remote ID 协议",
				"mac", mac,
				"protocol", msgStandard,
				"msg_type", strings.TrimSpace(msg.MessageType),
				"raw_hex", rawHex,
			)
			drone.Standard = msgStandard
			break
		}
	}

	// 合并数据（通用 + 协议特定）
	p.parser.MergeMessages(drone, messages)

	// 高度合理性检查（仅通用高度）
	if drone.Altitude == -999 || drone.Altitude == -1000 {
		drone.Altitude = 0 // 静默忽略固件无效值
	} else if drone.Altitude > 1000 || drone.Altitude < 10 {
		slog.Warn("高度数据异常",
			"mac", mac,
			"altitude", drone.Altitude,
			"raw_hex", rawHex,
		)
		drone.Altitude = 0
	}

	// 更新最后活动时间
	drone.LastSeen = time.Now()
	p.lastUpdate[mac] = time.Now()
	p.stats.totalMessages += len(messages)
	p.stats.lastUpdate = time.Now()

	// 保存无人机信息（包含协议特定数据）
	if err := db.SaveDrone(drone); err != nil {
		slog.Warn("保存无人机数据失败", "mac", mac, "error", err)
	}

	// 保存位置（通用坐标）
	if drone.Latitude != 0 && drone.Longitude != 0 {
		pos := &types.Position{
			Latitude:  drone.Latitude,
			Longitude: drone.Longitude,
			Altitude:  drone.Altitude,
			Speed:     drone.Speed,
			Heading:   drone.Heading,
			Timestamp: time.Now(),
		}
		p.positions[mac] = append(p.positions[mac], pos)
		if err := db.SavePosition(mac, pos, drone.Standard); err != nil {
			slog.Warn("保存位置数据失败", "mac", mac, "error", err)
		}
	}

	// 广播 WebSocket
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

func (p *Processor) GetDroneTrajectory(mac string, hours int) *types.Trajectory {
	// 优先从数据库获取
	dbPositions, err := db.GetTrajectory(mac, hours)
	if err == nil && len(dbPositions) > 0 {
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

	// 降级查询内存
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

func (p *Processor) GetAlerts(limit, offset int) ([]*types.Alert, int) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	all := make([]*types.Alert, 0, len(p.alerts))
	for _, a := range p.alerts {
		all = append(all, a)
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].CreatedAt.After(all[j].CreatedAt)
	})
	total := len(all)
	start := offset
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}
	return all[start:end], total
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

func (p *Processor) GetProcessorStats() types.ProcessorStats {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return types.ProcessorStats{
		TotalDrones:   p.stats.totalDrones,
		TotalMessages: p.stats.totalMessages,
		LastUpdate:    p.stats.lastUpdate,
	}
}
