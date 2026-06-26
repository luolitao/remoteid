package drone

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

const (
	defaultSignalStrength   = -100
	signalStrengthThreshold = -200 // 临时禁用
)

type RawPacket struct {
	Payload   []byte
	MAC       string
	RSSI      int
	Timestamp time.Time
}

type Sniffer struct {
	device   string
	packetCh chan<- RawPacket
	handle   *pcap.Handle

	stats struct {
		totalPackets   atomic.Int64
		beaconFrames   atomic.Int64
		actionFrames   atomic.Int64
		dronesDetected atomic.Int64
	}

	mu        sync.Mutex
	macCounts map[string]int
}

func NewSniffer(device string, packetCh chan<- RawPacket) *Sniffer {
	return &Sniffer{
		device:    device,
		packetCh:  packetCh,
		macCounts: make(map[string]int),
	}
}

func (s *Sniffer) Start(ctx context.Context) error {
	handle, err := pcap.OpenLive(s.device, 262144, true, 1*time.Second)
	if err != nil {
		return fmt.Errorf("打开网卡 %s 失败: %w", s.device, err)
	}
	s.handle = handle
	// ❌ 删除 defer s.handle.Close()
	// ✅ 改为在 Stop 或 captureLoop 退出时关闭

	// 清空 BPF 过滤器
	_ = handle.SetBPFFilter("")

	slog.Info("Sniffer 已启动，开始捕获管理帧", "device", s.device)

	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	packetSource.NoCopy = false

	go s.captureLoop(ctx, packetSource)
	return nil
}

func (s *Sniffer) captureLoop(ctx context.Context, packetSource *gopacket.PacketSource) {
	ticker := time.NewTicker(60 * time.Second)
	go func() {
		for range ticker.C {
			slog.Info("Sniffer packet stats",
				"total", s.stats.totalPackets.Load(),
				"beacons", s.stats.beaconFrames.Load(),
				"actions", s.stats.actionFrames.Load(),
				"drones", s.stats.dronesDetected.Load(),
			)
		}
	}()
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("Sniffer 捕获循环停止")
			// 在这里关闭 handle，释放资源
			if s.handle != nil {
				s.handle.Close()
				s.handle = nil
			}
			return
		case packet, ok := <-packetSource.Packets():
			if !ok {
				slog.Warn("Packet channel closed")
				// 如果通道意外关闭，也关闭 handle 并退出
				if s.handle != nil {
					s.handle.Close()
					s.handle = nil
				}
				return
			}
			// ★ 关键：检查 packet 是否为 nil
			if packet == nil {
				continue
			}
			s.stats.totalPackets.Add(1)

			// 安全获取 RadioTap
			signalStrength := defaultSignalStrength
			if radioTapLayer := packet.Layer(layers.LayerTypeRadioTap); radioTapLayer != nil {
				if radioTap, ok := radioTapLayer.(*layers.RadioTap); ok && radioTap != nil {
					signalStrength = int(radioTap.DBMAntennaSignal)
				}
			}
			// 暂时禁用信号过滤
			// if signalStrength < signalStrengthThreshold { continue }

			// 安全获取 Dot11
			dot11Layer := packet.Layer(layers.LayerTypeDot11)
			if dot11Layer == nil {
				continue
			}
			dot11, ok := dot11Layer.(*layers.Dot11)
			if !ok || dot11 == nil || dot11.Address2 == nil {
				continue
			}

			var frameBody []byte
			switch dot11.Type {
			case layers.Dot11TypeMgmtBeacon:
				s.stats.beaconFrames.Add(1)
				if beaconLayer := packet.Layer(layers.LayerTypeDot11MgmtBeacon); beaconLayer != nil {
					if beacon, ok := beaconLayer.(*layers.Dot11MgmtBeacon); ok && beacon != nil {
						frameBody = beacon.Payload
					}
				}
			case layers.Dot11TypeMgmtAction:
				s.stats.actionFrames.Add(1)
				if actionLayer := packet.Layer(layers.LayerTypeDot11MgmtAction); actionLayer != nil {
					if action, ok := actionLayer.(*layers.Dot11MgmtAction); ok && action != nil {
						frameBody = action.Payload
					}
				}
			default:
				continue
			}

			if len(frameBody) == 0 {
				continue
			}

			mac := dot11.Address2.String()
			s.parseTagsAndProcess(frameBody, mac, signalStrength)
		}
	}
}

// parseTagsAndProcess 安全遍历 IE
func (s *Sniffer) parseTagsAndProcess(ieList []byte, mac string, rssi int) {
	i := 0
	found := false
	for i+1 < len(ieList) {
		tagID := ieList[i]
		tagLen := int(ieList[i+1])
		if i+2+tagLen > len(ieList) {
			break
		}
		if tagID == 221 && tagLen >= 4 {
			ridPayload := ieList[i+2 : i+2+tagLen]
			if ridPayload[0] == 0xFA && ridPayload[1] == 0x0B &&
				ridPayload[2] == 0xBC && ridPayload[3] == 0x0D {
				found = true
				break // 只要找到一个 OUI 就发送整个 ieList
			}
		}
		i += 2 + tagLen
	}
	if found {
		s.handleRemoteID(ieList, mac, rssi) // ✅ 发送整个 IE 列表
	}
}

// handleRemoteID 安全发送
func (s *Sniffer) handleRemoteID(ieList []byte, mac string, rssi int) {
	s.mu.Lock()
	s.macCounts[mac]++
	count := s.macCounts[mac]
	s.mu.Unlock()

	s.stats.dronesDetected.Add(1)

	if count == 1 || count%100 == 0 {
		slog.Info("📡 捕获到 Remote ID 信标",
			"mac", mac,
			"signal_dbm", rssi,
			"msg_count", count,
		)
	}

	select {
	case s.packetCh <- RawPacket{
		Payload:   bytes.Clone(ieList), // ✅ 发送完整 IE 列表
		MAC:       mac,
		RSSI:      rssi,
		Timestamp: time.Now(),
	}:
	default:
		slog.Warn("packetCh 已满，丢弃 Remote ID 数据包", "mac", mac)
	}
}

func (s *Sniffer) GetStats() map[string]int64 {
	return map[string]int64{
		"total_packets":   s.stats.totalPackets.Load(),
		"beacon_frames":   s.stats.beaconFrames.Load(),
		"action_frames":   s.stats.actionFrames.Load(),
		"drones_detected": s.stats.dronesDetected.Load(),
	}
}

func (s *Sniffer) GetLastPacketTime() time.Time {
	return time.Now()
}

func (s *Sniffer) Stop() error {
	if s.handle != nil {
		// 外部调用 Stop 时关闭句柄
		s.handle.Close()
		s.handle = nil
	}
	return nil
}
