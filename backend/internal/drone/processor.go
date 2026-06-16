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

	// ✅ 新增：记录每个 MAC 地址收到的消息次数，用于控制日志打印频率
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
		macMsgCounts: make(map[string]int), // ✅ 新增初始化
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

		// 🛸 1. 首次发现新无人机 (天然只打印一次)
		slog.Info("🛸 发现新无人机", "mac", mac, "first_seen", drone.FirstSeen.Format(time.RFC3339))
	}

	// ✅ 2. 增加该 MAC 的计数，并判断是否为日志打印时机
	p.macMsgCounts[mac]++
	msgCount := p.macMsgCounts[mac]
	isLogTime := (msgCount == 1) || (msgCount%100 == 0) // 仅在第 1 次，或每 100 次打印

	// 提取 raw_hex (从第一条消息中获取，供日志使用)
	rawHex := ""
	if len(messages) > 0 {
		rawHex = messages[0].RawHex
	}

	// 3. 遍历解析出的消息
	for _, msg := range messages {
		msgStandard := strings.TrimSpace(msg.Standard)
		msgType := strings.TrimSpace(msg.MessageType)

		// 📡 协议识别 (仅在 drone.Standard 为空时打印一次)
		if msgStandard != "" && drone.Standard == "" {
			slog.Info("📡 成功解析 Remote ID 协议",
				"mac", mac,
				"protocol", msgStandard,
				"msg_type", msgType,
				"uas_id", strings.TrimSpace(msg.Data["uas_id"]),
				"raw_hex", rawHex)
			drone.Standard = msgStandard
		}

		// 📶 传输方式识别 (首次或定期打印)
		if transport, ok := msg.Data["transport"]; ok && transport != "" {
			transportUpper := strings.ToUpper(strings.TrimSpace(transport))
			if drone.Source == "" {
				drone.Source = transportUpper
			}

			// ✅ 核心：仅在 isLogTime 时打印，彻底解决刷屏问题
			if isLogTime {
				slog.Info("📶 识别到传输方式",
					"mac", mac,
					"transport", transportUpper,
					"msg_count", msgCount, // 附带当前计数，方便观察数据流稳定性
					"raw_hex", rawHex) // 附带 16 进制 Payload
			}
		}

		// 执行字段更新
		p.updateDroneFromMessage(drone, msg)
	}

	// 4. 更新状态与统计
	drone.LastSeen = time.Now()
	p.lastUpdate[mac] = time.Now()
	p.stats.totalMessages += len(messages)
	p.stats.lastUpdate = time.Now()

	// 5. 保存到数据库
	if err := db.SaveDrone(drone); err != nil {
		slog.Warn("保存无人机数据失败", "mac", mac, "error", err)
	}

	// 6. 广播 WebSocket 更新
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

func (p *Processor) updateDroneFromMessage(drone *types.DroneData, msg types.DroneMessage) {
	// ✅ 终极防御：自动清洗 parser.go 传来的所有幽灵空格
	msgType := strings.TrimSpace(msg.MessageType)

	// 清洗 Data map 的所有 key 和 value
	cleanData := make(map[string]string, len(msg.Data))
	for k, v := range msg.Data {
		cleanData[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}

	// 辅助函数：从清洗后的 map 中安全获取值
	getStr := func(key string) string { return cleanData[key] }
	getFloat := func(key string) float64 {
		if f, err := strconv.ParseFloat(cleanData[key], 64); err == nil {
			return f
		}
		return 0
	}
	getInt := func(key string) int {
		if i, err := strconv.Atoi(cleanData[key]); err == nil {
			return i
		}
		return 0
	}

	switch msgType {
	case "basic_id":
		if uasID := getStr("uas_id"); uasID != "" && uasID != "UNKNOWN" {
			drone.UASID = uasID
		}
		if uaType := getStr("ua_type"); uaType != "" {
			drone.UAType = uaType
		}
		if idType := getStr("id_type"); idType != "" {
			drone.IDType = idType
		}
		drone.Standard = strings.TrimSpace(msg.Standard)
		drone.Source = strings.TrimSpace(msg.Source)

	case "location":
		if lat := getFloat("latitude"); lat != 0 {
			drone.Latitude = lat
		}
		if lon := getFloat("longitude"); lon != 0 {
			drone.Longitude = lon
		}

		// 高度优先级：baro > geo > height_m
		altBaro := getFloat("altitude_baro")
		altGeo := getFloat("altitude_geo")
		heightM := getFloat("height_m")

		if altBaro != 0 {
			drone.Altitude = altBaro
		} else if altGeo != 0 {
			drone.Altitude = altGeo
		} else if heightM != 0 && drone.Altitude == 0 {
			drone.Altitude = heightM
		}

		// 高度合理性检查
		if drone.Altitude > 5000 || drone.Altitude < -500 {
			slog.Warn("高度数据异常", "mac", drone.MAC, "altitude", drone.Altitude, "raw_hex", msg.RawHex)
			drone.Altitude = 0
		}

		if speed := getFloat("speed_h"); speed != 0 {
			drone.Speed = speed
		}
		if heading := getFloat("direction"); heading != 0 {
			drone.Heading = heading
		}
		if speedV := getFloat("speed_v"); speedV != 0 {
			drone.SpeedVertical = speedV
		}

		if status := getStr("status"); status != "" {
			drone.FlightStatus = status
		}
		if ht := getStr("height_type"); ht != "" {
			drone.HeightType = ht
		}
		if ha := getStr("h_accuracy"); ha != "" {
			drone.HAccuracy = ha
		}
		if va := getStr("v_accuracy"); va != "" {
			drone.VAccuracy = va
		}
		if sa := getStr("s_accuracy"); sa != "" {
			drone.SAccuracy = sa
		}
		if ts := getStr("timestamp"); ts != "" {
			drone.LocationTimestamp = ts
		}

		// 记录轨迹
		if drone.Latitude != 0 && drone.Longitude != 0 {
			pos := &types.Position{
				Latitude:  drone.Latitude,
				Longitude: drone.Longitude,
				Altitude:  drone.Altitude,
				Speed:     drone.Speed,
				Heading:   drone.Heading,
				Timestamp: time.Now(),
			}
			p.positions[drone.MAC] = append(p.positions[drone.MAC], pos)
			if err := db.SavePosition(drone.MAC, pos, drone.Standard); err != nil {
				slog.Warn("保存位置数据失败", "mac", drone.MAC, "error", err)
			}
		}

	case "operator_id":
		if opID := getStr("operator_id"); opID != "" && opID != " " {
			drone.OperatorID = opID
		}

	case "system":
		if opLat := getFloat("operator_lat"); opLat != 0 {
			drone.OperatorLatitude = opLat
		}
		if opLon := getFloat("operator_lon"); opLon != 0 {
			drone.OperatorLongitude = opLon
		}
		if opAlt := getFloat("operator_alt"); opAlt != 0 {
			drone.OperatorAltitude = opAlt
		}
		if classification := getStr("classification"); classification != "" {
			drone.Classification = classification
		}
		if areaRadius := getInt("area_radius_m"); areaRadius > 0 {
			drone.AreaRadiusM = areaRadius
		}

	case "gb46750":
		drone.Standard = strings.TrimSpace(msg.Standard)
		drone.Source = strings.TrimSpace(msg.Source)
		if uasID := getStr("unique_id"); uasID != "" {
			drone.UASID = uasID
		}
		if realnameID := getStr("realname_id"); realnameID != "" {
			drone.OperatorID = realnameID
		}
		if uaCat := getStr("ua_category"); uaCat != "" {
			drone.UAType = uaCat
		}
		drone.IDType = "SerialNumber"

		if lat := getFloat("latitude"); lat != 0 {
			drone.Latitude = lat
		}
		if lon := getFloat("longitude"); lon != 0 {
			drone.Longitude = lon
		}

		altBaro := getFloat("altitude_baro")
		altGeo := getFloat("altitude_geo")
		heightM := getFloat("height_m")
		if altBaro != 0 {
			drone.Altitude = altBaro
		} else if altGeo != 0 {
			drone.Altitude = altGeo
		} else if heightM != 0 {
			drone.Altitude = heightM
		}

		if drone.Altitude > 5000 || drone.Altitude < -500 {
			drone.Altitude = 0
		}

		if speed := getFloat("speed_h"); speed != 0 {
			drone.Speed = speed
		}
		if speedV := getFloat("speed_v"); speedV != 0 {
			drone.SpeedVertical = speedV
		}
		if heading := getFloat("direction"); heading != 0 {
			drone.Heading = heading
		}
		if status := getStr("status"); status != "" {
			drone.FlightStatus = status
		}
		if ht := getStr("height_type"); ht != "" {
			drone.HeightType = ht
		}
		if ha := getStr("h_accuracy"); ha != "" {
			drone.HAccuracy = ha
		}
		if va := getStr("v_accuracy"); va != "" {
			drone.VAccuracy = va
		}
		if sa := getStr("s_accuracy"); sa != "" {
			drone.SAccuracy = sa
		}
		if ts := getStr("timestamp"); ts != "" {
			drone.LocationTimestamp = ts
		}

		if rcsLat := getFloat("rcs_latitude"); rcsLat != 0 {
			drone.OperatorLatitude = rcsLat
		}
		if rcsLon := getFloat("rcs_longitude"); rcsLon != 0 {
			drone.OperatorLongitude = rcsLon
		}
		if rcsAlt := getFloat("rcs_altitude"); rcsAlt != 0 {
			drone.OperatorAltitude = rcsAlt
		}
		if opCat := getStr("operation_category"); opCat != "" {
			drone.Classification = "GB46750-Cat" + opCat
		}

		if drone.Latitude != 0 && drone.Longitude != 0 {
			pos := &types.Position{
				Latitude:  drone.Latitude,
				Longitude: drone.Longitude,
				Altitude:  drone.Altitude,
				Speed:     drone.Speed,
				Heading:   drone.Heading,
				Timestamp: time.Now(),
			}
			p.positions[drone.MAC] = append(p.positions[drone.MAC], pos)
			db.SavePosition(drone.MAC, pos, drone.Standard)
		}
	}
}
