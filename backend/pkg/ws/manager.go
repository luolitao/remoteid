package ws

import (
	"log/slog"
	"sync"
)

// Manager 管理所有的 WebSocket 客户端
type Manager struct {
	// 💡 优化：使用 struct{} 作为 value，节省内存
	clients map[*Client]struct{}
	mutex   sync.RWMutex
}

func NewManager() *Manager {
	return &Manager{
		clients: make(map[*Client]struct{}),
	}
}

// Register 注册客户端
func (m *Manager) Register(client *Client) {
	m.mutex.Lock() // ✅ 修复 1：写操作必须使用写锁 (Lock)
	defer m.mutex.Unlock()

	m.clients[client] = struct{}{}
	slog.Debug("WebSocket 客户端注册", "ip", client.IP, "total", len(m.clients))
}

// Unregister 注销客户端
func (m *Manager) Unregister(client *Client) {
	m.mutex.Lock() // ✅ 修复 2：写操作必须使用写锁 (Lock)
	defer m.mutex.Unlock()

	if _, ok := m.clients[client]; ok {
		delete(m.clients, client)
		slog.Debug("WebSocket 客户端注销", "ip", client.IP, "total", len(m.clients))
	}
}

// Broadcast 广播消息给所有客户端
func (m *Manager) Broadcast(message []byte) {
	m.mutex.RLock() // ✅ 读操作使用读锁 (RLock)
	defer m.mutex.RUnlock()

	for client := range m.clients {
		select {
		case client.Send <- message:
			// 发送成功
		default:
			// ✅ 修复 3：客户端 channel 满了，绝不能在读锁下 delete map！
			slog.Warn("客户端消费过慢，丢弃消息并异步踢出", "ip", client.IP)

			// 异步踢出慢客户端，避免死锁和遍历冲突
			go func(c *Client) {
				m.Unregister(c)
				// 注意：不要在这里 close(c.Send)，让 client 自己的 writePump 处理关闭
			}(client)
		}
	}
}

// GetConnectionCount 获取当前连接数
func (m *Manager) GetConnectionCount() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return len(m.clients)
}

// 💡 修复 4：删除了无用的 Start() 方法和相关的 channel。
// 既然外部直接调用 Register 和 Broadcast，就不需要 Hub 模式的 channel 了。
