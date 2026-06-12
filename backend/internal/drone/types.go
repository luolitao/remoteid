package drone

import "time"

// UnpackedTelemetry 是各个协议解包后的归一化标准遥测数据
type UnpackedTelemetry struct {
	UASID     string  `json:"uas_id"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Altitude  float64 `json:"altitude"`
	Height    float64 `json:"height"`
	Heading   float64 `json:"heading"`
	Speed     float64 `json:"speed"`
	Protocol  string  `json:"protocol"`
}

// TrackedDrone 封装了归一化的遥测数据以及嗅探到的无线电物理元数据
type TrackedDrone struct {
	Telemetry  *UnpackedTelemetry `json:"telemetry"`
	LastSeen   time.Time          `json:"last_seen"`
	MACAddress string             `json:"mac_address,omitempty"` // 发射端网卡物理 MAC
	RSSI       int                `json:"rssi,omitempty"`        // 信号强度 (dBm)
}
