// pkg/ws/client.go
package ws

import (
	"log/slog"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
)

type Client struct {
	Conn         *websocket.Conn
	Send         chan []byte
	IP           string
	Created      time.Time
	lastActivity time.Time
}

func NewClient(conn *websocket.Conn, ip string) *Client {
	return &Client{
		Conn:         conn,
		Send:         make(chan []byte, 256),
		IP:           ip,
		Created:      time.Now(),
		lastActivity: time.Now(),
	}
}

func (c *Client) ReadPump(manager *Manager) {
	defer func() {
		manager.Unregister(c)
		c.Conn.Close()
	}()

	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.lastActivity = time.Now()
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, _, err := c.Conn.ReadMessage()
		if err != nil {
		if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
			slog.Warn("WebSocket 读取错误", "error", err)
		}
			break
		}
		c.lastActivity = time.Now()
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// 修正：直接发送 []byte 消息，不进行类型断言
			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
