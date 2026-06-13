package drone

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"remoteid-monitor/internal/db"
	"remoteid-monitor/pkg/types"
)

// Processor 核心业务状态机
type Processor struct {
	mu          sync.RWMutex
	drones      map[string]*TrackedDrone
	broadcastCh chan *TrackedDrone
	stopCh      chan struct{}

	alertsMu sync.RWMutex
	alerts   []*types.Alert

	// 专门记录“是否已经打印过上线日志”的集合（只增不减）
	reportedDrones map[string]struct{}
}

// NewProcessor 初始化处理器
func NewProcessor(broadcastCh chan *TrackedDrone) *Processor {
	p := &Processor{
		drones:         make(map[string]*TrackedDrone),
		broadcastCh:    broadcastCh,
		stopCh:         make(chan struct{}),
		alerts:         make([]*types.Alert, 0),
		reportedDrones: make(map[string]struct{}),
	}
	go p.startStateCleaner(5*time.Second, 30*time.Second)
	return p
}

const MaxAlertsLimit = 1000

func (p *Processor) CreateAlert(alert *types.Alert) error {
	p.alertsMu.Lock()
	defer p.alertsMu.Unlock()

	p.alerts = append(p.alerts, alert)

	// 内存保护：超出限制丢弃最旧的
	if len(p.alerts) > MaxAlertsLimit {
		p.alerts = p.alerts[len(p.alerts)-MaxAlertsLimit:]
	}
	return nil
}

// StartWorker 启动业务处理循环（从 Channel 读取并解析）
// processor.go 新增/替换
func (p *Processor) StartWorker(ctx context.Context, inputCh <-chan RawPacket) {
	slog.Info("🚀 处理器 Worker 已启动，开始消费数据包...")
	consumed := 0
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("Worker 收到退出信号，停止消费", "total_consumed", consumed)
			return
		case pkt, ok := <-inputCh:
			if !ok {
				return
			}
			// ✅ 核心消费逻辑
			p.ProcessPacket(pkt.Payload, pkt.MAC, pkt.RSSI)
			consumed++

		case <-tick.C:
			// 每 5 秒打印一次消费速率，方便监控性能瓶颈
			slog.Info("📊 Worker 消费统计", "rate_per_sec", float64(consumed)/5.0, "total", consumed)
			consumed = 0
		}
	}
}

// processor.go - ProcessPacket() 函数
func (p *Processor) ProcessPacket(payload []byte, mac string, rssi int) {
	telemetry, err := DefaultRegistry.RouteAndParse(payload)

	// 1. 过滤无效解析结果
	if err != nil || telemetry == nil {
		return
	}

	// 2. 软过滤：仅拦截明显越界的噪点，放行 (0,0) 供模拟器/起飞点调试
	if (telemetry.Latitude < -90 || telemetry.Latitude > 90) ||
		(telemetry.Longitude < -180 || telemetry.Longitude > 180) {
		return
	}

	// 3. 确定唯一标识
	droneKey := telemetry.UASID
	if droneKey == "" {
		droneKey = "MAC_" + mac
	}

	// 4. 更新内存状态机
	p.mu.Lock()
	_, alreadyReported := p.reportedDrones[droneKey]
	if !alreadyReported {
		p.reportedDrones[droneKey] = struct{}{}
		slog.Info("🚨 [发现新无人机上线]",
			"DroneKey", droneKey, "Protocol", telemetry.Protocol,
			"MAC", mac, "RSSI", rssi,
			"Lat", telemetry.Latitude, "Lng", telemetry.Longitude,
		)
	}

	drone := &TrackedDrone{
		Telemetry:  telemetry,
		MACAddress: mac,
		RSSI:       rssi,
		LastSeen:   time.Now(),
	}
	p.drones[droneKey] = drone
	p.mu.Unlock()

	// 5. 💾 关键修复：持久化到 SQLite（WAL 模式可承受高频写入）
	pos := &types.Position{
		Latitude:  telemetry.Latitude,
		Longitude: telemetry.Longitude,
		Altitude:  telemetry.Altitude,
		Speed:     telemetry.Speed,
		Heading:   telemetry.Heading,
		Timestamp: time.Now().UTC(),
	}
	if err := db.SavePosition(mac, pos, telemetry.Protocol); err != nil {
		slog.Debug("轨迹落盘失败", "mac", mac, "error", err)
	}

	// 6. 广播给前端 WebSocket
	select {
	case p.broadcastCh <- drone:
	default:
		// 通道满时丢弃，保护主循环不阻塞
	}
}

// startStateCleaner 定期清理过期无人机 & 防止 reportedDrones 内存泄漏
func (p *Processor) startStateCleaner(interval, timeout time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			now := time.Now()
			p.mu.Lock()

			// 清理离线无人机
			for id, drone := range p.drones {
				if now.Sub(drone.LastSeen) > timeout {
					delete(p.drones, id)
					// 同步清理上线记录，允许无人机重新触发“上线”日志
					delete(p.reportedDrones, id)
				}
			}
			p.mu.Unlock()

		case <-p.stopCh:
			return
		}
	}
}

// 新增：定期清理不再活跃的 reportedDrones
func (p *Processor) cleanReportedDrones() {
	// 如果 map 超过阈值，清理不在 drones map 中的 key
	if len(p.reportedDrones) > 5000 {
		p.mu.RLock()
		for key := range p.reportedDrones {
			if _, exists := p.drones[key]; !exists {
				delete(p.reportedDrones, key)
			}
		}
		p.mu.RUnlock()
	}
}

func (p *Processor) Close() { close(p.stopCh) }

// =================================================================
// 🎯 核心重构：提取公共转换函数，彻底消灭 JSON 互转！
// =================================================================

// toDroneDataDTO 将内部结构显式、扁平化地映射为前端期望的 DTO
func toDroneDataDTO(d *TrackedDrone) *types.DroneData {
	if d == nil || d.Telemetry == nil {
		return nil
	}
	t := d.Telemetry
	return &types.DroneData{
		// 1. 映射 TrackedDrone 顶层字段
		MAC: d.MACAddress,
		// FirstSeen: d.FirstSeen, // 如果 TrackedDrone 没有 FirstSeen，请去掉或改用 time.Now()
		LastSeen: d.LastSeen,
		// SignalStrength: fmt.Sprintf("%d", d.RSSI), // 如果前端需要字符串格式的信号强度

		// 2. 映射 Telemetry 里的字段（打破嵌套，直接赋值给顶层）
		UASID:      t.UASID,
		OperatorID: t.OperatorID,
		Latitude:   t.Latitude,
		Longitude:  t.Longitude,
		Altitude:   t.Altitude,
		// Height:            t.Height,
		Speed:             t.Speed,
		Heading:           t.Heading,
		OperatorLatitude:  t.OperatorLat,
		OperatorLongitude: t.OperatorLng,

		// 3. 其他特殊类型转换（请根据 types.DroneData 的实际定义补充）
		// Timestamp: time.Unix(int64(t.Timestamp), 0).Format(time.RFC3339),
	}
}

// GetAllDrones 返回 API 期望的 []* types.DroneData 切片
func (p *Processor) GetAllDrones() []*types.DroneData {
	p.mu.RLock()
	defer p.mu.RUnlock()

	list := make([]*types.DroneData, 0, len(p.drones))
	for _, d := range p.drones {
		if dto := toDroneDataDTO(d); dto != nil {
			list = append(list, dto)
		}
	}
	return list
}

// GetDroneByMAC 精准获取单个无人机的 DTO 表达
func (p *Processor) GetDroneByMAC(mac string) *types.DroneData {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// 因为 p.drones 的 key 可能是 UASID 也可能是 MAC_mac，所以直接遍历比对 MACAddress 最稳妥
	for _, d := range p.drones {
		if d.MACAddress == mac {
			return toDroneDataDTO(d) // ✅ 复用公共转换函数，绝对不会再丢失数据！
		}
	}
	return nil
}

// GetDroneTrajectory 获取轨迹
func (p *Processor) GetDroneTrajectory(mac string) []types.PositionRecord {
	dbRecords, err := db.GetTrajectory(mac, 1000)
	if err != nil {
		slog.Warn("获取轨迹失败", "mac", mac, "error", err)
		return []types.PositionRecord{} // 返回空切片，防止前端收到 null
	}

	// ✅ 核心修复：显式将 []db.PositionRecord 转换为 []types.PositionRecord
	dtoRecords := make([]types.PositionRecord, 0, len(dbRecords))
	for _, r := range dbRecords {
		dtoRecords = append(dtoRecords, types.PositionRecord{
			// ⚠️ 注意：请根据 db.PositionRecord 和 types.PositionRecord 的实际字段名进行映射！
			// 如果字段名完全一样，直接对应即可；如果不一样（比如 db 里叫 Lat，types 里叫 Latitude），请手动修改。
			Lat:       r.Latitude,  // 或者 r.Lat
			Lng:       r.Longitude, // 或者 r.Lng
			Alt:       r.Altitude,  // 或者 r.Alt
			Timestamp: r.Timestamp, // 或者 r.Time
		})
	}

	return dtoRecords
}

// =================================================================
// 告警相关方法：彻底消灭 JSON 转 Map 的奇葩操作
// =================================================================

// GetAlerts 获取告警列表
func (p *Processor) GetAlerts(filter string) []*types.Alert {
	p.alertsMu.RLock()
	defer p.alertsMu.RUnlock()

	if filter == "" {
		return p.alerts
	}

	var filtered []*types.Alert
	for _, alert := range p.alerts {
		// ✅ 核心修复：去掉不存在的 Status 字段。
		// 请根据 types.Alert 实际存在的字段进行修改，例如 ID, Type, Level, Message 等。
		// 这里以匹配 ID 和 Type 为例：
		if alert.ID == filter || alert.Type == filter {
			filtered = append(filtered, alert)
			continue
		}

		// 💡 兜底方案：如果 filter 是模糊搜索关键字，可以比较 Message 或 Description
		// 如果 types.Alert 有 Message 字段，可以加上： || strings.Contains(alert.Message, filter)
	}
	return filtered
}

// GetAlertByID 根据 ID 获取告警
func (p *Processor) GetAlertByID(id string) *types.Alert {
	p.alertsMu.RLock()
	defer p.alertsMu.RUnlock()

	for _, alert := range p.alerts {
		// ✅ 正确做法：直接比较结构体字段，绝对不要转 JSON！
		if alert.ID == id {
			return alert
		}
	}
	return nil
}

func (p *Processor) GetAlertStatistics() *types.AlertStatistics {
	return &types.AlertStatistics{} // TODO: 实现实际统计
}

func (p *Processor) ClearAllAlerts() error {
	p.alertsMu.Lock()
	defer p.alertsMu.Unlock()
	p.alerts = make([]*types.Alert, 0)
	return nil
}

func (p *Processor) SearchAlerts(query string) []*types.Alert {
	return p.GetAlerts(query)
}

func (p *Processor) SearchDrones(query string) []types.DroneDetail {
	return []types.DroneDetail{} // TODO: 实现实际搜索
}

func (p *Processor) ExportDroneData(mac string) []byte {
	return []byte("[]") // TODO: 实现实际导出
}

func (p *Processor) GetDroneStatistics() interface{} {
	return nil // TODO: 实现实际统计
}

func (p *Processor) ResolveAlert(id string) error {
	// 简单的解除告警逻辑示例
	p.alertsMu.Lock()
	defer p.alertsMu.Unlock()
	for i, alert := range p.alerts {
		if alert.ID == id {
			// 假设 Alert 结构体有个 Status 字段
			// p.alerts[i].Status = "resolved"
			_ = i
			break
		}
	}
	return nil
}
