// internal/drone/processor.go
package drone

// 需要导入的包
import (
	"fmt"
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

const activeDroneTimeout = 30 * time.Second // 活跃无人机判定超时

type Processor struct {
	parser     *RemoteIDParser
	wsManager  *ws.Manager
	mutex      sync.RWMutex
	drones     map[string]*types.DroneData
	alerts     map[string]*types.Alert
	positions  map[string][]*types.Position
	lastUpdate map[string]time.Time
	// 3. +++ 添加：stats 字段 +++
	stats struct {
		totalDrones   int
		totalMessages int
		lastUpdate    time.Time
	}
}

func NewProcessor(wsManager *ws.Manager) *Processor {
	p := &Processor{
		parser:     NewParser(),
		wsManager:  wsManager,
		drones:     make(map[string]*types.DroneData),
		alerts:     make(map[string]*types.Alert),
		positions:  make(map[string][]*types.Position),
		lastUpdate: make(map[string]time.Time),
	}

	return p
}

// ProcessDroneData 处理无人机数据
func (p *Processor) ProcessDroneData(mac string, messages []types.DroneMessage) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// 创建或更新无人机数据
	drone, exists := p.drones[mac]
	if !exists {
		drone = &types.DroneData{
			MAC:       mac,
			FirstSeen: time.Now(),
		}
		p.stats.totalDrones++
	}

	// 更新无人机信息
	for _, msg := range messages {
		p.updateDroneFromMessage(drone, msg)
	}

	drone.LastSeen = time.Now()
	p.drones[mac] = drone
	p.lastUpdate[mac] = time.Now()
	p.stats.totalMessages += len(messages)
	p.stats.lastUpdate = time.Now()

	// 保存到数据库
	if err := db.SaveDrone(drone); err != nil {
		slog.Warn("保存无人机数据失败", "mac", mac, "error", err)
	}

	// 广播更新
	if p.wsManager != nil {
		p.wsManager.BroadcastDroneUpdate(drone)
	}
}

// 4. +++ 修复：获取所有无人机 +++
func (p *Processor) GetAllDrones() []*types.DroneData {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	var result []*types.DroneData
	for _, drone := range p.drones {
		result = append(result, drone)
	}
	return result
}

// 5. +++ 修复：根据 MAC 获取无人机 +++
func (p *Processor) GetDroneByMAC(mac string) *types.DroneData {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	if drone, exists := p.drones[mac]; exists {
		return drone
	}
	return nil
}

// 6. +++ 添加：获取活跃无人机列表 +++
func (p *Processor) ListActiveDrones() []*types.DroneData {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	var active []*types.DroneData
	now := time.Now()

	for _, drone := range p.drones {
		if now.Sub(drone.LastSeen) < activeDroneTimeout {
			active = append(active, drone)
		}
	}

	return active
}

// 7. +++ 修复：获取无人机轨迹 +++
func (p *Processor) GetDroneTrajectory(mac string) *types.Trajectory {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	if positions, exists := p.positions[mac]; exists {
		return &types.Trajectory{
			MAC:     mac,
			Points:  positions,
			Count:   len(positions),
			Updated: time.Now(),
		}
	}
	return nil
}

// 8. +++ 修复：导出无人机数据 +++
func (p *Processor) ExportDroneData(mac string) *types.ExportData {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	if drone, exists := p.drones[mac]; exists {
		return &types.ExportData{
			Drone:     drone,
			Positions: p.positions[mac],
			Exported:  time.Now(),
		}
	}
	return nil
}

// 9. +++ 修复：搜索无人机 +++
func (p *Processor) SearchDrones(query string) []*types.DroneData {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	var results []*types.DroneData
	for _, drone := range p.drones {
		if p.matchesSearchQuery(drone, query) {
			results = append(results, drone)
		}
	}
	return results
}

// 10. +++ 修复：获取无人机统计 +++
func (p *Processor) GetDroneStatistics() *types.DroneStatistics {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	stats := &types.DroneStatistics{
		TotalDrones:    len(p.drones),
		ActiveDrones:   0,
		InactiveDrones: 0,
		LastUpdate:     time.Now(),
	}

	now := time.Now()
	for _, drone := range p.drones {
		if now.Sub(p.lastUpdate[drone.MAC]) < activeDroneTimeout {
			stats.ActiveDrones++
		} else {
			stats.InactiveDrones++
		}
	}

	return stats
}

// 11. +++ 修复：获取警报 +++
func (p *Processor) GetAlerts(limit string) []*types.Alert {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	var alerts []*types.Alert
	for _, alert := range p.alerts {
		alerts = append(alerts, alert)
	}

	// 按时间排序（最新的在前）
	sort.Slice(alerts, func(i, j int) bool {
		return alerts[i].CreatedAt.After(alerts[j].CreatedAt)
	})

	// 限制数量
	if limit != "" {
		if n, err := strconv.Atoi(limit); err == nil && n > 0 && n < len(alerts) {
			alerts = alerts[:n]
		}
	}

	return alerts
}

// 12. +++ 修复：创建警报 +++
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

	// 保存到数据库
	if _, err := db.CreateAlert(alert.Type, alert.Message, alert.TargetMAC); err != nil {
		slog.Warn("保存告警到数据库失败", "error", err)
	}

	// 广播警报
	if p.wsManager != nil {
		p.wsManager.BroadcastAlert(alert)
	}

	return alert
}

// 13. +++ 修复：根据 ID 获取警报 +++
func (p *Processor) GetAlertByID(id string) *types.Alert {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	if alert, exists := p.alerts[id]; exists {
		return alert
	}
	return nil
}

// 14. +++ 修复：解决警报 +++
func (p *Processor) ResolveAlert(id string) bool {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if alert, exists := p.alerts[id]; exists {
		alert.Resolved = true
		// 15. +++ 修复：创建指向时间的指针 +++
		resolvedAt := time.Now()
		alert.ResolvedAt = &resolvedAt
		alert.UpdatedAt = time.Now()

		// 更新数据库
		// 注：db.ResolveAlert 接受 int 类型的 ID，processor 使用 string 类型的 ID
		// 两者 ID 体系不同，这里暂不直接调用。告警通过内存管理，持久化在创建时完成。

		// 广播更新
		if p.wsManager != nil {
			p.wsManager.BroadcastAlert(alert)
		}

		return true
	}
	return false
}

// 16. +++ 修复：获取警报统计 +++
func (p *Processor) GetAlertStatistics() *types.AlertStatistics {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	stats := &types.AlertStatistics{
		TotalAlerts:    len(p.alerts),
		ResolvedAlerts: 0,
		PendingAlerts:  0,
		LastUpdate:     time.Now(),
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

// 17. +++ 修复：清除所有警报 +++
func (p *Processor) ClearAllAlerts() int {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	count := len(p.alerts)

	// 清除所有警报
	p.alerts = make(map[string]*types.Alert)

	// 从数据库删除已解决的旧警报（保留 1 小时内的）
	if _, err := db.ClearOldAlerts(1); err != nil {
		slog.Warn("清除旧警报失败", "error", err)
	}

	return count
}

// 18. +++ 修复：搜索警报 +++
func (p *Processor) SearchAlerts(query string) []*types.Alert {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	var results []*types.Alert
	for _, alert := range p.alerts {
		if p.matchesAlertSearchQuery(alert, query) {
			results = append(results, alert)
		}
	}
	return results
}

// 19. +++ 修复：解析帧 +++
func (p *Processor) ParseFrame(raw []byte) ([]types.DroneMessage, error) {
	return p.parser.ParseFrame(raw)
}

// 辅助方法
func (p *Processor) updateDroneFromMessage(drone *types.DroneData, msg types.DroneMessage) {
	switch msg.MessageType {
	case "basic_id":
		drone.UASID = msg.Data["uas_id"]
		drone.UAType = msg.Data["ua_type"]
		drone.IDType = msg.Data["id_type"]
		drone.Standard = msg.Standard
		drone.Source = msg.Source
	case "location":
		if lat, ok := msg.Data["latitude"]; ok {
			if f, err := strconv.ParseFloat(lat, 64); err == nil {
				drone.Latitude = f
			}
		}
		if lng, ok := msg.Data["longitude"]; ok {
			if f, err := strconv.ParseFloat(lng, 64); err == nil {
				drone.Longitude = f
			}
		}
		// 优先用 altitude_baro（气压高度），其次 altitude_geo，最后 altitude
		if alt, ok := msg.Data["altitude_baro"]; ok {
			if f, err := strconv.ParseFloat(alt, 64); err == nil {
				drone.Altitude = f
			}
		} else if alt, ok := msg.Data["altitude_geo"]; ok {
			if f, err := strconv.ParseFloat(alt, 64); err == nil {
				drone.Altitude = f
			}
		} else if alt, ok := msg.Data["altitude"]; ok {
			if f, err := strconv.ParseFloat(alt, 64); err == nil {
				drone.Altitude = f
			}
		}
		// 高度补充：height_m（离地高度）
		if height, ok := msg.Data["height_m"]; ok && drone.Altitude == 0 {
			if f, err := strconv.ParseFloat(height, 64); err == nil && f > 0 {
				drone.Altitude = f
			}
		}
		// 高度合理性检查：无人机限高通常不超过 500 米（模拟器信号不超过 120 米）
		// 如果高度超过 5000 米，很可能是解析错误，标记为无效
		if drone.Altitude > 5000 || drone.Altitude < -500 {
			slog.Warn("高度数据异常，可能是协议解析错误", "mac", drone.MAC, "altitude", drone.Altitude, "standard", msg.Standard)
			drone.Altitude = 0
		}
		if speed, ok := msg.Data["speed_h"]; ok {
			if f, err := strconv.ParseFloat(speed, 64); err == nil {
				drone.Speed = f
			}
		}
		if heading, ok := msg.Data["direction"]; ok {
			if f, err := strconv.ParseFloat(heading, 64); err == nil {
				drone.Heading = f
			}
		}
		// 垂直速度
		if speedV, ok := msg.Data["speed_v"]; ok {
			if f, err := strconv.ParseFloat(speedV, 64); err == nil {
				drone.SpeedVertical = f
			}
		}
		// 飞行状态
		if status, ok := msg.Data["status"]; ok {
			drone.FlightStatus = status
		}
		// 高度类型
		if ht, ok := msg.Data["height_type"]; ok {
			drone.HeightType = ht
		}
		// 精度信息
		if ha, ok := msg.Data["h_accuracy"]; ok {
			drone.HAccuracy = ha
		}
		if va, ok := msg.Data["v_accuracy"]; ok {
			drone.VAccuracy = va
		}
		if sa, ok := msg.Data["s_accuracy"]; ok {
			drone.SAccuracy = sa
		}
		// 位置时间戳
		if ts, ok := msg.Data["timestamp"]; ok {
			drone.LocationTimestamp = ts
		}
		// 添加位置到轨迹
		pos := &types.Position{
			Latitude:  drone.Latitude,
			Longitude: drone.Longitude,
			Altitude:  drone.Altitude,
			Speed:     drone.Speed,
			Heading:   drone.Heading,
			Timestamp: time.Now(),
		}
		p.positions[drone.MAC] = append(p.positions[drone.MAC], pos)

		// 保存位置到数据库
		if err := db.SavePosition(drone.MAC, pos, drone.Standard); err != nil {
			slog.Warn("保存位置数据失败", "mac", drone.MAC, "error", err)
		}

	case "operator_id":
		if opID, ok := msg.Data["operator_id"]; ok && opID != "" {
			drone.OperatorID = opID
		}
	case "system":
		if opLat, ok := msg.Data["operator_lat"]; ok {
			if f, err := strconv.ParseFloat(opLat, 64); err == nil {
				drone.OperatorLatitude = f
			}
		}
		if opLon, ok := msg.Data["operator_lon"]; ok {
			if f, err := strconv.ParseFloat(opLon, 64); err == nil {
				drone.OperatorLongitude = f
			}
		}
		if opAlt, ok := msg.Data["operator_alt"]; ok {
			if f, err := strconv.ParseFloat(opAlt, 64); err == nil {
				drone.OperatorAltitude = f
			}
		}
		if classification, ok := msg.Data["classification"]; ok {
			drone.Classification = classification
		}
		if areaRadius, ok := msg.Data["area_radius_m"]; ok {
			if r, err := strconv.Atoi(areaRadius); err == nil && r > 0 {
				drone.AreaRadiusM = r
			}
		}
	// GB 46750-2023: 所有数据在一个消息中，字段名不同于 ASTM/GB 42590
	case "gb46750":
		drone.Standard = msg.Standard
		drone.Source = msg.Source

		// Basic ID 映射：唯一产品识别码
		if uasID, ok := msg.Data["unique_id"]; ok && uasID != "" {
			drone.UASID = uasID
		}
		// 实名登记标志作为 OperatorID
		if realnameID, ok := msg.Data["realname_id"]; ok && realnameID != "" {
			drone.OperatorID = realnameID
		}
		// 无人机分类
		if uaCat, ok := msg.Data["ua_category"]; ok {
			drone.UAType = uaCat
		}
		drone.IDType = "SerialNumber"

		// Location 映射：无人机位置
		if lat, ok := msg.Data["latitude"]; ok {
			if f, err := strconv.ParseFloat(lat, 64); err == nil {
				drone.Latitude = f
			}
		}
		if lng, ok := msg.Data["longitude"]; ok {
			if f, err := strconv.ParseFloat(lng, 64); err == nil {
				drone.Longitude = f
			}
		}
		// 高度：优先气压高度，其次大地高度，最后相对高度
		if alt, ok := msg.Data["altitude_baro"]; ok {
			if f, err := strconv.ParseFloat(alt, 64); err == nil {
				drone.Altitude = f
			}
		} else if alt, ok := msg.Data["altitude_geo"]; ok {
			if f, err := strconv.ParseFloat(alt, 64); err == nil {
				drone.Altitude = f
			}
		} else if height, ok := msg.Data["height_m"]; ok {
			if f, err := strconv.ParseFloat(height, 64); err == nil {
				drone.Altitude = f
			}
		}
		// 高度合理性检查
		if drone.Altitude > 5000 || drone.Altitude < -500 {
			slog.Warn("高度数据异常，可能是协议解析错误", "mac", drone.MAC, "altitude", drone.Altitude, "standard", msg.Standard)
			drone.Altitude = 0
		}
		// 地速
		if speed, ok := msg.Data["speed_h"]; ok {
			if f, err := strconv.ParseFloat(speed, 64); err == nil {
				drone.Speed = f
			}
		}
		// 垂直速度
		if speedV, ok := msg.Data["speed_v"]; ok {
			if f, err := strconv.ParseFloat(speedV, 64); err == nil {
				drone.SpeedVertical = f
			}
		}
		// 航迹角
		if heading, ok := msg.Data["direction"]; ok {
			if f, err := strconv.ParseFloat(heading, 64); err == nil {
				drone.Heading = f
			}
		}
		// 运行状态
		if status, ok := msg.Data["status"]; ok {
			drone.FlightStatus = status
		}
		// 高度类型
		if ht, ok := msg.Data["height_type"]; ok {
			drone.HeightType = ht
		}
		// 精度信息
		if ha, ok := msg.Data["h_accuracy"]; ok {
			drone.HAccuracy = ha
		}
		if va, ok := msg.Data["v_accuracy"]; ok {
			drone.VAccuracy = va
		}
		if sa, ok := msg.Data["s_accuracy"]; ok {
			drone.SAccuracy = sa
		}
		// 时间戳（GB 46750 使用 Unix 毫秒）
		if ts, ok := msg.Data["timestamp"]; ok {
			drone.LocationTimestamp = ts
		}

		// System 映射：遥控站信息
		if rcsLat, ok := msg.Data["rcs_latitude"]; ok {
			if f, err := strconv.ParseFloat(rcsLat, 64); err == nil {
				drone.OperatorLatitude = f
			}
		}
		if rcsLon, ok := msg.Data["rcs_longitude"]; ok {
			if f, err := strconv.ParseFloat(rcsLon, 64); err == nil {
				drone.OperatorLongitude = f
			}
		}
		if rcsAlt, ok := msg.Data["rcs_altitude"]; ok {
			if f, err := strconv.ParseFloat(rcsAlt, 64); err == nil {
				drone.OperatorAltitude = f
			}
		}
		// 运行类别
		if opCat, ok := msg.Data["operation_category"]; ok {
			drone.Classification = "GB46750-Cat" + opCat
		}

		// 添加位置到轨迹
		if drone.Latitude != 0 || drone.Longitude != 0 {
			pos := &types.Position{
				Latitude:  drone.Latitude,
				Longitude: drone.Longitude,
				Altitude:  drone.Altitude,
				Speed:     drone.Speed,
				Heading:   drone.Heading,
				Timestamp: time.Now(),
			}
			p.positions[drone.MAC] = append(p.positions[drone.MAC], pos)

			// 保存位置到数据库
			if err := db.SavePosition(drone.MAC, pos, drone.Standard); err != nil {
				slog.Warn("保存位置数据失败", "mac", drone.MAC, "error", err)
			}
		}
	}
}

func (p *Processor) matchesSearchQuery(drone *types.DroneData, query string) bool {
	query = strings.ToLower(query)

	if strings.Contains(strings.ToLower(drone.MAC), query) {
		return true
	}
	if strings.Contains(strings.ToLower(drone.UASID), query) {
		return true
	}
	if strings.Contains(strings.ToLower(drone.UAType), query) {
		return true
	}

	return false
}

func (p *Processor) matchesAlertSearchQuery(alert *types.Alert, query string) bool {
	query = strings.ToLower(query)

	if strings.Contains(strings.ToLower(alert.ID), query) {
		return true
	}
	if strings.Contains(strings.ToLower(alert.Message), query) {
		return true
	}
	if strings.Contains(strings.ToLower(alert.Category), query) {
		return true
	}

	return false
}

// 辅助函数
func generateAlertID() string {
	return fmt.Sprintf("alert_%d", time.Now().UnixNano())
}
