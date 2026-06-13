// internal/api/ws.go
package api

import (
	"encoding/json"
	"log/slog"

	"remoteid-monitor/pkg/types"
)

// broadcastDroneUpdate 广播无人机更新
func (s *Server) broadcastDroneUpdate(drone *types.DroneData) {
	message := types.WSMessage{
		Type: "drone_update",
		Data: drone,
		MAC:  drone.MAC,
	}

	data, err := json.Marshal(message)
	if err != nil {
		slog.Warn("序列化无人机更新失败", "error", err)
		return
	}

	s.wsManager.Broadcast(data)
}

// broadcastAlert 广播警报
func (s *Server) broadcastAlert(alert *types.Alert) {
	message := types.WSMessage{
		Type: "alert",
		Data: alert,
	}

	data, err := json.Marshal(message)
	if err != nil {
		slog.Warn("序列化警报失败", "error", err)
		return
	}

	s.wsManager.Broadcast(data)
}

// broadcastSystemInfo 广播系统信息
func (s *Server) broadcastSystemInfo(info map[string]interface{}) {
	message := types.WSMessage{
		Type: "system_info",
		Data: info,
	}

	data, err := json.Marshal(message)
	if err != nil {
		slog.Warn("序列化系统信息失败", "error", err)
		return
	}

	s.wsManager.Broadcast(data)
}
