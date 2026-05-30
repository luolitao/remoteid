package drone

import "time"

// SystemStatus 系统状态
type SystemStatus struct {
	PacketsProcessed int           `json:"packets_processed"`
	ActiveDrones     int           `json:"active_drones"`
	LastUpdate       time.Time     `json:"last_update"`
	MemoryUsage      uint64        `json:"memory_usage"`
	CPUUsage         float64       `json:"cpu_usage"`
	InterfaceStatus  InterfaceStat `json:"interface_status"`
}

// InterfaceStat 网络接口状态
type InterfaceStat struct {
	Name      string  `json:"name"`
	Mode      string  `json:"mode"`
	Channel   int     `json:"channel"`
	Frequency float64 `json:"frequency"`
	SignalDBM int     `json:"signal_dbm"`
	Packets   int     `json:"packets"`
	Dropped   int     `json:"dropped"`
	Errors    int     `json:"errors"`
}
