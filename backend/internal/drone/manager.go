package drone

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"remoteid-monitor/pkg/capture"
	"remoteid-monitor/pkg/types"
	"remoteid-monitor/pkg/ws"
)

type Manager struct {
	parser    *RemoteIDParser
	processor *Processor
	capturers []capture.Capturer

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu             sync.RWMutex
	lastPacketTime time.Time
	aggregateStats types.CaptureStats // 聚合所有捕获器的统计
}

func NewManager(wsManager *ws.Manager, wifiIface string) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	m := &Manager{
		parser:    NewParser(),
		processor: NewProcessor(wsManager),
		ctx:       ctx,
		cancel:    cancel,
	}

	// 注册 WiFi 捕获器 (未来可轻松追加 BLECapturer 等)
	m.capturers = append(m.capturers, capture.NewWiFiCapturer(wifiIface))

	return m
}

func (m *Manager) Start() error {
	slog.Info("RemoteID 监控管理器启动...")
	packetChan := make(chan capture.RawPacket, 1000)

	// 启动所有捕获器
	for _, cap := range m.capturers {
		c := cap
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			if err := c.Start(m.ctx, packetChan); err != nil {
				slog.Error("捕获器启动失败", "name", c.Name(), "error", err)
			}
		}()
	}

	// 启动统一的处理协程
	m.wg.Add(1)
	go m.processLoop(packetChan)

	return nil
}

// processLoop 核心数据流：RawPacket -> Parser -> Processor
func (m *Manager) processLoop(packetChan <-chan capture.RawPacket) {
	defer m.wg.Done()

	for {
		select {
		case <-m.ctx.Done():
			return
		case pkt := <-packetChan:
			m.mu.Lock()
			m.lastPacketTime = pkt.Timestamp
			m.mu.Unlock()

			// 1. 提取前 64 字节 (128个 hex 字符) 用于调试显示
			rawHex := hex.EncodeToString(pkt.Payload)
			if len(rawHex) > 128 {
				rawHex = rawHex[:128] + "..."
			}

			// 2. 解析
			messages, err := m.parser.ParseFrame(pkt.Payload)
			if err != nil || len(messages) == 0 {
				m.updateStats(true, rawHex)
				continue
			}

			m.updateStats(false, rawHex)

			// 3. 补充元数据并注入 RawHex
			for i := range messages {
				messages[i].Data["signal_dbm"] = fmt.Sprintf("%d", pkt.SignalDBM)
				messages[i].Data["transport"] = pkt.Transport
				messages[i].RawHex = rawHex // 供 processor 的警告日志使用
			}

			// 4. 处理
			m.processor.ProcessDroneData(pkt.Source, messages)
		}
	}
}

func (m *Manager) updateStats(isError bool, hexSample string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if isError {
		m.aggregateStats.ParseErrors++
		m.aggregateStats.LastDroppedHex = hexSample
	} else {
		m.aggregateStats.DronesDetected++
		m.aggregateStats.LastParsedHex = hexSample
	}
}

func (m *Manager) Stop() {
	slog.Info("正在停止 RemoteID 监控管理器...")
	m.cancel()

	for _, c := range m.capturers {
		c.Stop()
	}

	m.wg.Wait()
	slog.Info("RemoteID 监控管理器已安全停止")
}

// ================= 暴露给 API 层的方法 =================

func (m *Manager) GetProcessor() *Processor {
	return m.processor
}

// GetLastPacketTime 实现 StatusProvider 接口，供 API 层健康检查使用
func (m *Manager) GetLastPacketTime() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastPacketTime
}

// GetCaptureStats 聚合所有 Capturer 的统计数据
func (m *Manager) GetCaptureStats() types.CaptureStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := m.aggregateStats
	stats.LastPacketTime = m.lastPacketTime

	// 累加各个 Capturer 的详细统计
	for _, c := range m.capturers {
		cStats := c.GetStats()
		stats.TotalPackets += cStats.TotalPackets
		stats.BeaconFrames += cStats.BeaconFrames
		stats.ActionFrames += cStats.ActionFrames
		stats.NormalDevices += cStats.NormalDevices
		if cStats.LastParsedHex != "" {
			stats.LastParsedHex = cStats.LastParsedHex
		}
		if cStats.LastDroppedHex != "" {
			stats.LastDroppedHex = cStats.LastDroppedHex
		}
	}
	return stats
}
