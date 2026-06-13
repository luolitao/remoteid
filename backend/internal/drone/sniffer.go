package drone

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

// RawPacket 用于 Sniffer 到 Processor 的异步传输
type RawPacket struct {
	Payload []byte
	MAC     string
	RSSI    int
}

// 在 Sniffer 结构体中添加 Channel
type Sniffer struct {
	device    string
	processor *Processor
	handle    *pcap.Handle
	PacketCh  chan RawPacket // 新增：缓冲抓到的包
}

func NewSniffer(device string, processor *Processor) *Sniffer {
	return &Sniffer{
		device:    device,
		processor: processor,
	}
}

func (s *Sniffer) Start(ctx context.Context) error {
	// 1. 打开网卡
	handle, err := pcap.OpenLive(s.device, 65535, true, pcap.BlockForever)
	if err != nil {
		return err
	}
	s.handle = handle
	s.PacketCh = make(chan RawPacket, 500) // 500 缓冲池，抵抗突发流量
	defer s.handle.Close()

	// 2. 硬件级 BPF 过滤：只放行 Beacon 管理帧
	_ = s.handle.SetBPFFilter("wlan type mgt subtype beacon")
	log.Printf("[Sniffer] 硬件混杂模式开启，纯净模式监视网卡 %s...", s.device)

	var (
		radioTap layers.RadioTap
		dot11    layers.Dot11
	)

	// 3. 💡 核心改动：只解码到最基础的 Dot11，绝不使用容易报错的管理帧解析层
	parser := gopacket.NewDecodingLayerParser(
		layers.LayerTypeRadioTap,
		&radioTap,
		&dot11,
	)
	parser.IgnoreUnsupported = true
	decodedLayers := make([]gopacket.LayerType, 0, 4)

	packetSource := gopacket.NewPacketSource(s.handle, s.handle.LinkType())

	// 启动解析 Worker (建议放在 Processor 初始化中)
	// go s.processor.StartWorker(ctx, s.PacketCh)
	// sniffer.go -> Start() 函数内
	var packetCount atomic.Int64

	for {
		select {
		case <-ctx.Done():
			return nil
		case packet := <-packetSource.Packets():
			if packet == nil {
				continue
			}

			// ✅ 新增：打印原始包统计（每 100 个包打印一次，避免刷屏）
			// ✅ 正确用法：直接调用方法，无需取地址
			packetCount.Add(1)

			// 每 100 包打印一次统计
			if packetCount.Load()%100 == 0 { // ✅ 用 .Load() 读取
				slog.Debug("📦 抓包统计",
					"total", packetCount.Load(),
					"device", s.device)
			}

			// ... 原有解析逻辑 ...

			// 解析基础头
			err := parser.DecodeLayers(packet.Data(), &decodedLayers)
			if err != nil {
				continue
			}

			// 必须是 Beacon 帧才处理
			if dot11.Type != layers.Dot11TypeMgmtBeacon {
				continue
			}

			// 获取 MAC 和 RSSI
			macStr := ""
			if dot11.Address2 != nil {
				macStr = dot11.Address2.String()
			}
			rssi := 0
			if radioTap.DBMAntennaSignal != 0 {
				rssi = int(radioTap.DBMAntennaSignal)
			}

			// 💡 4. 手动切片提取 Elements
			payload := dot11.Payload
			// Beacon 帧固定包含 12 字节的基础参数 (Timestamp 8 + Interval 2 + Capabilities 2)
			if len(payload) <= 12 {
				continue
			}

			// 跳过前 12 字节，剩下的全是 TLV 格式的 Tags (Information Elements)
			tagsData := payload[12:]
			s.parseTagsAndProcess(tagsData, macStr, rssi)

		}
	}
}

// GetLastPacketTime 获取最后一次抓包时间
func (s *Sniffer) GetLastPacketTime() time.Time {
	return time.Now()
}

// 💡 核心解析逻辑：原生切片遍历 + OUI 早期过滤
func (s *Sniffer) parseTagsAndProcess(tagsData []byte, mac string, rssi int) {
	i := 0
	// 遍历标准的 TLV 结构 [Tag ID (1字节)] [Tag Length (1字节)] [Data (Length字节)]
	for i+1 < len(tagsData) {
		tagID := tagsData[i]
		tagLen := int(tagsData[i+1])

		if i+2+tagLen > len(tagsData) {
			break
		}

		// 221 (0xDD) 即为 Vendor Specific 厂商特定元素
		if tagID == 221 {
			// 💡 必须这样切！跳过 1字节ID 和 1字节长度，精准提取载荷
			ridPayload := tagsData[i+2 : i+2+tagLen]

			// 安全打印日志：防止切片越界 panic
			printLen := len(ridPayload)
			if printLen > 20 {
				printLen = 20
			}

			// 💡 OUI 早期过滤：至少需要 4 字节 (3字节OUI + 1字节类型)
			// sniffer.go -> parseTagsAndProcess()
			if len(ridPayload) >= 4 {
				oui := fmt.Sprintf("%02X:%02X:%02X", ridPayload[0], ridPayload[1], ridPayload[2])
				typ := ridPayload[3]

				// ✅ 新增：打印所有 Vendor Specific 元素的 OUI+Type
				slog.Debug("🔍 Vendor Element",
					"mac", mac,
					"oui", oui,
					"type", fmt.Sprintf("0x%02X", typ),
					"len", len(ridPayload),
				)

				isDrone := false
				if ridPayload[0] == 0xFA && ridPayload[1] == 0x0B && ridPayload[2] == 0xBC && ridPayload[3] == 0x0D {
					isDrone = true
					// slog.Info("🎯 命中 ASTM OUI", "mac", mac)
				} else if ridPayload[0] == 0x06 && ridPayload[1] == 0x05 && ridPayload[2] == 0x04 && ridPayload[3] == 0xFD {
					isDrone = true
					slog.Info("🎯 命中 GB OUI", "mac", mac)
				}

				if isDrone {
					s.processor.ProcessPacket(ridPayload, mac, rssi)
				}
			}
		}

		i += 2 + tagLen
	}
}
