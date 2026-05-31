// internal/drone/sniffer.go (最终纯净版 - 移除模拟器检测)
package drone

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"remoteid/pkg/types"
	"strings"
	"sync"
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
		actionFrames   int
		otherMgmt      int
		dronesDetected int
		// 移除 simulatorCount
		normalDevices int
		lastLogTime   time.Time
		lastPacket    time.Time // 最后一次收到数据包的时间
	}

	packetWriter *pcapgo.Writer
	recordFile   *os.File

	mu sync.Mutex // 保护 closed 和 handle 的并发访问
}

func NewSniffer(iface string, processor *Processor) *Sniffer {
	return &Sniffer{
		iface:     iface,
		processor: processor,
	}
}

func (s *Sniffer) Start() error {
	if err := s.openAndCapture(); err != nil {
		return err
	}

	// 启动健康检查：每 10 秒检查一次是否有新数据，超过 60 秒无数据则重连
	go s.healthCheckLoop()

	slog.Info("2.4GHz 抓包引擎启动成功", "iface", s.iface)
	return nil
}

// openAndCapture 打开网卡并启动抓包循环，失败时返回错误
func (s *Sniffer) openAndCapture() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return fmt.Errorf("sniffer 已关闭")
	}
	// 关闭旧 handle
	if s.handle != nil {
		s.handle.Close()
		s.handle = nil
	}
	s.mu.Unlock()

	slog.Debug("打开监听模式接口", "iface", s.iface)

	handle, err := pcap.OpenLive(s.iface, 1024, true, 1*time.Second)
	if err != nil {
		if strings.Contains(err.Error(), "Permission denied") {
			return fmt.Errorf("pcap 权限不足。请运行: sudo setcap 'cap_net_raw,cap_net_admin+eip' remoteid")
		}
		return fmt.Errorf("设备打开失败: %w", err)
	}

	s.mu.Lock()
	s.handle = handle
	s.mu.Unlock()

	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	packetSource.NoCopy = true

	go s.captureLoop(packetSource)

	return nil
}

// healthCheckLoop 定期检查抓包是否仍在接收数据，超时则自动重连
func (s *Sniffer) healthCheckLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		closed := s.closed
		lastPacket := s.stats.lastPacket
		s.mu.Unlock()

		if closed {
			return
		}

		// 如果超过 60 秒没收到数据包，尝试重连
		if !lastPacket.IsZero() && time.Since(lastPacket) > 60*time.Second {
			slog.Warn("抓包超时未收到数据，尝试重连",
				"last_packet", lastPacket.Format(time.RFC3339),
				"since_seconds", time.Since(lastPacket).Seconds())

			if err := s.openAndCapture(); err != nil {
				slog.Error("抓包重连失败，将在下次检查时重试", "error", err)
			} else {
				slog.Info("抓包重连成功")
			}
		}
	}
}

func (s *Sniffer) captureLoop(packetSource *gopacket.PacketSource) {
	slog.Info("开始 2.4GHz 数据包捕获", "mode", "beacon-only")

	for packet := range packetSource.Packets() {
		if packet == nil {
			continue
		}

		s.stats.totalPackets++
		s.stats.lastPacket = time.Now()

		if s.isBeaconFrame(packet) {
			s.recordPacket(packet)
		}

		s.processPacket(packet)

		if time.Since(s.stats.lastLogTime) > statsLogInterval {
			s.logStats()
			s.stats.lastLogTime = time.Now()
		}
	}

	slog.Warn("抓包循环退出，channel 已关闭，等待健康检查重连")
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
		"action_frames", s.stats.actionFrames,
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
	case layers.Dot11TypeMgmtAction: // 0xD0
		isMgmtFrame = true
		frameSubtype = 13 // Action 子类型（含 Public Action Frame -> NAN SDF）
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
	case 13: // Action (NAN SDF)
		s.stats.actionFrames++
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

	// 检查 Remote ID 特征（ASTM/ASD-STAN Beacon + GB42590 + NAN）
	raw := packet.Data()

	// 根据帧类型选择检测方式
	var deviceType string
	if frameSubtype == 13 {
		// Action Frame: 检测 NAN SDF (Wi-Fi Alliance OUI: 50:6F:9A)
		deviceType = s.classifyNANDevice(srcMAC, raw)
	} else {
		// Beacon: 检测 Vendor Specific IE (ASD-STAN OUI: FA:0B:BC + 0x0D)
		deviceType = s.classifyDevice(srcMAC, raw, nil)
	}

	switch deviceType {
	case "drone":
		slog.Info("检测到真实无人机", "mac", srcMAC, "signal_dbm", signalStrength)
		s.stats.dronesDetected++

		var messages []types.DroneMessage
		var err error
		if frameSubtype == 13 {
			messages, err = s.processor.parser.ParseNANFrame(raw)
		} else {
			messages, err = s.processor.parser.ParseFrame(raw)
		}
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

// classifyDevice 设备分类（ASTM/ASD-STAN + GB42590）
//
// ASTM Beacon 和 GB42590 使用相同的 Vendor Specific IE：
//
//	OUI: FA:0B:BC (ASD-STAN), OUI_Type: 0x0D
//
// 通过消息内容区分：
//   - 字节 0 高 4 位 = 0xF → Packed 格式 → GB42590-2023
//   - 字节 0 高 4 位 = 2 → ASTM F3411-22a
//   - 字节 0 高 4 位 = 0 或 1 → 通过 Basic ID 字段区分
func (s *Sniffer) classifyDevice(mac string, raw []byte, gb []int) string {
	if s.isValidRemoteID(raw) {
		return "drone"
	}
	return "normal"
}

// isValidRemoteID 统一检查是否为 Remote ID 设备
//
// 搜索 ASD-STAN OUI (FA:0B:BC) + OUI_Type (0x0D) 的 Vendor Specific IE，
// 验证消息格式有效性。
//
// GB42590 Beacon 格式: OUI + OUI_Type + [MsgCounter 1B] + Messages
// ASTM Beacon 格式:   OUI + OUI_Type + Messages
func (s *Sniffer) isValidRemoteID(raw []byte) bool {
	for i := 0; i < len(raw)-4; i++ {
		if raw[i] == 0xFA && raw[i+1] == 0x0B && raw[i+2] == 0xBC {
			if i+4 < len(raw) && raw[i+3] == 0x0D {
				payload := raw[i+4:]
				if len(payload) < 1 {
					continue
				}

				firstNibble := (payload[0] >> 4) & 0x0F
				lowNibble := payload[0] & 0x0F

				// GB42590 Packed 格式 (高4位 = 0xF, 低4位 = 0x1)
				if firstNibble == 0xF {
					return true
				}

				// ASTM: 高4位=协议版本(2), 低4位=消息类型(0-5)
				if firstNibble == 2 && lowNibble <= 5 {
					return true
				}

				// GB42590 单消息: 低4位=接口版本(0x1), 高4位=报文类型(0-5)
				if lowNibble == 0x1 && firstNibble <= 5 {
					return true
				}

				// GB42590: 可能有 Message Counter，检查下一字节
				if len(payload) > 1 {
					nextNibble := (payload[1] >> 4) & 0x0F
					nextLow := payload[1] & 0x0F
					// ASTM: 高4位=2, 低4位<=5
					if nextNibble == 2 && nextLow <= 5 {
						return true
					}
					// GB42590: 低4位=0x1, 高4位<=5
					if nextLow == 0x1 && nextNibble <= 5 {
						return true
					}
				}
			}
		}
	}
	return false
}

// classifyNANDevice 检测 NAN (Wi-Fi Alliance OUI: 50:6F:9A) 的 Remote ID 设备
//
// NAN Service Discovery Frame 结构:
//
//	Action Frame (0x0D) -> Public Action Frame -> NAN Attributes
//	OUI: 50:6F:9A (Wi-Fi Alliance), OUI_Type: 0x13 (NAN)
//
// 搜索 Wi-Fi Alliance OUI + NAN OUI_Type (0x13)，检测是否包含
// Remote ID Service ID 或 ASD-STAN Vendor Specific 数据。
func (s *Sniffer) classifyNANDevice(mac string, raw []byte) string {
	if s.isValidNANRemoteID(raw) {
		return "drone"
	}
	return "normal"
}

// isValidNANRemoteID 检查 Action Frame 中是否包含 NAN Remote ID 数据
//
// NAN SDF 中包含 Remote ID 的方式有两种：
//  1. NAN Service Descriptor Attribute 中的 Service ID 为 Remote ID 特定值
//  2. NAN Vendor Specific Attribute 中包含 ASD-STAN OUI (FA:0B:BC) 数据
//
// 这里搜索两个关键特征：
//   - Wi-Fi Alliance OUI: 50:6F:9A (NAN 标识)
//   - ASD-STAN OUI: FA:0B:BC + 0x0D (Remote ID 数据)
func (s *Sniffer) isValidNANRemoteID(raw []byte) bool {
	hasWiFiAlliance := false
	hasRemoteID := false

	for i := 0; i < len(raw)-3; i++ {
		// 检测 Wi-Fi Alliance OUI (50:6F:9A) — NAN 标识
		if raw[i] == 0x50 && raw[i+1] == 0x6F && raw[i+2] == 0x9A {
			// NAN OUI_Type 通常是 0x13
			if i+3 < len(raw) && raw[i+3] == 0x13 {
				hasWiFiAlliance = true
			}
		}

		// 检测 ASD-STAN OUI (FA:0B:BC) + 0x0D — Remote ID 数据
		if raw[i] == 0xFA && raw[i+1] == 0x0B && raw[i+2] == 0xBC {
			if i+3 < len(raw) && raw[i+3] == 0x0D {
				hasRemoteID = true
			}
		}
	}

	// NAN SDF 中的 Remote ID：需要同时有 NAN 标识和 Remote ID 数据
	return hasWiFiAlliance && hasRemoteID
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

// GetLastPacketTime 获取最后一次收到数据包的时间（用于健康检查）
func (s *Sniffer) GetLastPacketTime() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stats.lastPacket
}

func (s *Sniffer) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

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
		s.handle = nil
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
