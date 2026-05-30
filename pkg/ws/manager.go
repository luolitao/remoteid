// pkg/ws/manager.go
package ws

import (
	"log/slog"
	"sync"
)

type Manager struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mutex      sync.RWMutex
}

func NewManager() *Manager {
	return &Manager{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 100),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (m *Manager) Register(client *Client) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.clients[client] = true
	slog.Debug("WebSocket 客户端注册", "ip", client.IP)
}

func (m *Manager) Unregister(client *Client) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if _, ok := m.clients[client]; ok {
		delete(m.clients, client)
		close(client.Send)
		slog.Debug("WebSocket 客户端注销", "ip", client.IP)
	}
}

func (m *Manager) Broadcast(message []byte) error {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	for client := range m.clients {
		select {
		case client.Send <- message:
		default:
			slog.Warn("无法发送消息给客户端", "ip", client.IP)
			close(client.Send)
			delete(m.clients, client)
		}
	}
	return nil
}

// 2. +++ 添加：获取连接数方法 +++
func (m *Manager) GetConnectionCount() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return len(m.clients)
}

func (m *Manager) Start() {
	for {
		select {
		case client := <-m.register:
			m.Register(client)
		case client := <-m.unregister:
			m.Unregister(client)
		case message := <-m.broadcast:
			m.Broadcast(message)
		}
	}
}

func (m *Manager) Stop() {
	for client := range m.clients {
		m.Unregister(client)
	}
}
