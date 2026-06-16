package capture

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"remoteid-monitor/pkg/types"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

const (
	defaultSignalStrength   = -100
	signalStrengthThreshold = -95
)

// WiFiCapturer 实现了 Capturer 接口，专门用于捕获 2.4GHz/5GHz WiFi 管理帧
type WiFiCapturer struct {
	iface     string
	handle    *pcap.Handle
	name      string
	mu        sync.Mutex // 保护 handle
	muStats   sync.Mutex // 保护 stats 和 macCounts
	stats     types.CaptureStats
	macCounts map[string]int // ✅ 新增：用于控制底层日志打印频率
}

// NewWiFiCapturer 创建一个新的 WiFi 捕获器实例
func NewWiFiCapturer(iface string) *WiFiCapturer {
	return &WiFiCapturer{
		iface:     iface,
		name:      "WiFi",
		macCounts: make(map[string]int), // ✅ 初始化计数器
	}
}

func (c *WiFiCapturer) Name() string { return c.name }

func (c *WiFiCapturer) Start(ctx context.Context, out chan<- RawPacket) error {
	c.mu.Lock()
	if c.handle != nil {
		c.handle.Close()
		c.handle = nil
	}
	slog.Info("正在初始化 WiFi 捕获器", "interface", c.iface)

	handle, err := pcap.OpenLive(c.iface, 1024, true, 1*time.Second)
	if err != nil {
		c.mu.Unlock()
		if strings.Contains(err.Error(), "Permission denied") {
			return fmt.Errorf("pcap 权限不足。请确保以 root 运行，或执行: sudo setcap 'cap_net_raw,cap_net_admin+eip' $(which remoteid-monitor)")
		}
		return fmt.Errorf("打开网卡 %s 失败: %w", c.iface, err)
	}

	c.handle = handle
	c.mu.Unlock()

	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	packetSource.NoCopy = true
	slog.Info("WiFi 捕获器启动成功，开始监听管理帧", "interface", c.iface)

	go c.captureLoop(ctx, packetSource, out)
	return nil
}

func (c *WiFiCapturer) captureLoop(ctx context.Context, packetSource *gopacket.PacketSource, out chan<- RawPacket) {
	for {
		select {
		case <-ctx.Done():
			slog.Info("WiFi 捕获器收到停止信号，退出抓包循环")
			return
		case packet := <-packetSource.Packets():
			if packet == nil {
				continue
			}

			c.muStats.Lock()
			c.stats.TotalPackets++
			c.stats.LastPacketTime = packet.Metadata().Timestamp
			c.muStats.Unlock()

			// 1. 提取 RadioTap 层 (信号强度)
			signalStrength := defaultSignalStrength
			if radioTapLayer := packet.Layer(layers.LayerTypeRadioTap); radioTapLayer != nil {
				if radioTap, ok := radioTapLayer.(*layers.RadioTap); ok {
					signalStrength = int(radioTap.DBMAntennaSignal)
				}
			}

			if signalStrength < signalStrengthThreshold {
				continue
			}

			// 2. 提取 Dot11 层并判断帧类型
			dot11Layer := packet.Layer(layers.LayerTypeDot11)
			if dot11Layer == nil {
				continue
			}

			dot11, ok := dot11Layer.(*layers.Dot11)
			if !ok || dot11.Address2 == nil {
				continue
			}

			// ✅ 修复 QF1003：将 if/else-if 链重构为 tagged switch
			var frameBody []byte
			switch dot11.Type {
			case layers.Dot11TypeMgmtBeacon:
				c.muStats.Lock()
				c.stats.BeaconFrames++
				c.muStats.Unlock()
				if beaconLayer := packet.Layer(layers.LayerTypeDot11MgmtBeacon); beaconLayer != nil {
					if beacon, ok := beaconLayer.(*layers.Dot11MgmtBeacon); ok {
						frameBody = beacon.Payload // 纯净的 IE 列表
					}
				}
			case layers.Dot11TypeMgmtAction:
				c.muStats.Lock()
				c.stats.ActionFrames++
				c.muStats.Unlock()
				if actionLayer := packet.Layer(layers.LayerTypeDot11MgmtAction); actionLayer != nil {
					if action, ok := actionLayer.(*layers.Dot11MgmtAction); ok {
						frameBody = action.Payload // Action Frame 的 Payload
					}
				}
			default:
				continue // 忽略其他帧类型
			}

			if len(frameBody) == 0 {
				continue
			}

			// ✅ 核心新增：在 Capturer 层快速识别 Remote ID 特征并控制日志输出
			// 使用 bytes.Contains 快速扫描 ASD-STAN OUI (FA:0B:BC + 0x0D) 或 Legacy OUI
			isRemoteID := bytes.Contains(frameBody, []byte{0xFA, 0x0B, 0xBC, 0x0D}) ||
				bytes.Contains(frameBody, []byte{0x06, 0x05, 0x04, 0xFD})

			if isRemoteID {
				srcMAC := dot11.Address2.String()

				c.muStats.Lock()
				c.macCounts[srcMAC]++
				count := c.macCounts[srcMAC]
				c.stats.DronesDetected++
				c.muStats.Unlock()

				// 仅在第 1 次，或每 100 次打印，彻底解决刷屏问题
				if count == 1 || count%100 == 0 {
					hexLen := len(frameBody)
					if hexLen > 128 {
						hexLen = 128
					}
					rawHex := fmt.Sprintf("%X", frameBody[:hexLen])
					if len(frameBody) > 128 {
						rawHex += "..."
					}

					slog.Info("📡 捕获到 Remote ID 信标 (WiFi Capturer)",
						"mac", srcMAC,
						"signal_dbm", signalStrength,
						"msg_count", count,
						"frame_body_hex", rawHex, // 打印纯净的 IE 列表 16 进制
					)
				}
			}

			// 3. 构造统一的 RawPacket
			// 注意：由于启用了 NoCopy，必须复制 Payload 以防止底层 buffer 被 gopacket 复用覆盖
			payloadCopy := bytes.Clone(frameBody)

			rawPacket := RawPacket{
				Source:    dot11.Address2.String(),
				Timestamp: packet.Metadata().Timestamp,
				SignalDBM: signalStrength,
				Payload:   payloadCopy,
				Transport: "wifi",
			}

			// 4. 非阻塞发送
			select {
			case out <- rawPacket:
			default:
				c.muStats.Lock()
				c.stats.ParseErrors++
				c.muStats.Unlock()
				slog.Warn("WiFi 数据包处理通道已满，丢弃数据包", "mac", rawPacket.Source)
			}
		}
	}
}

func (c *WiFiCapturer) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.handle != nil {
		slog.Info("正在关闭 WiFi 捕获器", "interface", c.iface)
		c.handle.Close()
		c.handle = nil
	}
	return nil
}

func (c *WiFiCapturer) GetStats() types.CaptureStats {
	c.muStats.Lock()
	defer c.muStats.Unlock()
	return c.stats
}
