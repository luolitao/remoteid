package capture

import (
	"bytes"
	"context"
	"encoding/hex"
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

type WiFiCapturer struct {
	iface  string
	handle *pcap.Handle
	name   string
	mu     sync.Mutex

	stats types.CaptureStats // 内部统计
}

func NewWiFiCapturer(iface string) *WiFiCapturer {
	return &WiFiCapturer{
		iface: iface,
		name:  "WiFi",
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

			c.mu.Lock()
			c.stats.TotalPackets++
			c.stats.LastPacketTime = packet.Metadata().Timestamp
			c.mu.Unlock()

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

			// 仅捕获 Beacon 和 Action 管理帧
			if dot11.Type != layers.Dot11TypeMgmtBeacon && dot11.Type != layers.Dot11TypeMgmtAction {
				continue
			}

			c.mu.Lock()
			if dot11.Type == layers.Dot11TypeMgmtBeacon {
				c.stats.BeaconFrames++
			} else {
				c.stats.ActionFrames++
			}
			c.mu.Unlock()

			// 3. 构造统一的 RawPacket (必须 Clone 防止底层 buffer 被复用)
			payloadCopy := bytes.Clone(packet.Data())
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
				c.mu.Lock()
				c.stats.ParseErrors++ // 通道满视为丢弃
				c.stats.LastDroppedHex = hex.EncodeToString(payloadCopy)[:min(128, len(payloadCopy)*2)]
				c.mu.Unlock()
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
	c.mu.Lock()
	defer c.mu.Unlock()
	// 返回副本
	stats := c.stats
	if len(stats.LastParsedHex) > 128 {
		stats.LastParsedHex = stats.LastParsedHex[:128] + "..."
	}
	if len(stats.LastDroppedHex) > 128 {
		stats.LastDroppedHex = stats.LastDroppedHex[:128] + "..."
	}
	return stats
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
