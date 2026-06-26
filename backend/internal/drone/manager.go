package drone

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"remoteid-monitor/pkg/types"
	"remoteid-monitor/pkg/ws"
)

type Manager struct {
	parser    *RemoteIDParser
	processor *Processor
	sniffer   *Sniffer
	iface     string

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu             sync.RWMutex
	lastPacketTime time.Time
	stats          struct {
		totalPackets   int
		parseErrors    int
		dronesDetected int
		lastDroppedHex string
		lastParsedHex  string
	}
}

func NewManager(wsManager *ws.Manager, wifiIface string) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	m := &Manager{
		parser:    NewParser(),
		processor: NewProcessor(wsManager),
		ctx:       ctx,
		cancel:    cancel,
		iface:     wifiIface,
	}
	return m
}

func (m *Manager) Start() error {
	// ... 后续创建 Sniffer
	slog.Info("RemoteID 监控管理器启动...")
	packetChan := make(chan RawPacket, 1000)

	// 创建 Sniffer 并传入 packetCh
	m.sniffer = NewSniffer(m.iface, packetChan)

	// 启动 Sniffer
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		if err := m.sniffer.Start(m.ctx); err != nil {
			slog.Error("Sniffer 启动失败", "error", err)
		}
	}()

	// 启动处理协程
	m.wg.Add(1)
	go m.processLoop(packetChan)

	return nil
}

// processLoop 消费 RawPacket，解析并交给 processor
func (m *Manager) processLoop(packetChan <-chan RawPacket) {
	defer m.wg.Done()

	for {
		select {
		case <-m.ctx.Done():
			slog.Info("处理循环停止")
			return
		case pkt := <-packetChan:
			m.mu.Lock()
			m.lastPacketTime = pkt.Timestamp
			m.stats.totalPackets++
			m.mu.Unlock()

			rawHex := hex.EncodeToString(pkt.Payload)
			if len(rawHex) > 128 {
				rawHex = rawHex[:128] + "..."
			}

			// 解析
			messages, err := m.parser.ParseFrame(pkt.Payload)
			if err != nil || len(messages) == 0 {
				m.mu.Lock()
				m.stats.parseErrors++
				m.stats.lastDroppedHex = rawHex
				m.mu.Unlock()
				slog.Debug("ParseFrame result", "count", len(messages), "err", err)
				continue
			}

			m.mu.Lock()
			m.stats.dronesDetected++
			m.stats.lastParsedHex = rawHex
			m.mu.Unlock()

			// 补充元数据
			for i := range messages {
				messages[i].Data["signal_dbm"] = fmt.Sprintf("%d", pkt.RSSI)
				messages[i].Data["transport"] = "wifi"
			}

			// 交给 processor 处理（使用 MAC 地址作为源标识）
			m.processor.ProcessDroneData(pkt.MAC, messages)
		}
	}
}

func (m *Manager) Stop() {
	slog.Info("正在停止 RemoteID 监控管理器...")
	m.cancel()

	if m.sniffer != nil {
		m.sniffer.Stop()
	}

	m.wg.Wait()
	slog.Info("RemoteID 监控管理器已安全停止")
}

// GetProcessor 暴露 processor 供 API 使用
func (m *Manager) GetProcessor() *Processor {
	return m.processor
}

// GetLastPacketTime 实现健康检查
func (m *Manager) GetLastPacketTime() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastPacketTime
}

// GetStats 获取统计信息（可选）
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return map[string]interface{}{
		"total_packets":   m.stats.totalPackets,
		"parse_errors":    m.stats.parseErrors,
		"drones_detected": m.stats.dronesDetected,
		"last_packet":     m.lastPacketTime,
	}
}

// GetCaptureStats 实现 StatusProvider 接口
func (m *Manager) GetCaptureStats() types.CaptureStats {
	var stats types.CaptureStats

	if m.sniffer != nil {
		snifferStats := m.sniffer.GetStats()
		stats.TotalPackets = int(snifferStats["total_packets"])
		stats.BeaconFrames = int(snifferStats["beacon_frames"])
		stats.ActionFrames = int(snifferStats["action_frames"])
		stats.DronesDetected = int(snifferStats["drones_detected"])
	}

	m.mu.RLock()
	stats.ParseErrors = m.stats.parseErrors
	stats.LastParsedHex = m.stats.lastParsedHex
	stats.LastDroppedHex = m.stats.lastDroppedHex
	stats.LastPacketTime = m.lastPacketTime
	m.mu.RUnlock()

	return stats
}
