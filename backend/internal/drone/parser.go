// internal/drone/parser.go
package drone

// 需要导入的包
import (
	"encoding/binary"
	"fmt"
	"math"
	"remoteid-monitor/pkg/types"
	"strings"
	"unicode/utf8"
)

// ========== 协议概述 ==========
//
// 支持的 Remote ID 标准：
//
//  ASTM F3411-22a / ASD-STAN prEN 4709-002（国际标准）
//     - WiFi Beacon: Vendor Specific IE (Element ID 0xDD)
//       OUI: FA:0B:BC (ASD-STAN), OUI_Type: 0x0D
//       Vendor IE 结构: [0xDD] [Len] [OUI:3B] [VendType:1B] [MsgCounter:1B] [Messages...]
//       每条消息 25 字节: [Header:1B] [Payload:24B]
//       Header: 高4位=消息类型(0-5), 低4位=协议版本(2)
//     - WiFi NAN: NAN Service Discovery Frame
//       OUI: 50:6F:9A (Wi-Fi Alliance)
//
//  GB 42590-2023（中国国标）
//     - 复用 ASD-STAN OUI (FA:0B:BC) + OUI_Type (0x0D)
//     - 协议版本 nibble = 0x1（区别于 ASTM 的 0x2）
//     - 字段偏移与 ASTM 不同：Location/System 消息比 ASTM 多 1 字节偏移
//     - 支持 Packed 格式（消息类型 0xF）
//
//  GB 46750-2023（中国国标）
//     - 复用 ASD-STAN OUI (FA:0B:BC) + OUI_Type (0x0D)
//     - 完全自定义的变长消息格式（非 25 字节固定消息）
//     - 通过 data[1]==0xFF 识别（区别于 ASTM 的 Header 和 GB 42590 的 0xF1）
//     - 格式: [MessageCounter:1B] [0xFF:1B] [版本号:1B] [数据长度:1B] [数据标识:3B] [数据内容:变长]
//     - 数据标识位表定义 21 个数据项，按需组合

const (
	// ASTM/ASD-STAN Vendor Specific IE OUI
	asdStanOUI     = "\xFA\x0B\xBC" // ASD-STAN OUI
	asdStanOUIType = 0x0D           // ASD-STAN OUI Type
	msgSize        = 25             // 每条 Remote ID 消息固定 25 字节

	// 旧版 ASTM / OpenDroneID 临时 OUI（部分 ESP32 早期实现使用）
	legacyASTMOUI     = "\x06\x05\x04" // 旧版 ASTM OUI
	legacyASTMOUIType = 0xFD           // 旧版 ASTM OUI Type

	// ASTM 专属常量
	astmProtocolVersion = 2 // ASTM F3411-22a 协议版本号（字节 0 高 4 位）

	// GB 42590-2023 常量
	gbProtocolVersion = 1 // GB 42590-2023 协议版本号（字节 0 高 4 位）

	// GB 46750-2023 常量
	gb46750Magic        = 0xFF // GB 46750 数据类型标识魔数（data[1]）
	gb46750MajorVersion = 0x1  // GB 46750 主版本号（版本字节高3位）
)

type RemoteIDParser struct{}

func NewParser() *RemoteIDParser {
	return &RemoteIDParser{}
}

// ParseFrame 解析 Beacon 数据帧中的 Remote ID 消息（ASTM/ASD-STAN）
//
// 支持格式:
//   - WiFi Beacon Vendor Specific IE (OUI: FA:0B:BC + OUI_Type: 0x0D)
//   - ASTM F3411-22a
func (p *RemoteIDParser) ParseFrame(raw []byte) ([]types.DroneMessage, error) {
	return p.parseVendorIE(raw)
}

// ParseNANFrame 解析 NAN Service Discovery Frame 中的 Remote ID 消息
//
// NAN SDF 中 Remote ID 的承载方式：
//   - NAN Vendor Specific Attribute (Attribute ID 0xDD) 中携带 ASD-STAN OUI 数据
//   - 解析后得到的 Remote ID 消息与 Beacon 格式相同（ASTM F3411-22a）
//
// NAN SDF 结构概述：
//
//	Action Frame (0x0D) -> Category: Public (0x04) -> Action: NAN (0x09)
//	-> OUI: 50:6F:9A (Wi-Fi Alliance) -> OUI_Type: 0x13 (NAN)
//	-> NAN Attributes (TLV 格式)
//	   -> Attribute ID 0xDD (Vendor Specific) -> ASD-STAN OUI (FA:0B:BC) + Remote ID Messages
func (p *RemoteIDParser) ParseNANFrame(raw []byte) ([]types.DroneMessage, error) {
	return p.parseNANSDF(raw)
}

// parseNANSDF 解析 NAN Service Discovery Frame 中的 Remote ID 消息
//
// 在原始帧数据中搜索 Wi-Fi Alliance OUI (50:6F:9A) + NAN OUI_Type (0x13)，
// 然后在其后的 NAN Attributes 中查找 Vendor Specific Attribute (ID 0xDD)，
// 该属性包含 ASD-STAN OUI (FA:0B:BC) + Remote ID 数据。
func (p *RemoteIDParser) parseNANSDF(raw []byte) ([]types.DroneMessage, error) {
	var messages []types.DroneMessage

	// 搜索 Wi-Fi Alliance OUI (50:6F:9A) + NAN OUI_Type (0x13)
	for idx := 0; idx <= len(raw)-4; idx++ {
		if raw[idx] != 0x50 || raw[idx+1] != 0x6F || raw[idx+2] != 0x9A {
			continue
		}
		if idx+3 >= len(raw) || raw[idx+3] != 0x13 {
			continue
		}

		// 找到 NAN 标识，现在在其后的 NAN Attributes 中搜索 Remote ID 数据
		// NAN Attributes 从 OUI_Type 之后开始（即 idx+4 之后）
		attrOffset := idx + 4

		// 搜索 NAN Vendor Specific Attribute (ID 0xDD) 中包含 ASD-STAN OUI 的
		msgs := p.parseNANAttributes(raw, attrOffset)
		messages = append(messages, msgs...)

		// 如果已经在 NAN Attributes 中找到了 Vendor Specific IE 里的 Remote ID，
		// 同时也在整个原始帧中搜索可能嵌入的 ASD-STAN Vendor IE
		// （有些实现直接在 NAN Action Frame 的 payload 中嵌入完整的 Vendor Specific IE）
		if len(messages) == 0 {
			msgs, _ := p.parseVendorIE(raw)
			messages = append(messages, msgs...)
		}

		break // 找到一处 NAN OUI 后退出
	}

	return messages, nil
}

// parseNANAttributes 解析 NAN Attributes 中的 Remote ID 数据
//
// NAN Attribute 是 TLV 格式：
//
//	[Attribute ID: 1B] [Length: 2B LE] [Value: Length bytes]
//
// Vendor Specific Attribute (ID 0xDD) 的 Value 包含：
//
//	[OUI: 3B] [Vendor Specific Body...]
//
// 如果 OUI 为 FA:0B:BC (ASD-STAN)，则 Body 中包含 Remote ID 消息。
func (p *RemoteIDParser) parseNANAttributes(raw []byte, offset int) []types.DroneMessage {
	var messages []types.DroneMessage

	for offset+3 <= len(raw) {
		attrID := raw[offset]
		if attrID == 0x00 {
			// NAN Attribute ID 0 表示结束
			break
		}

		if offset+3 > len(raw) {
			break
		}
		attrLen := int(binary.LittleEndian.Uint16(raw[offset+1 : offset+3]))

		if attrLen == 0 || offset+3+attrLen > len(raw) {
			break
		}

		attrValue := raw[offset+3 : offset+3+attrLen]

		// 检查 Vendor Specific Attribute (ID 0xDD)
		if attrID == 0xDD && len(attrValue) >= 5 {
			// 检查 OUI 是否为 ASD-STAN (FA:0B:BC) + OUI_Type (0x0D)
			if attrValue[0] == 0xFA && attrValue[1] == 0x0B && attrValue[2] == 0xBC && attrValue[3] == 0x0D {
				// 扫描查找 ASTM 消息 Header（跳过 Message Counter + 可能的额外元数据）
				msgStart := p.findASTMMessageHeader(attrValue, 4)
				if msgStart >= 0 {
					msgs := p.parseASTMBeaconMessages(attrValue[msgStart:])
					messages = append(messages, msgs...)
				}
			}
		}

		// 移动到下一个 Attribute
		offset += 3 + attrLen
	}

	return messages
}

// parseVendorIE 解析 Vendor Specific IE 中的 Remote ID 消息
//
// 支持三种协议格式（均使用 OUI: FA:0B:BC + OUI_Type: 0x0D）：
//
//  1. GB 46750-2023: data[1]==0xFF，变长自定义格式
//     格式: [MsgCounter:1B] [0xFF:1B] [版本:1B] [数据长度:1B] [数据标识:3B] [数据内容:变长]
//
//  2. ASTM F3411-22a / GB 42590-2023: 固定 25 字节消息
//     Header: 高4位=消息类型(0-5), 低4位=协议版本(1=GB, 2=ASTM)
//
//  3. 旧版 ASTM (OUI: 06:05:04 + 0xFD)
//
// 检测策略：先检测 GB 46750 (data[1]==0xFF)，再查找 ASTM/GB 消息 Header
func (p *RemoteIDParser) parseVendorIE(raw []byte) ([]types.DroneMessage, error) {
	var messages []types.DroneMessage

	// 搜索 ASD-STAN OUI (FA:0B:BC) + OUI_Type (0x0D)
	for idx := 0; idx <= len(raw)-5; idx++ {
		if string(raw[idx:idx+3]) != asdStanOUI || raw[idx+3] != asdStanOUIType {
			continue
		}

		// OUI+Type 之后的数据: [MsgCounter:1B] [payload...]
		dataStart := idx + 4 // OUI+Type 之后
		if dataStart >= len(raw) {
			continue
		}

		// 策略 1: 检测 GB 46750-2023 格式 (data[1]==0xFF)
		if p.isGB46750Format(raw, dataStart) {
			msgs := p.parseGB46750Payload(raw[dataStart:])
			messages = append(messages, msgs...)
			if len(messages) > 0 {
				break
			}
		}

		// 策略 2: 查找 ASTM/GB 消息 Header
		msgStart := p.findASTMMessageHeader(raw, dataStart)
		if msgStart >= 0 {
			payload := raw[msgStart:]
			msgs := p.parseASTMBeaconMessages(payload)
			messages = append(messages, msgs...)
			if len(messages) > 0 {
				break
			}
		}
	}

	// 如果没找到标准 ASD-STAN OUI，尝试搜索旧版 ASTM OUI (06:05:04 + 0xFD)
	if len(messages) == 0 {
		for idx := 0; idx <= len(raw)-5; idx++ {
			if string(raw[idx:idx+3]) != legacyASTMOUI || raw[idx+3] != legacyASTMOUIType {
				continue
			}

			scanStart := idx + 4
			msgStart := p.findASTMMessageHeader(raw, scanStart)
			if msgStart < 0 {
				continue
			}

			payload := raw[msgStart:]
			msgs := p.parseASTMBeaconMessages(payload)
			messages = append(messages, msgs...)
			if len(messages) > 0 {
				break
			}
		}
	}

	return messages, nil
}

// isGB46750Format 检测数据是否为 GB 46750-2023 格式
// data 从 OUI+Type 之后开始（包含 Message Counter）
// GB 46750 格式: [MsgCounter:1B] [0xFF:1B] [版本号:1B] [数据长度:1B] [数据标识:3B] [数据内容:变长]
// 检测条件: data[1]==0xFF 且版本号高3位==0x1 且长度足够
func (p *RemoteIDParser) isGB46750Format(raw []byte, dataStart int) bool {
	if dataStart+7 > len(raw) {
		return false
	}
	// data[1] == 0xFF (魔数)
	if raw[dataStart+1] != gb46750Magic {
		return false
	}
	// 版本号高3位 == 0x1
	version := raw[dataStart+2]
	majorVer := (version >> 5) & 0x07
	if majorVer != gb46750MajorVersion {
		return false
	}
	return true
}

// findASTMMessageHeader 在 raw 中从 scanStart 开始扫描，查找 ASTM 或 GB 消息 Header
// Header 格式: 高4位=消息类型(0-5), 低4位=协议版本(1=GB42590 或 2=ASTM)
// 即 byte 满足 (b>>4)<=5 && ((b&0x0F)==1 || (b&0x0F)==2)
// 最多扫描 8 字节（覆盖 Message Counter + 可能的额外元数据）
func (p *RemoteIDParser) findASTMMessageHeader(raw []byte, scanStart int) int {
	maxScan := scanStart + 8
	if maxScan > len(raw) {
		maxScan = len(raw)
	}
	for i := scanStart; i < maxScan; i++ {
		b := raw[i]
		msgType := (b >> 4) & 0x0F
		protoVer := b & 0x0F
		// ASTM (protoVer=2) 或 GB (protoVer=1)
		if msgType <= 5 && (protoVer == astmProtocolVersion || protoVer == gbProtocolVersion) {
			return i
		}
		// 兼容旧版格式：高4位=协议版本, 低4位=消息类型
		if (msgType == astmProtocolVersion || msgType == gbProtocolVersion) && protoVer <= 5 {
			return i
		}
	}
	return -1
}

// parseASTMBeaconMessages 解析 Beacon 格式的单条/多条消息
// 支持 ASTM F3411-22a 和 GB 42590-2023，通过 Header 字节的低4位区分：
//
//	低4位=1 → GB 42590-2023（字段偏移与 ASTM 不同）
//	低4位=2 → ASTM F3411-22a
//
// 每条消息 25 字节: [Header:1B] [Payload:24B]
func (p *RemoteIDParser) parseASTMBeaconMessages(payload []byte) []types.DroneMessage {
	var messages []types.DroneMessage

	offset := 0
	for offset+msgSize <= len(payload) {
		msgData := payload[offset : offset+msgSize]
		// 高4位=消息类型, 低4位=协议版本
		msgType := (msgData[0] >> 4) & 0x0F
		protoVer := msgData[0] & 0x0F

		var messageType string
		var data map[string]string
		var standard string

		if protoVer == gbProtocolVersion {
			// GB 42590-2023 格式
			messageType, data = p.decodeGBMessage(msgData, msgType)
			standard = "GB 42590-2023"
		} else {
			// ASTM F3411-22a 格式（默认，protoVer==2 或未知）
			messageType, data = p.decodeASTMMessage(msgData, msgType)
			standard = "ASTM F3411-22a"
		}

		if messageType != "" {
			messages = append(messages, types.DroneMessage{
				MessageType: messageType,
				Standard:    standard,
				Data:        data,
				Source:      "ASTM",
			})
		}
		offset += msgSize
	}

	return messages
}

// ========== ASTM F3411-22a 消息解码 ==========

// decodeASTMMessage 解码 ASTM 单条消息
// msgData 是完整的 25 字节消息:
//   msgData[0] 高4位=消息类型, 低4位=协议版本
//   msgData[1:] = 24字节载荷（与参考代码 data[1:] 对应）
func (p *RemoteIDParser) decodeASTMMessage(msgData []byte, msgType uint8) (string, map[string]string) {
	payload := msgData[1:] // 跳过 Header 字节，24字节载荷
	data := make(map[string]string)
	var messageType string

	switch msgType {
	case 0: // Basic ID
		messageType = "basic_id"
		// payload[0]: 高4位=ID Type, 低4位=UA Type
		// （注意：参考代码是 type_byte >> 4 = id_type, type_byte & 0x0F = uav_type）
		idType := (payload[0] >> 4) & 0x0F
		uaType := payload[0] & 0x0F
		data["uas_id"] = cleanString(payload[1:21], "UNKNOWN")
		data["ua_type"] = getASTMUATypeName(uaType)
		data["id_type"] = getASTMIDTypeName(idType)

	case 1: // Location
		// payload[0]: Status(高4位) | Reserved | HeightType | EWDirection | SpeedMult
		// payload[1]: Direction
		// payload[2]: SpeedHorizontal
		// payload[3]: SpeedVertical
		// payload[4:8]: Latitude(int32 LE)
		// payload[8:12]: Longitude(int32 LE)
		// payload[12:14]: AltitudeBaro(uint16 LE)
		// payload[14:16]: AltitudeGeo(uint16 LE)
		// payload[16:18]: Height(uint16 LE)
		// payload[18]: VertAccuracy(高4位) | HorizAccuracy(低4位)
		// payload[19]: BaroAccuracy(高4位) | SpeedAccuracy(低4位)
		// payload[20:22]: TimeStamp(uint16 LE)
		messageType = "location"
		p.decodeASTMLocation(payload, data)

	case 2: // Authentication
		messageType = "authentication"

	case 3: // Self ID
		messageType = "self_id"
		// payload[0]: Description Type
		// payload[1:24]: Description (23 bytes)
		data["description"] = cleanString(payload[1:24], "")

	case 4: // System
		messageType = "system"
		p.decodeASTMSystem(payload, data)

	case 5: // Operator ID
		messageType = "operator_id"
		// payload[0]: Operator ID Type
		// payload[1:21]: Operator ID (20 bytes)
		data["operator_id"] = cleanString(payload[1:21], "")

	default:
		// 未知消息类型
	}

	return messageType, data
}

// decodeASTMLocation 解码 ASTM Location 消息
// payload 是 msgData[1:]（24字节），对应参考代码的 data[1:]
// payload[0]=Status, payload[1]=Direction, ..., payload[20:22]=TimeStamp
func (p *RemoteIDParser) decodeASTMLocation(payload []byte, data map[string]string) {
	if len(payload) < 22 {
		return
	}
	speedMult := payload[0] & 0x01
	ewDirection := (payload[0] >> 1) & 0x01
	heightType := (payload[0] >> 2) & 0x01
	status := (payload[0] >> 4) & 0x0F

	direction := float64(payload[1])
	if ewDirection == 1 {
		direction += 180.0
	}

	speedMultFactor := 0.25
	if speedMult == 1 {
		speedMultFactor = 0.75
	}
	speedH := float64(payload[2]) * speedMultFactor
	speedV := float64(int8(payload[3])-64) * 0.5

	lat := float64(int32(binary.LittleEndian.Uint32(payload[4:8]))) / 10000000.0
	lon := float64(int32(binary.LittleEndian.Uint32(payload[8:12]))) / 10000000.0
	altBaro := float64(binary.LittleEndian.Uint16(payload[12:14]))*0.5 - 1000.0
	altGeo := float64(binary.LittleEndian.Uint16(payload[14:16]))*0.5 - 1000.0
	height := float64(binary.LittleEndian.Uint16(payload[16:18])) * 0.5

	data["status"] = getASTMStatusName(status)
	data["direction"] = fmt.Sprintf("%.2f", direction)
	data["speed_h"] = fmt.Sprintf("%.2f", speedH)
	data["speed_v"] = fmt.Sprintf("%.2f", speedV)
	if math.Abs(lat) < 90.0 {
		data["latitude"] = fmt.Sprintf("%.7f", lat)
	}
	if math.Abs(lon) < 180.0 {
		data["longitude"] = fmt.Sprintf("%.7f", lon)
	}
	data["altitude_baro"] = fmt.Sprintf("%.2f", altBaro)
	data["altitude_geo"] = fmt.Sprintf("%.2f", altGeo)
	data["height_m"] = fmt.Sprintf("%.2f", height)
	if heightType == 0 {
		data["height_type"] = "AboveTakeoff"
	} else {
		data["height_type"] = "AboveGround"
	}
	data["h_accuracy"] = getASTMHorizontalAccuracy(payload[18] & 0x0F)
	data["v_accuracy"] = getASTMVerticalAccuracy((payload[18] >> 4) & 0x0F)
	data["baro_accuracy"] = getASTMVerticalAccuracy((payload[19] >> 4) & 0x0F)
	data["s_accuracy"] = getASTMSpeedAccuracy(payload[19] & 0x0F)
	data["timestamp"] = fmt.Sprintf("%.1f", float64(binary.LittleEndian.Uint16(payload[20:22]))*0.1)
}

// decodeASTMSystem 解码 ASTM System 消息
// payload 是 msgData[1:]（24字节），对应参考代码的 data[1:]
// payload[0]=flags, payload[1:5]=opLat, payload[5:9]=opLon, ...
func (p *RemoteIDParser) decodeASTMSystem(payload []byte, data map[string]string) {
	if len(payload) < 22 {
		return
	}
	// payload[0]: 高4位=OperatorLocationType(2bits) | 低2位=ClassificationType
	opLocType := (payload[0] >> 2) & 0x03
	classification := payload[0] & 0x03

	opLat := float64(int32(binary.LittleEndian.Uint32(payload[1:5]))) / 10000000.0
	opLon := float64(int32(binary.LittleEndian.Uint32(payload[5:9]))) / 10000000.0
	areaCount := binary.LittleEndian.Uint16(payload[9:11])
	areaRadius := payload[11]
	areaCeiling := float64(binary.LittleEndian.Uint16(payload[12:14]))*0.5 - 1000.0
	areaFloor := float64(binary.LittleEndian.Uint16(payload[14:16]))*0.5 - 1000.0
	opAlt := float64(binary.LittleEndian.Uint16(payload[16:18]))*0.5 - 1000.0

	data["flags"] = fmt.Sprintf("0x%02X", payload[0])
	data["operator_lat"] = fmt.Sprintf("%.7f", opLat)
	data["operator_lon"] = fmt.Sprintf("%.7f", opLon)
	data["operator_alt"] = fmt.Sprintf("%.2f", opAlt)
	data["area_count"] = fmt.Sprintf("%d", areaCount)
	data["area_radius_m"] = fmt.Sprintf("%d", areaRadius)
	data["area_ceiling"] = fmt.Sprintf("%.2f", areaCeiling)
	data["area_floor"] = fmt.Sprintf("%.2f", areaFloor)
	data["operator_loc_type"] = getOperatorLocTypeName(opLocType)
	data["classification"] = getASTMClassificationName(classification)
}

// cleanString 清理字符串：去除空字符和不可见字符
func cleanString(b []byte, defaultVal string) string {
	s := strings.TrimRightFunc(string(b), func(r rune) bool {
		return !utf8.ValidRune(r) || r == 0
	})
	s = strings.TrimSpace(s)
	if s == "" {
		return defaultVal
	}
	return s
}

// ========== ASTM F3411-22a 辅助函数 ==========

func getASTMUATypeName(uaType uint8) string {
	names := []string{
		"None", "Aeroplane", "Helicopter/Multirotor", "Gyroplane",
		"VTOL", "Ornithopter", "Glider", "Kite",
		"FreeBalloon", "CaptiveBalloon", "Airship",
		"FreeFall/Parachute", "Rocket", "TetheredPowered",
		"GroundObstacle", "Other",
	}
	if int(uaType) < len(names) {
		return names[uaType]
	}
	return fmt.Sprintf("Unknown(%d)", uaType)
}

func getASTMIDTypeName(idType uint8) string {
	names := []string{
		"None", "SerialNumber", "CAARegistrationID",
		"UTMID", "SpecificSessionID", "Other",
	}
	if int(idType) < len(names) {
		return names[idType]
	}
	return fmt.Sprintf("Unknown(%d)", idType)
}

func getASTMStatusName(status uint8) string {
	names := []string{
		"Undeclared", "Ground", "Airborne",
		"Emergency", "RemoteIDSystemFailure",
		"Emergency(EU)", "Reserved6", "Reserved7",
	}
	if int(status) < len(names) {
		return names[status]
	}
	return fmt.Sprintf("Unknown(%d)", status)
}

func getASTMClassificationName(classification uint8) string {
	names := []string{
		"Undeclared", "EU", "Reserved2", "Reserved3",
		"Reserved4", "Reserved5", "Reserved6", "Reserved7",
	}
	if int(classification) < len(names) {
		return names[classification]
	}
	return fmt.Sprintf("Unknown(%d)", classification)
}

func getOperatorLocTypeName(opLocType uint8) string {
	switch opLocType {
	case 0:
		return "TakeOff"
	case 1:
		return "Dynamic"
	case 2:
		return "Fixed"
	default:
		return fmt.Sprintf("Reserved(%d)", opLocType)
	}
}

func getASTMHorizontalAccuracy(acc uint8) string {
	switch acc {
	case 0:
		return "Unknown"
	case 1:
		return "<10m"
	case 2:
		return "<3m"
	case 3:
		return "<1m"
	default:
		return fmt.Sprintf("Unknown(%d)", acc)
	}
}

func getASTMVerticalAccuracy(acc uint8) string {
	switch acc {
	case 0:
		return "Unknown"
	case 1:
		return "<10m"
	case 2:
		return "<3m"
	case 3:
		return "<1m"
	default:
		return fmt.Sprintf("Unknown(%d)", acc)
	}
}

func getASTMSpeedAccuracy(acc uint8) string {
	switch acc {
	case 0:
		return "Unknown"
	case 1:
		return "<10m/s"
	case 2:
		return "<3m/s"
	case 3:
		return "<1m/s"
	case 4:
		return "<0.3m/s"
	default:
		return fmt.Sprintf("Unknown(%d)", acc)
	}
}

// ========== GB 42590-2023 消息解码 ==========
//
// GB 42590-2023 与 ASTM F3411-22a 使用相同的 OUI (FA:0B:BC + 0x0D)，
// 但通过 Header 字节的低 4 位区分：GB=0x1, ASTM=0x2。
//
// 关键差异 — 字段偏移：
//   ASTM Location: payload[0]=Status, [1]=Direction, [2]=SpeedH, [3]=SpeedV, [4:8]=Lat, [8:12]=Lon, [12:14]=AltBaro
//   GB   Location: payload[0]=Flags(Status+DirHigh), [1]=DirLow, [2]=SpeedH, [3]=SpeedV, [4:8]=Lat, [8:12]=Lon, [12:14]=AltBaro
//   （GB 和 ASTM 的 Location 字段偏移实际相同，差异在于方向编码方式）
//
//   ASTM System:   payload[0]=Flags, [1:5]=OpLat, [5:9]=OpLon, ...
//   GB   System:   payload[0]=Flags, [1:5]=OpLat, [5:9]=OpLon, ...
//   （GB 和 ASTM 的 System 字段偏移相同）
//
// GB 方向编码：12位(高4+低8) * 360.0 / 65535.0
// ASTM 方向编码：8位 0-180 + EW 标志位

// decodeGBMessage 解码 GB 42590-2023 单条消息
// msgData 是完整的 25 字节消息: msgData[0]=Header, msgData[1:]=24字节载荷
func (p *RemoteIDParser) decodeGBMessage(msgData []byte, msgType uint8) (string, map[string]string) {
	payload := msgData[1:] // 跳过 Header 字节，24字节载荷
	data := make(map[string]string)
	var messageType string

	switch msgType {
	case 0: // Basic ID
		messageType = "basic_id"
		// GB Basic ID 与 ASTM 结构相同: payload[0] 高4位=ID Type, 低4位=UA Type
		idType := (payload[0] >> 4) & 0x0F
		uaType := payload[0] & 0x0F
		data["uas_id"] = cleanString(payload[1:21], "UNKNOWN")
		data["ua_type"] = getGBUATypeName(uaType)
		data["id_type"] = getGBIDTypeName(idType)

	case 1: // Location
		messageType = "location"
		p.decodeGBLocation(payload, data)

	case 2: // Authentication
		messageType = "authentication"

	case 3: // Self ID
		messageType = "self_id"
		data["description"] = cleanString(payload[1:24], "")

	case 4: // System
		messageType = "system"
		p.decodeGBSystem(payload, data)

	case 5: // Operator ID
		messageType = "operator_id"
		data["operator_id"] = cleanString(payload[1:21], "")

	default:
		// 未知消息类型
	}

	return messageType, data
}

// decodeGBLocation 解码 GB 42590-2023 Location 消息
//
// GB Location 24字节载荷结构（与 ASTM 字段偏移相同，但方向编码方式不同）：
//
//	payload[0]: Status(高4位) | DirectionHigh(低4位)
//	payload[1]: DirectionLow
//	payload[2]: SpeedHorizontal
//	payload[3]: SpeedVertical
//	payload[4:8]: Latitude(int32 LE / 1e7)
//	payload[8:12]: Longitude(int32 LE / 1e7)
//	payload[12:14]: AltitudeBaro(uint16 LE * 0.5 - 1000)
//	payload[14:16]: AltitudeGeo(uint16 LE * 0.5 - 1000)
//	payload[16:18]: Height(uint16 LE * 0.5)
//	payload[18]: VertAccuracy(高4位) | HorizAccuracy(低4位)
//	payload[19]: BaroAccuracy(高4位) | SpeedAccuracy(低4位)
//	payload[20:22]: TimeStamp(uint16 LE * 0.1)
func (p *RemoteIDParser) decodeGBLocation(payload []byte, data map[string]string) {
	if len(payload) < 22 {
		return
	}

	status := (payload[0] >> 4) & 0x0F

	// GB 方向编码：12位 (高4位在 payload[0] 低4位，低8位在 payload[1])
	directionHigh := payload[0] & 0x0F
	directionLow := payload[1]
	directionRaw := (uint16(directionHigh) << 8) | uint16(directionLow)
	var direction float64
	if directionRaw != 0xFFFF {
		direction = float64(directionRaw) * 360.0 / 65535.0
	} else {
		direction = -1 // 无效方向
	}

	speedH := float64(payload[2]) * 0.25
	if payload[2] == 255 {
		speedH = -1
	}

	// GB SpeedVertical: uint8 值减去 128 得到有符号速度
	speedVRaw := int16(payload[3]) - 128
	speedV := float64(speedVRaw) * 0.5
	if payload[3] == 255 {
		speedV = -999
	}

	latRaw := int32(binary.LittleEndian.Uint32(payload[4:8]))
	var lat float64
	if latRaw != 0x7FFFFFFF {
		lat = float64(latRaw) / 10000000.0
	} else {
		lat = math.NaN()
	}

	lonRaw := int32(binary.LittleEndian.Uint32(payload[8:12]))
	var lon float64
	if lonRaw != 0x7FFFFFFF {
		lon = float64(lonRaw) / 10000000.0
	} else {
		lon = math.NaN()
	}

	altRaw := binary.LittleEndian.Uint16(payload[12:14])
	var altBaro float64
	if altRaw != 0xFFFF {
		altBaro = float64(altRaw)*0.5 - 1000.0
	} else {
		altBaro = math.NaN()
	}

	altGeoRaw := binary.LittleEndian.Uint16(payload[14:16])
	var altGeo float64
	if altGeoRaw != 0xFFFF {
		altGeo = float64(altGeoRaw)*0.5 - 1000.0
	} else {
		altGeo = math.NaN()
	}

	heightRaw := binary.LittleEndian.Uint16(payload[16:18])
	height := float64(heightRaw) * 0.5

	data["status"] = getGBStatusName(status)
	if direction >= 0 {
		data["direction"] = fmt.Sprintf("%.2f", direction)
	}
	if speedH >= 0 {
		data["speed_h"] = fmt.Sprintf("%.2f", speedH)
	}
	if speedV > -999 {
		data["speed_v"] = fmt.Sprintf("%.2f", speedV)
	}
	if !math.IsNaN(lat) && math.Abs(lat) < 90.0 {
		data["latitude"] = fmt.Sprintf("%.7f", lat)
	}
	if !math.IsNaN(lon) && math.Abs(lon) < 180.0 {
		data["longitude"] = fmt.Sprintf("%.7f", lon)
	}
	if !math.IsNaN(altBaro) {
		data["altitude_baro"] = fmt.Sprintf("%.2f", altBaro)
	}
	if !math.IsNaN(altGeo) {
		data["altitude_geo"] = fmt.Sprintf("%.2f", altGeo)
	}
	data["height_m"] = fmt.Sprintf("%.2f", height)
	// GB 没有 height_type 字段，默认 AboveTakeoff
	data["height_type"] = "AboveTakeoff"
	data["h_accuracy"] = getGBHorizontalAccuracy(payload[18] & 0x0F)
	data["v_accuracy"] = getGBVerticalAccuracy((payload[18] >> 4) & 0x0F)
	data["baro_accuracy"] = getGBVerticalAccuracy((payload[19] >> 4) & 0x0F)
	data["s_accuracy"] = getGBSpeedAccuracy(payload[19] & 0x0F)
	data["timestamp"] = fmt.Sprintf("%.1f", float64(binary.LittleEndian.Uint16(payload[20:22]))*0.1)
}

// decodeGBSystem 解码 GB 42590-2023 System 消息
//
// GB System 24字节载荷结构：
//
//	payload[0]: OperatorLocationType(高2位) | Classification(低3位)
//	payload[1:5]: OperatorLatitude(int32 LE / 1e7)
//	payload[5:9]: OperatorLongitude(int32 LE / 1e7)
//	payload[9:11]: AreaCount(uint16 LE)
//	payload[11]: AreaRadius(uint8)
//	payload[12:14]: AreaCeiling(uint16 LE * 0.5 - 1000)
//	payload[14:16]: AreaFloor(uint16 LE * 0.5 - 1000)
//	payload[16:18]: OperatorAltitude(uint16 LE * 0.5 - 1000)
func (p *RemoteIDParser) decodeGBSystem(payload []byte, data map[string]string) {
	if len(payload) < 18 {
		return
	}
	opLocType := (payload[0] >> 2) & 0x03
	classification := payload[0] & 0x07

	opLat := float64(int32(binary.LittleEndian.Uint32(payload[1:5]))) / 10000000.0
	opLon := float64(int32(binary.LittleEndian.Uint32(payload[5:9]))) / 10000000.0
	areaCount := binary.LittleEndian.Uint16(payload[9:11])
	areaRadius := payload[11]
	areaCeiling := float64(binary.LittleEndian.Uint16(payload[12:14]))*0.5 - 1000.0
	areaFloor := float64(binary.LittleEndian.Uint16(payload[14:16]))*0.5 - 1000.0
	opAlt := float64(binary.LittleEndian.Uint16(payload[16:18]))*0.5 - 1000.0

	data["flags"] = fmt.Sprintf("0x%02X", payload[0])
	data["operator_lat"] = fmt.Sprintf("%.7f", opLat)
	data["operator_lon"] = fmt.Sprintf("%.7f", opLon)
	data["operator_alt"] = fmt.Sprintf("%.2f", opAlt)
	data["area_count"] = fmt.Sprintf("%d", areaCount)
	data["area_radius_m"] = fmt.Sprintf("%d", areaRadius)
	data["area_ceiling"] = fmt.Sprintf("%.2f", areaCeiling)
	data["area_floor"] = fmt.Sprintf("%.2f", areaFloor)
	data["operator_loc_type"] = getGBOperatorLocTypeName(opLocType)
	data["classification"] = getGBClassificationName(classification)
}

// ========== GB 42590-2023 辅助函数 ==========

func getGBUATypeName(uaType uint8) string {
	names := []string{
		"None/NotDeclared", "Aeroplane/FixedWing", "Helicopter/Multirotor", "Gyroplane",
		"HybridLift", "Ornithopter", "Glider", "Kite",
		"FreeBalloon", "CaptiveBalloon", "Airship",
		"FreeFall/Parachute", "Rocket", "TetheredPowered",
		"GroundObstacle", "Other",
	}
	if int(uaType) < len(names) {
		return names[uaType]
	}
	return fmt.Sprintf("Unknown(%d)", uaType)
}

func getGBIDTypeName(idType uint8) string {
	names := []string{
		"None", "SerialNumber", "CAARegistrationID",
		"UTMAssignedUUID", "SpecificSessionID",
	}
	if int(idType) < len(names) {
		return names[idType]
	}
	return fmt.Sprintf("Unknown(%d)", idType)
}

func getGBStatusName(status uint8) string {
	names := []string{
		"Undeclared", "Ground", "Airborne",
		"Emergency", "RemoteIDSystemFailure",
	}
	if int(status) < len(names) {
		return names[status]
	}
	return fmt.Sprintf("Unknown(%d)", status)
}

func getGBClassificationName(classification uint8) string {
	names := []string{
		"Undefined", "USA", "China",
		"EU", "UK", "Japan", "Australia", "Other",
	}
	if int(classification) < len(names) {
		return names[classification]
	}
	return fmt.Sprintf("Unknown(%d)", classification)
}

func getGBOperatorLocTypeName(opLocType uint8) string {
	switch opLocType {
	case 0:
		return "TakeOff"
	case 1:
		return "Dynamic"
	case 2:
		return "Fixed"
	default:
		return fmt.Sprintf("Reserved(%d)", opLocType)
	}
}

func getGBHorizontalAccuracy(acc uint8) string {
	switch acc {
	case 0:
		return "Unknown"
	case 1:
		return "<10m"
	case 2:
		return "<3m"
	case 3:
		return "<1m"
	default:
		return fmt.Sprintf("Unknown(%d)", acc)
	}
}

func getGBVerticalAccuracy(acc uint8) string {
	switch acc {
	case 0:
		return "Unknown"
	case 1:
		return "<10m"
	case 2:
		return "<3m"
	case 3:
		return "<1m"
	default:
		return fmt.Sprintf("Unknown(%d)", acc)
	}
}

func getGBSpeedAccuracy(acc uint8) string {
	switch acc {
	case 0:
		return "Unknown"
	case 1:
		return "<10m/s"
	case 2:
		return "<3m/s"
	case 3:
		return "<1m/s"
	case 4:
		return "<0.3m/s"
	default:
		return fmt.Sprintf("Unknown(%d)", acc)
	}
}

// ========== GB 46750-2023 消息解码 ==========
//
// GB 46750-2023 使用与 ASTM/GB 42590 相同的 OUI (FA:0B:BC + 0x0D)，
// 但采用完全不同的变长自定义消息格式。
//
// Wire 格式:
//
//	[MessageCounter:1B] [0xFF:1B] [版本号:1B] [数据内容长度:1B] [数据标识:3B] [数据内容:变长]
//
// 版本号: 高3位=主版本(0x1), 低5位=子版本
// 数据标识: 固定 3 字节，每位表示一个数据项是否存在（bit7→bit1，bit0=扩展标志）
// 数据内容: 按标识位从高到低、标识字节从左到右的顺序排列
//
// 数据项定义（21 项）：
//
//	标识字节1: 001 唯一产品识别码(20B) / 002 实名登记标志(8B) / 003 运行类别(1B) /
//	          004 无人机分类(1B) / 005 遥控站位置类型(1B) / 006 遥控站位置(8B) / 007 遥控站高度(2B)
//	标识字节2: 008 无人机位置(8B) / 009 航迹角(2B) / 010 地速(2B) / 011 相对高度(2B) /
//	          012 垂直速度(1B) / 013 大地高度(2B) / 014 气压高度(2B)
//	标识字节3: 015 运行状态(1B) / 016 坐标系类型(1B) / 017 水平精度(1B) /
//	          018 垂直精度(1B) / 019 速度精度(1B) / 020 时间戳(6B) / 021 时间戳精度(1B)

// parseGB46750Payload 解析 GB 46750-2023 格式的 payload
// payload 从 Message Counter 开始: [MsgCounter:1B] [0xFF:1B] [版本号:1B] [数据长度:1B] [数据标识:3B] [数据内容:变长]
// 返回一个 DroneMessage，包含所有解析出的数据项
func (p *RemoteIDParser) parseGB46750Payload(payload []byte) []types.DroneMessage {
	if len(payload) < 7 {
		return nil
	}

	version := payload[2]
	majorVer := (version >> 5) & 0x07
	minorVer := version & 0x1F

	contentLen := int(payload[3]) // 数据内容长度（字节数）
	flags := payload[4:7]         // 3 字节数据标识
	content := payload[7:]        // 数据内容起始

	if len(content) < contentLen {
		contentLen = len(content)
	}

	data := make(map[string]string)
	data["gb46750_version"] = fmt.Sprintf("%d.%d", majorVer, minorVer)
	data["msg_counter"] = fmt.Sprintf("%d", payload[0])

	// 解析数据标识位表
	p.decodeGB46750Fields(flags, content[:contentLen], data)

	return []types.DroneMessage{
		{
			MessageType: "gb46750",
			Standard:    "GB 46750-2023",
			Data:        data,
			Source:      "ASTM",
		},
	}
}

// decodeGB46750Fields 根据数据标识位表解析 GB 46750 数据内容
// flags: 3 字节数据标识
// content: 数据内容字节
// data: 输出 map
func (p *RemoteIDParser) decodeGB46750Fields(flags []byte, content []byte, data map[string]string) {
	if len(flags) < 3 {
		return
	}

	offset := 0
	contentLen := len(content)

	// 按标识字节顺序解析（3 个字节）
	for byteIdx := 0; byteIdx < 3 && byteIdx < len(flags); byteIdx++ {
		flag := flags[byteIdx]

		// 按位从高到低 (bit7→bit1)，bit0 是扩展标志位
		for bit := 7; bit >= 1; bit-- {
			if (flag & (1 << bit)) == 0 {
				continue
			}
			if offset >= contentLen {
				return
			}

			switch {
			case byteIdx == 0:
				// ---- 标识字节1 ----
				switch bit {
				case 7: // 0x80: 001 唯一产品识别码 (M) — 20字节 ASCII 大端序
					if offset+20 <= contentLen {
						data["unique_id"] = cleanString(content[offset:offset+20], "")
						offset += 20
					}
				case 6: // 0x40: 002 实名登记标志 (M) — 8字节 ASCII 大端序
					if offset+8 <= contentLen {
						data["realname_id"] = cleanString(content[offset:offset+8], "")
						offset += 8
					}
				case 5: // 0x20: 003 运行类别 (O) — 1 字节
					data["operation_category"] = fmt.Sprintf("%d", content[offset])
					offset += 1
				case 4: // 0x10: 004 无人机分类 (M) — 1 字节
					data["ua_category"] = getGB46750UACategoryName(content[offset])
					offset += 1
				case 3: // 0x08: 005 遥控站位置类型 (M) — 1 字节
					data["rcs_loc_type"] = getGB46750RCSLocTypeName(content[offset])
					offset += 1
			case 2: // 0x04: 006 遥控站位置 (M) — 8字节 LE int32×1e7 (lon|lat)
				if offset+8 <= contentLen {
					rcsLon := float64(int32(binary.LittleEndian.Uint32(content[offset:offset+4]))) / 10000000.0
					rcsLat := float64(int32(binary.LittleEndian.Uint32(content[offset+4:offset+8]))) / 10000000.0
					if math.Abs(rcsLon) < 180.0 {
						data["rcs_longitude"] = fmt.Sprintf("%.7f", rcsLon)
					}
					if math.Abs(rcsLat) < 90.0 {
						data["rcs_latitude"] = fmt.Sprintf("%.7f", rcsLat)
					}
						offset += 8
					}
				case 1: // 0x02: 007 遥控站高度 (M) — 2字节 LE (val+1000)×2
					if offset+2 <= contentLen {
						raw := binary.LittleEndian.Uint16(content[offset : offset+2])
						if raw != 0 {
							alt := float64(raw)/2.0 - 1000.0
							data["rcs_altitude"] = fmt.Sprintf("%.2f", alt)
						}
						offset += 2
					}
				}

			case byteIdx == 1:
				// ---- 标识字节2 ----
				switch bit {
			case 7: // 0x80: 008 无人机位置 (M) — 8字节 LE int32×1e7 (lon|lat)
				if offset+8 <= contentLen {
					uavLon := float64(int32(binary.LittleEndian.Uint32(content[offset:offset+4]))) / 10000000.0
					uavLat := float64(int32(binary.LittleEndian.Uint32(content[offset+4:offset+8]))) / 10000000.0
					if math.Abs(uavLon) < 180.0 {
						data["longitude"] = fmt.Sprintf("%.7f", uavLon)
					}
					if math.Abs(uavLat) < 90.0 {
						data["latitude"] = fmt.Sprintf("%.7f", uavLat)
					}
						offset += 8
					}
				case 6: // 0x40: 009 航迹角 (M) — 2字节 LE uint16 × 0.1° 分辨率
					if offset+2 <= contentLen {
						raw := binary.LittleEndian.Uint16(content[offset : offset+2])
						if raw != 0xFFFF {
							angle := float64(raw) / 10.0
							data["direction"] = fmt.Sprintf("%.2f", angle)
						}
						offset += 2
					}
				case 5: // 0x20: 010 地速 (M) — 2字节 LE uint16 × 0.1m/s 分辨率
					if offset+2 <= contentLen {
						raw := binary.LittleEndian.Uint16(content[offset : offset+2])
						if raw != 0xFFFF {
							speed := float64(raw) / 10.0
							data["speed_h"] = fmt.Sprintf("%.2f", speed)
						}
						offset += 2
					}
				case 4: // 0x10: 011 相对高度 (O) — 2字节 LE (val+9000)×2
					if offset+2 <= contentLen {
						raw := binary.LittleEndian.Uint16(content[offset : offset+2])
						if raw != 0 {
							height := float64(raw)/2.0 - 9000.0
							data["height_m"] = fmt.Sprintf("%.2f", height)
							data["height_type"] = "AboveTakeoff"
						}
						offset += 2
					}
				case 3: // 0x08: 012 垂直速度 (O) — 1字节，bit7=方向(0上升/1下降)，bit6-0=实际值×2
					raw := content[offset]
					if raw != 0xFF {
						v := float64(raw&0x7F) / 2.0
						if (raw & 0x80) != 0 {
							v = -v
						}
						data["speed_v"] = fmt.Sprintf("%.2f", v)
					}
					offset += 1
				case 2: // 0x04: 013 大地高度 (M) — 2字节 LE (val+1000)×2
					if offset+2 <= contentLen {
						raw := binary.LittleEndian.Uint16(content[offset : offset+2])
						if raw != 0 {
							alt := float64(raw)/2.0 - 1000.0
							data["altitude_geo"] = fmt.Sprintf("%.2f", alt)
						}
						offset += 2
					}
				case 1: // 0x02: 014 气压高度 (O) — 2字节 LE (val+1000)×2
					if offset+2 <= contentLen {
						raw := binary.LittleEndian.Uint16(content[offset : offset+2])
						if raw != 0 {
							alt := float64(raw)/2.0 - 1000.0
							data["altitude_baro"] = fmt.Sprintf("%.2f", alt)
						}
						offset += 2
					}
				}

			case byteIdx == 2:
				// ---- 标识字节3 ----
				switch bit {
				case 7: // 0x80: 015 运行状态 (M) — 1 字节
					data["status"] = getGB46750StatusName(content[offset])
					offset += 1
				case 6: // 0x40: 016 坐标系类型 (M) — 1 字节
					data["coord_system"] = getGB46750CoordSystemName(content[offset])
					offset += 1
				case 5: // 0x20: 017 水平精度 (M) — 1 字节
					data["h_accuracy"] = getGB46750AccuracyName(content[offset])
					offset += 1
				case 4: // 0x10: 018 垂直精度 (M) — 1 字节
					data["v_accuracy"] = getGB46750AccuracyName(content[offset])
					offset += 1
				case 3: // 0x08: 019 速度精度 (M) — 1 字节
					data["s_accuracy"] = getGB46750AccuracyName(content[offset])
					offset += 1
				case 2: // 0x04: 020 时间戳 (M) — 6字节 LE Unix 毫秒时间戳
					if offset+6 <= contentLen {
						var ts uint64
						for i := 0; i < 6; i++ {
							ts |= uint64(content[offset+i]) << (i * 8)
						}
						data["timestamp_ms"] = fmt.Sprintf("%d", ts)
						// 同时转换为秒级时间戳字符串（用于 location timestamp）
						data["timestamp"] = fmt.Sprintf("%.1f", float64(ts)/1000.0)
						offset += 6
					}
				case 1: // 0x02: 021 时间戳精度 (M) — 1 字节
					data["ts_accuracy"] = getGB46750TSAccuracyName(content[offset])
					offset += 1
				}
			}
		}
	}
}

// ========== GB 46750-2023 辅助函数 ==========

func getGB46750UACategoryName(cat uint8) string {
	// GB 46750 民用无人驾驶航空器分类
	names := []string{
		"微型(0)", "轻型(1)", "小型(2)", "中型(3)", "大型(4)",
	}
	if int(cat) < len(names) {
		return names[cat]
	}
	return fmt.Sprintf("Unknown(%d)", cat)
}

func getGB46750RCSLocTypeName(t uint8) string {
	switch t {
	case 0:
		return "TakeOff"
	case 1:
		return "Dynamic"
	case 2:
		return "Fixed"
	default:
		return fmt.Sprintf("Reserved(%d)", t)
	}
}

func getGB46750StatusName(status uint8) string {
	switch status {
	case 0:
		return "Undeclared"
	case 1:
		return "Ground"
	case 2:
		return "Airborne"
	case 3:
		return "Emergency"
	default:
		return fmt.Sprintf("Unknown(%d)", status)
	}
}

func getGB46750CoordSystemName(cs uint8) string {
	switch cs {
	case 0:
		return "WGS84"
	case 1:
		return "CGCS2000"
	default:
		return fmt.Sprintf("Unknown(%d)", cs)
	}
}

func getGB46750AccuracyName(acc uint8) string {
	switch acc {
	case 0:
		return "Unknown"
	case 1:
		return "<10m"
	case 2:
		return "<3m"
	case 3:
		return "<1m"
	case 4:
		return "<0.3m"
	default:
		return fmt.Sprintf("Unknown(%d)", acc)
	}
}

func getGB46750TSAccuracyName(acc uint8) string {
	switch acc {
	case 0:
		return "Unknown"
	case 1:
		return "<0.1s"
	case 2:
		return "<0.2s"
	case 3:
		return "<0.3s"
	case 4:
		return "<0.4s"
	case 5:
		return "<0.5s"
	case 6:
		return "<1s"
	case 7:
		return "<2s"
	case 8:
		return "<3s"
	case 9:
		return "<4s"
	case 10:
		return "<5s"
	default:
		return fmt.Sprintf("Unknown(%d)", acc)
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
