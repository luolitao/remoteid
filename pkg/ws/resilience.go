// pkg/ws/resilience.go
package ws

import "time"

type ResilientClient struct {
	*Client
	reconnectAttempts int
	lastDisconnect    time.Time
}

func (rc *ResilientClient) ShouldReconnect() bool {
	// 指数退避重连
	backoff := time.Duration(1<<rc.reconnectAttempts) * time.Second
	return time.Since(rc.lastDisconnect) > backoff
}
