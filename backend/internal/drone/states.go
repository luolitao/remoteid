package drone

import "time"

// UnpackedTelemetry 是各个协议解包后的归一化标准遥测数据
// drone/types.go 或同文件顶部
type UnpackedTelemetry struct {
	Protocol string `json:"protocol"`
	UASID    string `json:"uas_id"`

	// 位置信息
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Altitude  float64 `json:"altitude"`
	Height    float64 `json:"height"`

	// 运动状态
	Speed   float64 `json:"speed"`
	Heading float64 `json:"heading"`

	// 操作员信息（模拟器可能发送）
	OperatorID  string  `json:"operator_id"`
	OperatorLat float64 `json:"operator_lat"`
	OperatorLng float64 `json:"operator_lng"`

	// 时间戳（秒级，从午夜开始）
	Timestamp  uint32 `json:"timestamp"`
	TakeoffLat float64
	TakeoffLng float64
	Emergency  bool
}

// TrackedDrone 封装了归一化的遥测数据以及嗅探到的无线电物理元数据
type TrackedDrone struct {
	Telemetry   *UnpackedTelemetry `json:"telemetry"`
	LastSeen    time.Time          `json:"last_seen"`
	MACAddress  string             `json:"mac_address,omitempty"` // 发射端网卡物理 MAC
	RSSI        int                `json:"rssi,omitempty"`        // 信号强度 (dBm)
	LastDBWrite time.Time          `json:"-"`                     // 记录最后一次落盘时间
}
