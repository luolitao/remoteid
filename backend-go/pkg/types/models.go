package types

import (
	"time"
)

type DroneData struct {
	MAC               string    `json:"mac"`
	UASID             string    `json:"uas_id"`
	UAType            string    `json:"ua_type"`
	IDType            string    `json:"id_type"`
	Standard          string    `json:"standard"`
	Source            string    `json:"source"`
	Latitude          float64   `json:"latitude"`
	Longitude         float64   `json:"longitude"`
	Altitude          float64   `json:"altitude"`
	Speed             float64   `json:"speed"`
	Heading           float64   `json:"heading"`
	SpeedVertical     float64   `json:"speed_v"`
	HeightType        string    `json:"height_type"`
	FlightStatus      string    `json:"flight_status"`
	HAccuracy         string    `json:"h_accuracy"`
	VAccuracy         string    `json:"v_accuracy"`
	SAccuracy         string    `json:"s_accuracy"`
	LocationTimestamp string    `json:"timestamp"`
	OperatorLatitude  float64   `json:"operator_latitude"`
	OperatorLongitude float64   `json:"operator_longitude"`
	OperatorAltitude  float64   `json:"operator_altitude"`
	AreaRadiusM       int       `json:"area_radius_m"`
	Classification    string    `json:"classification_region"`
	ChinaCompliant    bool      `json:"china_compliant"`
	SignalStrength    string    `json:"signal_strength"`
	BatteryLevel      string    `json:"battery_level"`
	FlightTime        string    `json:"flight_time"`
	FirstSeen         time.Time `json:"first_seen"`
	LastSeen          time.Time `json:"last_seen"`
}

type Position struct {
	Latitude  float64   `json:"latitude"`
	Longitude float64   `json:"longitude"`
	Altitude  float64   `json:"altitude"`
	Speed     float64   `json:"speed"`
	Heading   float64   `json:"heading"`
	Timestamp time.Time `json:"timestamp"`
}

type Trajectory struct {
	MAC     string      `json:"mac"`
	Points  []*Position `json:"points"`
	Count   int         `json:"count"`
	Updated time.Time   `json:"updated"`
}

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

type ExportData struct {
	Drone     *DroneData  `json:"drone"`
	Positions []*Position `json:"positions"`
	Exported  time.Time   `json:"exported"`
}

type DroneStatistics struct {
	TotalDrones    int       `json:"total_drones"`
	ActiveDrones   int       `json:"active_drones"`
	InactiveDrones int       `json:"inactive_drones"`
	LastUpdate     time.Time `json:"last_update"`
}

type AlertStatistics struct {
	TotalAlerts    int       `json:"total_alerts"`
	ResolvedAlerts int       `json:"resolved_alerts"`
	PendingAlerts  int       `json:"pending_alerts"`
	LastUpdate     time.Time `json:"last_update"`
}

// 2. +++ 添加缺失的类型 +++
type DroneMessage struct {
	MessageType string            `json:"message_type"` // 修正字段名
	Standard    string            `json:"standard"`
	Data        map[string]string `json:"data"` // 修正类型
	Source      string            `json:"source"`
}

// 3. +++ 添加数据库相关类型 +++
type DroneUpdate struct {
	MAC       string    `json:"mac"`
	Position  Position  `json:"position"`
	Timestamp time.Time `json:"timestamp"`
}

type DroneDetail struct {
	MAC               string    `json:"mac"`
	UASID             string    `json:"uas_id"`
	Position          Position  `json:"position"`
	LastSeen          time.Time `json:"last_seen"`
	FirstSeen         time.Time `json:"first_seen"`
	TotalMessages     int       `json:"total_messages"`
	SignalStrengthAvg float64   `json:"signal_strength_avg"`
}

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

type WSMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
	MAC  string      `json:"mac,omitempty"`
}
