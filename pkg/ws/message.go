// pkg/ws/message.go
package ws

import (
	"encoding/json"
	"log/slog"
	"time"
)

type WebSocketMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
	Time    time.Time   `json:"time"`
}

func NewMessage(typ string, payload interface{}) []byte {
	msg := WebSocketMessage{
		Type:    typ,
		Payload: payload,
		Time:    time.Now().UTC(),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		slog.Warn("WebSocket 消息序列化失败", "type", typ, "error", err)
		return nil
	}
	return data
}
