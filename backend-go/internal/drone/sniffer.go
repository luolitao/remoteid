// internal/drone/sniffer.go (最终纯净版 - 移除模拟器检测)
package drone

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/google/gopacket/pcapgo"
)

const (
	defaultSignalStrength   = -100  // 默认信号强度 (dBm)
	signalStrengthThreshold = -95   // 信号强度过滤阈值 (dBm)
	statsLogInterval        = 3 * time.Second // 统计日志输出间隔
)

type Sniffer struct {
	iface     string
	handle    *pcap.Handle
	processor *Processor
	closed    bool
	stats     struct {
		totalPackets   int
		beaconFrames   int
		probeRequests  int
		probeResponses int
		authFrames     int
		otherMgmt      int
		dronesDetected int
		// 移除 simulatorCount
		normalDevices int
		lastLogTime   time.Time
	}

	packetWriter *pcapgo.Writer
	recordFile   *os.File
}

func NewSniffer(iface string, processor *Processor) *Sniffer {
	return &Sniffer{
		iface:     iface,
		processor: processor,
	}
}

func (s *Sniffer) Start() error {
	slog.Debug("使用已配置的监听模式接口", "iface", s.iface)

	handle, err := pcap.OpenLive(s.iface, 1024, true, 1*time.Second)
	if err != nil {
		if strings.Contains(err.Error(), "Permission denied") {
			return fmt.Errorf("pcap 权限不足。请运行: sudo setcap 'cap_net_raw,cap_net_admin+eip' remoteid")
		}
		return fmt.Errorf("设备打开失败: %w", err)
	}
	s.handle = handle

	packetSource := gopacket.NewPacketSource(s.handle, s.handle.LinkType())
	packetSource.NoCopy = true

	go s.captureLoop(packetSource)

	slog.Info("2.4GHz 抓包引擎启动成功", "iface", s.iface)
	return nil
}

func (s *Sniffer) captureLoop(packetSource *gopacket.PacketSource) {
	slog.Info("开始 2.4GHz 数据包捕获", "mode", "beacon-only")

	for packet := range packetSource.Packets() {
		if packet == nil {
			continue
		}

		s.stats.totalPackets++

		if s.isBeaconFrame(packet) {
			s.recordPacket(packet)
		}

		s.processPacket(packet)

		if time.Since(s.stats.lastLogTime) > statsLogInterval {
			s.logStats()
			s.stats.lastLogTime = time.Now()
		}
	}
}

func (s *Sniffer) isBeaconFrame(packet gopacket.Packet) bool {
	dot11Layer := packet.Layer(layers.LayerTypeDot11)
	if dot11Layer == nil {
		return false
	}

	dot11, ok := dot11Layer.(*layers.Dot11)
	if !ok {
		return false
	}

	frameType := dot11.Type
	switch frameType {
	case layers.Dot11TypeMgmtBeacon: // 0x20
		return true
	default:
		return false
	}
}

func (s *Sniffer) recordPacket(packet gopacket.Packet) {
	if s.packetWriter == nil || s.recordFile == nil {
		return
	}

	captureInfo := packet.Metadata().CaptureInfo
	if err := s.packetWriter.WritePacket(captureInfo, packet.Data()); err != nil {
		slog.Warn("记录数据包失败", "error", err)
	}
}

func (s *Sniffer) logStats() {
	slog.Info("2.4GHz 抓包统计",
		"total_packets", s.stats.totalPackets,
		"beacon_frames", s.stats.beaconFrames,
		"drones", s.stats.dronesDetected,
		"normal_devices", s.stats.normalDevices)
}

func (s *Sniffer) processPacket(packet gopacket.Packet) {
	// 检查 RadioTap 层（信号强度信息）
	radioTapLayer := packet.Layer(layers.LayerTypeRadioTap)
	var signalStrength int = defaultSignalStrength
	if radioTapLayer != nil {
		if radioTap, ok := radioTapLayer.(*layers.RadioTap); ok {
			signalStrength = int(radioTap.DBMAntennaSignal)
		}
	}

	// 检查 Dot11 层
	dot11Layer := packet.Layer(layers.LayerTypeDot11)
	if dot11Layer == nil {
		return
	}

	dot11, ok := dot11Layer.(*layers.Dot11)
	if !ok {
		return
	}

	// 正确的类型比较
	frameType := dot11.Type

	isMgmtFrame := false
	frameSubtype := uint8(0)

	switch frameType {
	case layers.Dot11TypeMgmtBeacon: // 0x20
		isMgmtFrame = true
		frameSubtype = 8 // Beacon 子类型
	case layers.Dot11TypeMgmtProbeReq: // 0x10
		isMgmtFrame = true
		frameSubtype = 4 // Probe Request 子类型
	case layers.Dot11TypeMgmtProbeResp: // 0x14
		isMgmtFrame = true
		frameSubtype = 5 // Probe Response 子类型
	case layers.Dot11TypeMgmtAuthentication: // 0x2c
		isMgmtFrame = true
		frameSubtype = 11 // Authentication 子类型
	case layers.Dot11TypeMgmtAssociationReq: // 0x00
		isMgmtFrame = true
		frameSubtype = 0 // Association Request 子类型
	case layers.Dot11TypeMgmtAssociationResp: // 0x04
		isMgmtFrame = true
		frameSubtype = 1 // Association Response 子类型
	case layers.Dot11TypeMgmtDeauthentication: // 0x30
		isMgmtFrame = true
		frameSubtype = 12 // Deauthentication 子类型
	default:
		isMgmtFrame = false
		frameSubtype = 0
	}

	if !isMgmtFrame {
		return
	}

	// 统计管理帧类型
	switch frameSubtype {
	case 8: // Beacon
		s.stats.beaconFrames++
	case 4: // Probe Request
		s.stats.probeRequests++
	case 5: // Probe Response
		s.stats.probeResponses++
	case 11: // Authentication
		s.stats.authFrames++
	default:
		s.stats.otherMgmt++
	}

	// 获取源 MAC
	var srcMAC string
	if dot11.Address2 != nil {
		srcMAC = dot11.Address2.String()
	}
	if srcMAC == "" {
		return
	}

	// 信号强度过滤
	if signalStrength < signalStrengthThreshold {
		return
	}

	// 检查 Beacon 信息
	if frameSubtype == 8 { // Beacon
		beaconLayer := packet.Layer(layers.LayerTypeDot11MgmtBeacon)
		if beaconLayer != nil {
			if beacon, ok := beaconLayer.(*layers.Dot11MgmtBeacon); ok {
				// 从 Payload 解析信息元素
				payload := beacon.Payload
				if len(payload) > 0 {
					ssid := s.parseSSIDFromPayload(payload)
					if ssid != "" && ssid != "Unknown" {
						// 保留 SSID 信息用于设备识别
					}
				}
			}
		}
	}

	// 检查 GB42590 特征
	raw := packet.Data()

	gbPositions := s.findGB42590Features(raw)
	for _, pos := range gbPositions {
		slog.Debug("发现 GB42590 特征", "mac", srcMAC, "position", pos,
			"trailing_bytes", fmt.Sprintf("%x", raw[pos:pos+min(10, len(raw)-pos)]))
	}

	// 设备分类
	deviceType := s.classifyDevice(srcMAC, raw, gbPositions)

	switch deviceType {
	case "drone":
		slog.Info("检测到真实无人机", "mac", srcMAC, "signal_dbm", signalStrength)
		s.stats.dronesDetected++

		messages, err := s.processor.parser.ParseFrame(raw)
		if err != nil {
			slog.Warn("无人机数据解析失败", "mac", srcMAC, "error", err)
			return
		}

		if len(messages) > 0 {
			slog.Debug("无人机数据解析成功", "mac", srcMAC, "message_count", len(messages))
			s.processor.ProcessDroneData(srcMAC, messages)
		} else {
			slog.Warn("无人机无有效数据", "mac", srcMAC)
		}

	case "normal":
		// 不显示普通设备日志
		s.stats.normalDevices++

	default:
		// 不显示未知设备日志
	}
}

// 1. +++ 简化设备分类 +++
func (s *Sniffer) classifyDevice(mac string, raw []byte, gb []int) string {
	if s.isValidGB42590Device(raw, gb) {
		return "drone"
	}
	return "normal"
}

// 2. +++ 只检查 GB42590 +++
func (s *Sniffer) isValidGB42590Device(raw []byte, gb []int) bool {
	for _, pos := range gb {
		if s.isValidGB42590RemoteID(raw, pos) {
			return true
		}
	}
	return false
}

// 3. +++ 只保留 GB42590 特征查找 +++
func (s *Sniffer) findGB42590Features(data []byte) []int {
	var positions []int
	for i := 0; i < len(data)-3; i++ {
		if data[i] == 0xFA && data[i+1] == 0x0B && data[i+2] == 0xBC {
			positions = append(positions, i)
		}
	}
	return positions
}

func (s *Sniffer) isValidGB42590RemoteID(data []byte, pos int) bool {
	if pos+10 > len(data) {
		return false
	}

	// GB42590 Remote ID 通常紧跟 Vendor Type (0x0D)
	if pos+3 < len(data) && data[pos+3] == 0x0D {
		return true
	}

	return false
}

func (s *Sniffer) parseSSIDFromPayload(payload []byte) string {
	offset := 0
	for offset < len(payload) {
		if offset+2 > len(payload) {
			break
		}

		elementID := payload[offset]
		length := int(payload[offset+1])

		if offset+2+length > len(payload) {
			break
		}

		elementData := payload[offset+2 : offset+2+length]

		if elementID == 0 && length > 0 {
			ssid := strings.TrimFunc(string(elementData), func(r rune) bool {
				return r < 32 || r > 126
			})
			return ssid
		}

		offset += 2 + length
	}

	return "Unknown"
}

func (s *Sniffer) extractSSIDFromDot11(packet gopacket.Packet) string {
	for _, layer := range packet.Layers() {
		if layer.LayerType() == layers.LayerTypeDot11InformationElement {
			ie, ok := layer.(*layers.Dot11InformationElement)
			if !ok {
				continue
			}

			if ie.ID == 0 && len(ie.Info) > 0 {
				ssid := strings.TrimFunc(string(ie.Info), func(r rune) bool {
					return r < 32 || r > 126
				})

				if ssid == "" || strings.HasPrefix(ssid, "\x00") {
					return "Hidden_SSID"
				}
				return ssid
			}
		}
	}

	// 备用方法
	raw := packet.Data()
	if idx := bytes.Index(raw, []byte("DJI_")); idx != -1 {
		end := idx + 32
		if end > len(raw) {
			end = len(raw)
		}
		candidate := string(raw[idx:end])
		if nullIdx := strings.Index(candidate, "\x00"); nullIdx != -1 {
			candidate = candidate[:nullIdx]
		}
		if candidate != "" && len(candidate) > 4 {
			return candidate
		}
	}

	if bytes.Contains(raw, []byte("CAAC")) ||
		bytes.Contains(raw, []byte("GB425")) ||
		bytes.Contains(raw, []byte{0xFA, 0x0B, 0xBC}) {
		return "CAAC_Device"
	}

	srcMAC := ""
	if dot11Layer := packet.Layer(layers.LayerTypeDot11); dot11Layer != nil {
		if dot11, ok := dot11Layer.(*layers.Dot11); ok && dot11.Address2 != nil {
			srcMAC = dot11.Address2.String()
		}
	}

	if srcMAC != "" {
		lowerMAC := strings.ToLower(srcMAC)
		if strings.HasPrefix(lowerMAC, "54:75:95") ||
			strings.HasPrefix(lowerMAC, "d8:3a:dd") {
			return "DJI_Device"
		}
		if strings.HasPrefix(lowerMAC, "02:f8:31") {
			return "CAAC_Device"
		}
	}

	return "Unknown"
}

func (s *Sniffer) Stop() {
	if s.closed {
		return
	}

	slog.Info("2.4GHz 抓包引擎停止中...")

	if s.packetWriter != nil {
		if err := s.recordFile.Close(); err != nil {
			slog.Warn("关闭记录文件失败", "error", err)
		}
		slog.Info("Beacon 记录文件已保存", "file", s.recordFile.Name())
	}

	if s.handle != nil {
		s.handle.Close()
	}

	s.closed = true
	slog.Info("2.4GHz 抓包引擎停止完成")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
