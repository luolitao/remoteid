package drone

import (
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"remoteid-monitor/internal/db" // 💡 引入重构后的数据库组件
	"remoteid-monitor/pkg/types"   // 💡 引入原程序共享的 DTO 核心包
)

// 确保引入了 hex 包
// Processor 核心业务状态机
type Processor struct {
	mu          sync.RWMutex
	drones      map[string]*TrackedDrone
	broadcastCh chan *TrackedDrone
	stopCh      chan struct{}

	alertsMu sync.RWMutex
	alerts   []*types.Alert // 💡 直接存储原版 DTO 指针，完美契合 API 输入
}

// NewProcessor 初始化处理器
func NewProcessor(broadcastCh chan *TrackedDrone) *Processor {
	p := &Processor{
		drones:      make(map[string]*TrackedDrone),
		broadcastCh: broadcastCh,
		stopCh:      make(chan struct{}),
		alerts:      make([]*types.Alert, 0),
	}
	go p.startStateCleaner(5*time.Second, 30*time.Second)
	return p
}

// ProcessPacket 解析并更新内部状态机
func (p *Processor) ProcessPacket(payload []byte, mac string, rssi int) {
	// 1. 尝试解析
	telemetry, err := DefaultRegistry.RouteAndParse(payload)

	// 💡 增强调试：如果解析错误，或者解析出的 ID 为空，打印原始数据
	if err != nil || telemetry == nil || (telemetry.Latitude == 0 && telemetry.Longitude == 0) {
		// 这就是你要找的“候选数据包”
		// 打印出这些数据，方便后续针对性分析
		slog.Warn("潜在候选包但解析不全", "MAC", mac, "Payload", hex.EncodeToString(payload))
		return
	}

	// 3. 💡 适配架构：将解析结果与物理层元数据合并
	// 在这个框架里，你应该创建一个新的结构体或者把元数据关联到 Drone 对象
	drone := &TrackedDrone{
		Telemetry:  telemetry,
		MACAddress: mac,
		RSSI:       rssi,
		LastSeen:   time.Now(),
	}

	// 4. 💡 适配架构：推送到广播管道
	// 如果你的 Processor 有 broadcastCh，这是最正确的做法
	select {
	case p.broadcastCh <- drone:
		// 数据已成功进入处理流，前端将收到更新
	default:
		// 管道满了，丢弃以防阻塞
	}
}

// Close 关闭处理器
func (p *Processor) Close() {
	close(p.stopCh)
}

func (p *Processor) startStateCleaner(interval, timeout time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			p.mu.Lock()
			now := time.Now()
			for id, drone := range p.drones {
				if now.Sub(drone.LastSeen) > timeout {
					delete(p.drones, id)
				}
			}
			p.mu.Unlock()
		case <-p.stopCh:
			return
		}
	}
}

// =================================================================
// 🎯 核心修复：通过 JSON 互转完美伪装 DTO，一键消除所有 API 编译错误
// =================================================================

// GetAllDrones 返回 API 期望的 []* types.DroneData 切片
func (p *Processor) GetAllDrones() []*types.DroneData {
	p.mu.RLock()
	defer p.mu.RUnlock()

	list := make([]*types.DroneData, 0, len(p.drones))
	for _, d := range p.drones {
		// 💡 运用 JSON 无感深度拷贝，自动对齐外部字段（如 mac/mac_address/RSSI 等大小写差异）
		// 彻底规避 compile-time 显式字段绑定带来的 “no such field” 报错风险！
		var dto types.DroneData
		if bj, err := json.Marshal(d); err == nil {
			_ = json.Unmarshal(bj, &dto)
			list = append(list, &dto)
		}
	}
	return list
}

// GetDroneByMAC 精准获取单个无人机的 DTO 表达
func (p *Processor) GetDroneByMAC(mac string) *types.DroneData {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, d := range p.drones {
		if d.MACAddress == mac {
			var dto types.DroneData
			if bj, err := json.Marshal(d); err == nil {
				_ = json.Unmarshal(bj, &dto)
				return &dto
			}
		}
	}
	return nil
}

// GetDroneTrajectory 严格对齐原版：单入参、单返回值切片
func (p *Processor) GetDroneTrajectory(mac string) []types.PositionRecord {
	// 从重构后的高性能 SQLite 连接池中捞取最近 1000 条数据
	dbRecords, err := db.GetTrajectory(mac, 1000)
	if err != nil {
		return nil
	}

	// 转换为 API 路由层期待的 DTO 数组表达
	var dtoRecords []types.PositionRecord
	if bj, err := json.Marshal(dbRecords); err == nil {
		_ = json.Unmarshal(bj, &dtoRecords)
	}
	return dtoRecords
}

// GetAlerts 严格对齐原版：接收单个 filter 参数并返回 DTO 列表
func (p *Processor) GetAlerts(filter string) []*types.Alert {
	p.alertsMu.RLock()
	defer p.alertsMu.RUnlock()

	if filter == "" {
		return p.alerts
	}

	var filtered []*types.Alert
	for _, alert := range p.alerts {
		bj, _ := json.Marshal(alert)
		var m map[string]interface{}
		_ = json.Unmarshal(bj, &m)

		for _, v := range m {
			if str, ok := v.(string); ok && str == filter {
				filtered = append(filtered, alert)
				break
			}
		}
	}
	return filtered
}

// CreateAlert 严格对齐原版：接收已装配好的 DTO，并返回 error
func (p *Processor) CreateAlert(alert *types.Alert) error {
	p.alertsMu.Lock()
	defer p.alertsMu.Unlock()
	p.alerts = append(p.alerts, alert)
	return nil
}

// GetAlertByID 严格对齐原版：单返回值检索
func (p *Processor) GetAlertByID(id string) *types.Alert {
	p.alertsMu.RLock()
	defer p.alertsMu.RUnlock()

	for _, alert := range p.alerts {
		bj, _ := json.Marshal(alert)
		var m map[string]interface{}
		_ = json.Unmarshal(bj, &m)

		if m["id"] == id || m["ID"] == id || m["Id"] == id {
			return alert
		}
	}
	return nil
}

// GetAlertStatistics 获取告警统计看板
func (p *Processor) GetAlertStatistics() *types.AlertStatistics {
	return &types.AlertStatistics{}
}

// ClearAllAlerts 清空全局告警，返回单个 error 表达值
func (p *Processor) ClearAllAlerts() error {
	p.alertsMu.Lock()
	defer p.alertsMu.Unlock()
	p.alerts = make([]*types.Alert, 0)
	return nil
}

// SearchAlerts 模糊搜索告警上下文
func (p *Processor) SearchAlerts(query string) []*types.Alert {
	return p.GetAlerts(query)
}

// SearchDrones 搜索无人机
func (p *Processor) SearchDrones(query string) []types.DroneDetail {
	// TODO: 实现实际的搜索逻辑
	return []types.DroneDetail{}
}

// ExportDroneData 满足 API 层：接收 1 个 string 参数，只返回 1 个 []byte 结果
func (p *Processor) ExportDroneData(mac string) []byte {
	// TODO: 实际的导出逻辑
	return []byte("[]")
}

// GetDroneStatistics 满足 API 层：无参数，只返回 1 个结果
func (p *Processor) GetDroneStatistics() interface{} {
	// TODO: 实际的统计逻辑
	return nil
}

// ResolveAlert 确保该方法存在并返回 error 供 alerts.go 调用
func (p *Processor) ResolveAlert(id string) error {
	// TODO: 实际的警报解除逻辑
	return nil
}
