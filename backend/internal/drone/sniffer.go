package drone

import (
	"context"
	"log"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

type Sniffer struct {
	device    string
	processor *Processor
	handle    *pcap.Handle
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

	for {
		select {
		case <-ctx.Done():
			return nil
		case packet := <-packetSource.Packets():
			if packet == nil {
				continue
			}

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
			if len(ridPayload) >= 4 {
				isDrone := false

				// 规则 1: 现代 ASD-STAN / GB46750 标准 (OUI: FA:0B:BC, Type: 0x0D)
				if ridPayload[0] == 0xFA && ridPayload[1] == 0x0B && ridPayload[2] == 0xBC && ridPayload[3] == 0x0D {
					isDrone = true
				} else if ridPayload[0] == 0x06 && ridPayload[1] == 0x05 && ridPayload[2] == 0x04 && ridPayload[3] == 0xFD {
					// 规则 2: 早期 ASTM 标准 (OUI: 06:05:04, Type: 0xFD)
					isDrone = true
				}

				// 如果确认是无人机 OUI，才送入 Processor 处理
				if isDrone {
					// slog.Info("🎯 命中无人机 OUI，送入处理！", "MAC", mac, "| 长度:", len(ridPayload), " 字节 | 原始Hex:", hex.EncodeToString(ridPayload))
					s.processor.ProcessPacket(ridPayload, mac, rssi)
				}
			}
		}

		i += 2 + tagLen
	}
}
