package types

import (
	"time"
)

type DroneData struct {
	// 标识
	MAC        string `json:"mac"`
	UASID      string `json:"uas_id"`
	OperatorID string `json:"operator_id"`
	UAType     string `json:"ua_type"` // 新增
	IDType     string `json:"id_type"` // 新增

	// 位置与运动
	Latitude      float64 `json:"latitude"`
	Longitude     float64 `json:"longitude"`
	Altitude      float64 `json:"altitude"`
	Speed         float64 `json:"speed"`
	Heading       float64 `json:"heading"`
	SpeedVertical float64 `json:"speed_v,omitempty"`

	// 状态与精度
	FlightStatus      string `json:"flight_status,omitempty"`
	HeightType        string `json:"height_type,omitempty"`
	HAccuracy         string `json:"h_accuracy,omitempty"`
	VAccuracy         string `json:"v_accuracy,omitempty"`
	SAccuracy         string `json:"s_accuracy,omitempty"`
	LocationTimestamp string `json:"timestamp,omitempty"`

	// 操作员区域
	OperatorLatitude  float64 `json:"operator_latitude,omitempty"`
	OperatorLongitude float64 `json:"operator_longitude,omitempty"`
	OperatorAltitude  float64 `json:"operator_altitude,omitempty"`
	Classification    string  `json:"classification,omitempty"`
	AreaRadiusM       int     `json:"area_radius_m,omitempty"`

	// 元数据
	Standard       string    `json:"standard"`
	Source         string    `json:"source"`
	SignalStrength string    `json:"signal_strength,omitempty"`
	BatteryLevel   string    `json:"battery_level,omitempty"`
	FlightTime     string    `json:"flight_time,omitempty"`
	FirstSeen      time.Time `json:"first_seen"`
	LastSeen       time.Time `json:"last_seen"`

	// 协议特定
	Protocol string       `json:"protocol"`
	GB46750  *GB46750Data `json:"gb46750,omitempty"`
	GB42590  *GB42590Data `json:"gb42590,omitempty"`
	ASTM     *ASTMData    `json:"astm,omitempty"`
}

// DroneMessage 解析器输出的单条原始消息
type DroneMessage struct {
	MessageType string         `json:"message_type"` // 修正字段名
	Standard    string         `json:"standard"`
	Data        map[string]any `json:"data"` // ✅ 优化：改为 any，允许直接存储 float64/int，避免前端二次字符串转换
	Source      string         `json:"source"`
	RawHex      string         `json:"raw_hex,omitempty"` // ✅ 新增：用于打印原始数据包 16 进制
}

// DroneUpdate 用于内部状态更新的轻量级结构
type DroneUpdate struct {
	MAC       string    `json:"mac"`
	Position  Position  `json:"position"`
	Timestamp time.Time `json:"timestamp"`
}

// DroneDetail 数据库存储或详情页使用的完整档案
type DroneDetail struct {
	MAC               string    `json:"mac"`
	UASID             string    `json:"uas_id"`
	Position          Position  `json:"position"`
	LastSeen          time.Time `json:"last_seen"`
	FirstSeen         time.Time `json:"first_seen"`
	TotalMessages     int       `json:"total_messages"`
	SignalStrengthAvg float64   `json:"signal_strength_avg"`
}

// Position 统一的地理位置与运动状态结构体，提高复用性
type Position struct {
	Latitude  float64   `json:"latitude"`
	Longitude float64   `json:"longitude"`
	Altitude  float64   `json:"altitude"`
	Speed     float64   `json:"speed"`
	Heading   float64   `json:"heading"`
	Timestamp time.Time `json:"timestamp"`
}

// GB 46750 特有的数据
type GB46750Data struct {
	Version           string    `json:"version,omitempty"`            // 如 "1.0"
	UniqueID          string    `json:"unique_id,omitempty"`          // 新增
	UACategory        string    `json:"ua_category,omitempty"`        // 微型(0) 等
	RealNameID        string    `json:"realname_id,omitempty"`        // 实名ID
	OperationCategory string    `json:"operation_category,omitempty"` // 操作类别
	RCSLocType        string    `json:"rcs_loc_type,omitempty"`       // 遥控站定位类型
	RCSLocation       *Position `json:"rcs_location,omitempty"`       // 遥控站位置
	RCSAltitude       float64   `json:"rcs_altitude,omitempty"`       // 遥控站高度
	Status            string    `json:"status,omitempty"`             // 飞行状态
	CoordSystem       string    `json:"coord_system,omitempty"`       // 坐标系统
	HAccuracy         string    `json:"h_accuracy,omitempty"`         // 水平精度
	VAccuracy         string    `json:"v_accuracy,omitempty"`         // 垂直精度
	SAccuracy         string    `json:"s_accuracy,omitempty"`         // 速度精度
	TimestampMS       uint64    `json:"timestamp_ms,omitempty"`       // 时间戳(毫秒)
	TSSAccuracy       string    `json:"ts_accuracy,omitempty"`        // 时间戳精度
	// 若还有其他字段，继续添加
}

// ASTM F3411 特有的数据（可按需补充）
type ASTMData struct {
	// 根据实际需要定义
}

// GB 42590 特有的数据
type GB42590Data struct {
	// 根据实际需要定义
}

// Trajectory 轨迹数据
type Trajectory struct {
	MAC     string      `json:"mac"`
	Points  []*Position `json:"points"`
	Count   int         `json:"count"`
	Updated time.Time   `json:"updated"`
}

// Alert 告警记录
type Alert struct {
	ID         string     `json:"id"`
	Message    string     `json:"message"`
	Category   string     `json:"category"`
	Severity   string     `json:"severity"`
	TargetMAC  string     `json:"target_mac"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	Resolved   bool       `json:"resolved"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
	// 1. +++ 添加缺失字段 +++
	Type      string    `json:"type"`      // 警报类型
	Timestamp time.Time `json:"timestamp"` // 时间戳
	MAC       string    `json:"mac"`       // 相关 MAC 地址
}

// ExportData 导出数据结构
type ExportData struct {
	Drone     *DroneData  `json:"drone"`
	Positions []*Position `json:"positions"`
	Exported  time.Time   `json:"exported"`
}

// DroneStatistics 无人机统计
type DroneStatistics struct {
	TotalDrones    int       `json:"total_drones"`
	ActiveDrones   int       `json:"active_drones"`
	InactiveDrones int       `json:"inactive_drones"`
	LastUpdate     time.Time `json:"last_update"`
}

// AlertStatistics 告警统计
type AlertStatistics struct {
	TotalAlerts    int       `json:"total_alerts"`
	ResolvedAlerts int       `json:"resolved_alerts"`
	PendingAlerts  int       `json:"pending_alerts"`
	LastUpdate     time.Time `json:"last_update"`
}

// SystemInfo 系统运行状态
type SystemInfo struct {
	Version      string    `json:"version"`
	BuildTime    time.Time `json:"build_time"`
	GoVersion    string    `json:"go_version"`
	OS           string    `json:"os"`
	Arch         string    `json:"arch"`
	NumGoroutine int       `json:"num_goroutine"`
	NumCPU       int       `json:"num_cpu"`
	Uptime       string    `json:"uptime"`
	Timestamp    time.Time `json:"timestamp"`
}

// WSMessage WebSocket 通信消息
type WSMessage struct {
	Type string `json:"type"`
	Data any    `json:"data"`
	MAC  string `json:"mac,omitempty"`
}

// CaptureStats 抓包层实时统计
type CaptureStats struct {
	TotalPackets   int       `json:"total_packets"`
	BeaconFrames   int       `json:"beacon_frames"`
	ActionFrames   int       `json:"action_frames"`
	DronesDetected int       `json:"drones_detected"`
	NormalDevices  int       `json:"normal_devices"`
	ParseErrors    int       `json:"parse_errors"`
	LastPacketTime time.Time `json:"last_packet_time"`
	// 16进制显示：仅保留前 128 个字符 (64字节)，避免 API 响应过大
	LastParsedHex  string `json:"last_parsed_hex,omitempty"`
	LastDroppedHex string `json:"last_dropped_hex,omitempty"`
}

// ProcessorStats 解析与处理层统计
type ProcessorStats struct {
	TotalDrones   int       `json:"total_drones"`
	TotalMessages int       `json:"total_messages"`
	LastUpdate    time.Time `json:"last_update"`
}
