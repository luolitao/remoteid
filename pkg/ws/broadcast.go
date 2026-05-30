// pkg/ws/broadcast.go
package ws

import (
	"encoding/json"
	"log/slog"
	"remoteid/pkg/types"
	"time"
)

// BroadcastMessage 广播已序列化的消息
func (m *Manager) BroadcastMessage(msg interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		slog.Warn("消息序列化失败", "error", err)
		return err
	}
	return m.Broadcast(data)
}

// BroadcastError 广播错误消息
func (m *Manager) BroadcastError(errMsg string) error {
	return m.BroadcastMessage(types.WSMessage{
		Type: "error",
		Data: map[string]string{
			"message": errMsg,
			"time":    time.Now().Format(time.RFC3339),
		},
	})
}

// BroadcastDroneUpdate 广播无人机更新
func (m *Manager) BroadcastDroneUpdate(drone *types.DroneData) error {
	return m.BroadcastMessage(types.WSMessage{
		Type: "drone_update",
		Data: drone,
		MAC:  drone.MAC,
	})
}

// BroadcastAlert 广播警报
func (m *Manager) BroadcastAlert(alert *types.Alert) error {
	return m.BroadcastMessage(types.WSMessage{
		Type: "alert",
		Data: alert,
		MAC:  alert.TargetMAC,
	})
}
