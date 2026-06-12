package drone

import (
	"context"
	"log"
	"net"

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
	// 分配 16MB 缓冲区防突发丢包
	handle, err := pcap.OpenLive(s.device, 65535, true, pcap.BlockForever)
	if err != nil {
		return err
	}
	s.handle = handle
	defer s.handle.Close()

	// 硬件级 BPF 过滤：只捕获 Beacon 与 Action 管理帧
	_ = s.handle.SetBPFFilter("wlan type mgt subtype beacon || wlan type mgt subtype action")
	log.Printf("[Sniffer] 硬件混杂模式开启，全面监视网卡 %s...", s.device)

	// 预分配零拷贝 Layer 缓存区，规避大吞吐高频 GC 震荡
	var eth layers.RadioTap
	var dot11 layers.Dot11
	parser := gopacket.NewDecodingLayerParser(layers.LayerTypeRadioTap, &eth, &dot11)
	decoded := make([]gopacket.LayerType, 0, 2)

	packetSource := gopacket.NewPacketSource(s.handle, s.handle.LinkType())

	for {
		select {
		case <-ctx.Done():
			return nil
		case packet := <-packetSource.Packets():
			if packet == nil {
				continue
			}

			err := parser.DecodeLayers(packet.Data(), &decoded)
			if err != nil {
				continue
			}

			rssi := 0
			if eth.DBMAntennaSignal != 0 {
				rssi = int(eth.DBMAntennaSignal)
			}

			macStr := ""
			if dot11.Address2 != nil {
				macStr = net.HardwareAddr(dot11.Address2).String()
			}

			payload := packet.ApplicationLayer()
			if payload == nil {
				if len(dot11.Payload) > 0 {
					s.processor.ProcessPacket(dot11.Payload, macStr, rssi)
				}
				continue
			}

			s.processor.ProcessPacket(payload.Payload(), macStr, rssi)
		}
	}
}
