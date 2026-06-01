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
//  1. ASTM F3411-22a / ASD-STAN prEN 4709-002（国际标准）
//     - WiFi Beacon: Vendor Specific IE (Element ID 0xDD)
//       OUI: FA:0B:BC (ASD-STAN), OUI_Type: 0x0D
//     - WiFi NAN: NAN Service Discovery Frame
//       OUI: 50:6F:9A (Wi-Fi Alliance)
//     - 报头: 高4位=协议版本(固定2), 低4位=消息类型(0-5)
//
//  2. GB42590-2023（中国国家标准，WiFi Beacon 广播式）
//     参考: GB42590-2023 附录A, GB46750-2025
//     - 传输层: Vendor Specific IE, OUI: FA:0B:BC, OUI_Type: 0x0D
//       OUI 后紧跟 1 字节 Message Counter (0-255 循环计数)
//     - 报头: 高4位=报文类型(0x0-0xF), 低4位=接口版本(固定0x1)
//     - 支持 Packed 格式(0xF) 打包多条报文
//
//  3. GB46750-2025（中国国家标准，网络式传输）
//     - 使用 TLV 风格数据包，通过位掩码标识数据项
//     - 格式: [0xFF] [版本号] [数据长度] [数据标识(可变)] [数据内容项...]
//
// ASTM 和 GB42590 Beacon 的差异：
//   - 报头定义不同: ASTM 高4位=协议版本, GB42590 高4位=报文类型
//   - Basic ID byte 1 nibble 分配不同
//   - System 消息字段布局不同
//   - GB42590 支持 Packed 格式 (0xFx)
//   - Location byte 1 位布局相同 (SpeedMult/EWDir/HeightType/Status)

const (
	// 通用：ASTM Beacon / GB42590 共享的 Vendor Specific IE OUI
	asdStanOUI     = "\xFA\x0B\xBC" // ASD-STAN OUI
	asdStanOUIType = 0x0D           // ASD-STAN OUI Type
	msgSize        = 25             // 每条 Remote ID 消息固定 25 字节

	// 旧版 ASTM / OpenDroneID 临时 OUI（部分 ESP32 早期实现使用）
	// 参考：Wi-Fi Alliance 早期分配的临时 OUI，esp32-open-droneid 等开源项目使用
	legacyASTMOUI     = "\x06\x05\x04" // 旧版 ASTM OUI
	legacyASTMOUIType = 0xFD           // 旧版 ASTM OUI Type

	// ASTM 专属常量
	astmProtocolVersion = 2 // ASTM F3411-22a 协议版本号（字节 0 高 4 位）

	// GB42590 专属常量
	gb42590InterfaceVersion = 0x1 // GB42590 接口版本（字节 0 低 4 位，固定 0x1）
	gb42590PackedHeader     = 0xF // Packed 格式报文类型（高 4 位 = 0xF）
)

type RemoteIDParser struct{}

func NewParser() *RemoteIDParser {
	return &RemoteIDParser{}
}

// ParseFrame 解析 Beacon 数据帧中的 Remote ID 消息（ASTM/ASD-STAN + GB42590）
//
// 支持格式:
//   - WiFi Beacon Vendor Specific IE (OUI: FA:0B:BC + OUI_Type: 0x0D)
//   - 自动识别 ASTM F3411-22a / GB42590-2023
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
		if attrID == 0xDD && len(attrValue) >= 4 {
			// 检查 OUI 是否为 ASD-STAN (FA:0B:BC) + OUI_Type (0x0D)
			if attrValue[0] == 0xFA && attrValue[1] == 0x0B && attrValue[2] == 0xBC && attrValue[3] == 0x0D {
				// Remote ID 消息从 OUI_Type 之后开始
				remoteIDPayload := attrValue[4:]

				// 尝试按 ASTM 格式解析（NAN 中的 Remote ID 通常是 ASTM 格式）
				// 先检查是否是 GB42590 格式
				if len(remoteIDPayload) > 0 {
					lowNibble := remoteIDPayload[0] & 0x0F

					// 优先检查 GB42590（低4位=0x1），即使高4位=2也不能误判为ASTM
					if lowNibble == gb42590InterfaceVersion {
						// GB42590 格式，跳过 Message Counter
						if len(remoteIDPayload) > 1 {
							msgs := p.parseGB42590BeaconMessages(remoteIDPayload[1:])
							messages = append(messages, msgs...)
						}
					} else {
						// ASTM 格式（无 Message Counter）
						msgs := p.parseASTMBeaconMessages(remoteIDPayload)
						messages = append(messages, msgs...)
					}
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
// GB42590 附录A 定义的 Beacon 帧 Vendor Specific IE 结构:
//
//	[Element ID: 0xDD] [Len] [OUI: FA:0B:BC] [Vend Type: 0x0D] [MsgCounter: 1B] [Messages...]
//
// ASTM 的 Vendor Specific IE 结构:
//
//	[Element ID: 0xDD] [Len] [OUI: FA:0B:BC] [Vend Type: 0x0D] [Messages...]
//
// 两者 OUI 和 OUI_Type 相同，差异在于 GB42590 多了 Message Counter。
// 通过报头字节区分：ASTM 高4位=协议版本(2)，GB42590 低4位=接口版本(0x1)。
func (p *RemoteIDParser) parseVendorIE(raw []byte) ([]types.DroneMessage, error) {
	var messages []types.DroneMessage

	// 搜索 ASD-STAN OUI (FA:0B:BC) + OUI_Type (0x0D)
	for idx := 0; idx <= len(raw)-4; idx++ {
		if string(raw[idx:idx+3]) != asdStanOUI || raw[idx+3] != asdStanOUIType {
			continue
		}

		payload := raw[idx+4:]
		if len(payload) < 1 {
			continue
		}

		firstNibble := (payload[0] >> 4) & 0x0F
		lowNibble := payload[0] & 0x0F

		// 区分标准：
		//
		// GB42590 Beacon 格式在 OUI+Type 后紧跟 1 字节 Message Counter（0-255），
		// 然后是报文数据。因此 payload[0] 是 Message Counter，payload[1] 才是真正的报头。
		//
		// 关键：不能用 payload[0]（Message Counter）来判断标准！因为 Counter 值
		// 的 nibble 组合是随机的（93.75% 概率 lowNibble != 0x1），会导致误判。
		//
		// 正确做法：
		//  1. 先检查 payload[1] 是否是 GB42590 Packed (0xF1) 或 单消息 (lowNibble=0x1)
		//  2. 再检查 payload[0] 是否是 ASTM 报头 (firstNibble=2)
		//  3. 如果都不是，尝试按 GB42590（跳过 Counter）和 ASTM（不跳过）分别解析，取成功者
		if len(payload) >= 2 {
			// 检查 payload[1]（跳过可能的 Message Counter）是否为 GB42590
			nextLowNibble := payload[1] & 0x0F

			if nextLowNibble == gb42590InterfaceVersion {
				// GB42590-2023 Beacon 格式（Packed 或单消息）
				// 跳过 1 字节 Message Counter
				msgPayload := payload[1:]
				msgs := p.parseGB42590BeaconMessages(msgPayload)
				messages = append(messages, msgs...)
			} else if lowNibble == gb42590InterfaceVersion {
				// GB42590 无 Message Counter（非标准但兼容）
				msgs := p.parseGB42590BeaconMessages(payload)
				messages = append(messages, msgs...)
			} else if firstNibble == astmProtocolVersion {
				// ASTM F3411-22a Beacon 格式（无 Message Counter）
				msgs := p.parseASTMBeaconMessages(payload)
				messages = append(messages, msgs...)
			} else if firstNibble <= 5 {
				// 旧版格式：无 Message Counter，按 ASTM 尝试解析
				msgs := p.parseASTMBeaconMessages(payload)
				messages = append(messages, msgs...)
			} else {
				// 可能是 GB42590 但 Message Counter 的 nibble 刚好落在无法判断的范围
				// 尝试按 GB42590（跳过 Counter）解析
				msgPayload := payload[1:]
				msgs := p.parseGB42590BeaconMessages(msgPayload)
				if len(msgs) > 0 {
					messages = append(messages, msgs...)
				} else {
					// 回退：尝试按 ASTM 解析
					msgs = p.parseASTMBeaconMessages(payload)
					messages = append(messages, msgs...)
				}
			}
		} else {
			// payload 只有 1 字节，无法判断，按 ASTM 尝试
			msgs := p.parseASTMBeaconMessages(payload)
			messages = append(messages, msgs...)
		}
		// 如果成功解析到消息，退出搜索；否则继续查找后续 OUI
		if len(messages) > 0 {
			break
		}
	}

	// 如果没找到标准 ASD-STAN OUI，尝试搜索旧版 ASTM OUI (06:05:04 + 0xFD)
	// 部分 ESP32 开源实现使用此 OUI
	if len(messages) == 0 {
		for idx := 0; idx <= len(raw)-4; idx++ {
			if string(raw[idx:idx+3]) != legacyASTMOUI || raw[idx+3] != legacyASTMOUIType {
				continue
			}

			payload := raw[idx+4:]
			if len(payload) < 1 {
				continue
			}

			firstNibble := (payload[0] >> 4) & 0x0F

			// 旧版 ASTM 格式（无 Message Counter）
			if firstNibble == astmProtocolVersion || firstNibble <= 5 {
				msgs := p.parseASTMBeaconMessages(payload)
				messages = append(messages, msgs...)
			}
			if len(messages) > 0 {
				break
			}
		}
	}

	return messages, nil
}

// parseASTMBeaconMessages 解析 ASTM Beacon 格式的单条/多条消息
// ASTM 无 Message Counter，直接以 25 字节为单位解析
func (p *RemoteIDParser) parseASTMBeaconMessages(payload []byte) []types.DroneMessage {
	var messages []types.DroneMessage

	offset := 0
	for offset+msgSize <= len(payload) {
		msgData := payload[offset : offset+msgSize]
		msgType := msgData[0] & 0x0F

		// ASTM: 高4位=协议版本(2), 低4位=消息类型
		messageType, data := p.decodeASTMMessage(msgData, msgType)
		if messageType != "" {
			messages = append(messages, types.DroneMessage{
				MessageType: messageType,
				Standard:    "ASTM F3411-22a",
				Data:        data,
				Source:      "ASTM",
			})
		}
		offset += msgSize
	}

	return messages
}

// parseGB42590BeaconMessages 解析 GB42590 Beacon 格式的消息
// 报头: 高4位=报文类型, 低4位=接口版本(0x1)
// 报文类型 0xF = Packed, 其他 = 单条消息
func (p *RemoteIDParser) parseGB42590BeaconMessages(payload []byte) []types.DroneMessage {
	var messages []types.DroneMessage

	if len(payload) < 1 {
		return messages
	}

	msgType := (payload[0] >> 4) & 0x0F

	if msgType == gb42590PackedHeader {
		// Packed 格式 (0xFx)
		return p.parseGB42590Packed(payload)
	}

	// 单条或多条消息（25字节对齐）
	offset := 0
	for offset+msgSize <= len(payload) {
		msgData := payload[offset : offset+msgSize]
		mt := (msgData[0] >> 4) & 0x0F

		messageType, data := p.decodeGB42590Message(msgData, mt)
		if messageType != "" {
			messages = append(messages, types.DroneMessage{
				MessageType: messageType,
				Standard:    "GB42590-2023",
				Data:        data,
				Source:      "GB42590",
			})
		}
		offset += msgSize
	}

	return messages
}

// ========== ASTM F3411-22a 消息解码 ==========

// decodeASTMMessage 解码 ASTM 单条消息
// msgData[0] 高4位=协议版本(2), 低4位=消息类型
// msgData[1:] = 24字节载荷
func (p *RemoteIDParser) decodeASTMMessage(msgData []byte, msgType uint8) (string, map[string]string) {
	payload := msgData[1:]
	data := make(map[string]string)
	var messageType string

	switch msgType {
	case 0: // Basic ID
		messageType = "basic_id"
		// ASTM: UA Type (高4位), ID Type (低4位)
		uaType := (payload[0] >> 4) & 0x0F
		idType := payload[0] & 0x0F
		data["uas_id"] = cleanString(payload[1:21], "UNKNOWN")
		data["ua_type"] = getASTMUATypeName(uaType)
		data["id_type"] = getASTMIDTypeName(idType)

	case 1: // Location
		// ASTM F3411-22a Location:
		// byte 1: [7-4]Status(4) [3]Reserved [2]HeightType [1]EWDirection [0]SpeedMult
		// byte 2: Direction(1)
		// byte 3: SpeedHorizontal(1)
		// byte 4: SpeedVertical(1)
		// bytes 5-8: Latitude(int32 LE)
		// bytes 9-12: Longitude(int32 LE)
		// bytes 13-14: AltitudeBaro(uint16 LE)
		// bytes 15-16: AltitudeGeo(uint16 LE)
		// bytes 17-18: Height(uint16 LE)
		// byte 19: [7-4]VertAccuracy [3-0]HorizAccuracy
		// byte 20: [7-4]BaroAccuracy [3-0]SpeedAccuracy
		// bytes 21-22: TimeStamp(uint16 LE, 0.1s单位)
		// byte 23: [7-4]TSAccuracy [3-0]Reserved
		// byte 24: Reserved
		messageType = "location"
		p.decodeASTMLocation(payload, data)

	case 2: // Authentication
		messageType = "authentication"

	case 3: // Self ID
		messageType = "self_id"
		data["description"] = cleanString(payload[0:23], "")

	case 4: // System
		messageType = "system"
		p.decodeASTMSystem(payload, data)

	case 5: // Operator ID
		messageType = "operator_id"
		data["operator_id"] = cleanString(payload[0:20], "")

	default:
		// 未知消息类型
	}

	return messageType, data
}

// decodeASTMLocation 解码 ASTM Location 消息
func (p *RemoteIDParser) decodeASTMLocation(payload []byte, data map[string]string) {
	if len(payload) < 23 {
		return
	}
	speedMult := payload[0] & 0x01
	ewDirection := (payload[0] >> 1) & 0x01
	heightType := (payload[0] >> 2) & 0x01
	status := (payload[0] >> 4) & 0x0F

	direction := float64(payload[1]) * 360.0 / 255.0
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
func (p *RemoteIDParser) decodeASTMSystem(payload []byte, data map[string]string) {
	if len(payload) < 23 {
		return
	}
	// ASTM F3411-22a System:
	// byte 1: [7-4]Reserved [3-2]OperatorLocationType [1-0]ClassificationType
	// bytes 2-5: OperatorLatitude(int32 LE)
	// bytes 6-9: OperatorLongitude(int32 LE)
	// bytes 10-11: AreaCount(uint16 LE)
	// byte 12: AreaRadius(uint8)
	// bytes 13-14: AreaCeiling(uint16 LE, 0.5m - 1000)
	// bytes 15-16: AreaFloor(uint16 LE, 0.5m - 1000)
	// bytes 17-18: OperatorAltitude(uint16 LE, 0.5m - 1000)
	// byte 19: [7]CategoryEU [6-0]ClassEU
	// bytes 20-21: OperatorAltitudeGeo(uint16 LE, 0.5m - 1000)
	// bytes 22-24: Timestamp(uint32 LE, Unix秒)

	flags := payload[0]
	opLocType := (flags >> 2) & 0x03
	classification := flags & 0x03

	opLat := float64(int32(binary.LittleEndian.Uint32(payload[1:5]))) / 10000000.0
	opLon := float64(int32(binary.LittleEndian.Uint32(payload[5:9]))) / 10000000.0
	areaCount := binary.LittleEndian.Uint16(payload[9:11])
	areaRadius := payload[11]
	areaCeiling := float64(binary.LittleEndian.Uint16(payload[12:14]))*0.5 - 1000.0
	areaFloor := float64(binary.LittleEndian.Uint16(payload[14:16]))*0.5 - 1000.0
	opAlt := float64(binary.LittleEndian.Uint16(payload[16:18]))*0.5 - 1000.0

	data["flags"] = fmt.Sprintf("0x%02X", flags)
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

// ========== GB42590-2023 消息解码 ==========

// decodeGB42590Message 解码 GB42590 单条消息
// 报头: 高4位=报文类型, 低4位=接口版本(0x1)
// msgData[1:] = 24字节载荷
func (p *RemoteIDParser) decodeGB42590Message(msgData []byte, msgType uint8) (string, map[string]string) {
	payload := msgData[1:]
	data := make(map[string]string)
	var messageType string

	switch msgType {
	case 0x0: // 基本ID报文（静态）
		messageType = "basic_id"
		// GB42590: ID Type(高4位的高3位), UA Type(低4位)
		idType := (payload[0] >> 4) & 0x07
		uaType := payload[0] & 0x0F
		data["uas_id"] = cleanString(payload[1:21], "CAAC-UNKNOWN")
		data["ua_type"] = getGBUAtypeName(uaType)
		data["id_type"] = getGBIDTypeName(idType)

	case 0x1: // 位置向量报文（动态）
		messageType = "location"
		p.decodeGB42590Location(payload, data)

	case 0x2: // 预留
		messageType = "reserved"

	case 0x3: // 运行描述报文（静态，可选）
		messageType = "self_id"
		descType := payload[0]
		descText := cleanString(payload[1:24], "")
		data["description_type"] = getGBDescTypeName(descType)
		data["description"] = descText

	case 0x4: // 系统报文（静态）
		messageType = "system"
		p.decodeGB42590System(payload, data)

	case 0x5: // 预留
		messageType = "reserved"

	default:
		// 未知报文类型
	}

	return messageType, data
}

// decodeGB42590Location 解码 GB42590 位置向量报文
//
// 参考 GB42590-2023 表4 及 博文文档:
//
//	byte 1: [7-4]运行状态(4) [3]预留 [2]高度类型 [1]E/W方向标志 [0]速度乘数
//	byte 2: 航迹角(1, 0-179, E/W方向标志表示是否>=180)
//	byte 3: 地速(1, 速度乘数决定分辨率)
//	byte 4: 垂直速度(1, 爬升正/下降负)
//	bytes 5-8: 纬度(int32 LE, 实际值×10^7)
//	bytes 9-12: 经度(int32 LE, 实际值×10^7)
//	bytes 13-14: 气压高度(uint16 LE, (实际值+1000)×2, 0.5m分辨率)
//	bytes 15-16: 几何高度(uint16 LE, (实际值+1000)×2, 0.5m分辨率)
//	bytes 17-18: 距地高度(uint16 LE, 0.5m分辨率)
//	byte 19: [7-4]垂直精度 [3-0]水平精度
//	byte 20: [7-4]预留 [3-0]速度精度
//	bytes 21-22: 时间戳(uint16 LE, 0.1s单位, 以当前小时为起始)
//	byte 23: [7-4]预留 [3-0]时间戳精度
//	byte 24: 预留
func (p *RemoteIDParser) decodeGB42590Location(payload []byte, data map[string]string) {
	if len(payload) < 23 {
		return
	}

	// byte 1: 位布局与 ASTM 相同
	speedMult := payload[0] & 0x01
	ewDirection := (payload[0] >> 1) & 0x01
	heightType := (payload[0] >> 2) & 0x01
	status := (payload[0] >> 4) & 0x0F

	// byte 2: 航迹角 (0-179, 配合 E/W 方向标志)
	direction := float64(payload[1]) * 360.0 / 255.0
	if ewDirection == 1 {
		direction += 180.0
	}

	// byte 3: 地速
	speedMultFactor := 0.25
	if speedMult == 1 {
		speedMultFactor = 0.75
	}
	speedH := float64(payload[2]) * speedMultFactor

	// byte 4: 垂直速度
	speedV := float64(int8(payload[3])-64) * 0.5

	// bytes 5-8: 纬度 (int32 LE, /1e7)
	lat := float64(int32(binary.LittleEndian.Uint32(payload[4:8]))) / 10000000.0
	// bytes 9-12: 经度 (int32 LE, /1e7)
	lon := float64(int32(binary.LittleEndian.Uint32(payload[8:12]))) / 10000000.0
	// bytes 13-14: 气压高度
	altBaro := float64(binary.LittleEndian.Uint16(payload[12:14]))*0.5 - 1000.0
	// bytes 15-16: 几何高度
	altGeo := float64(binary.LittleEndian.Uint16(payload[14:16]))*0.5 - 1000.0
	// bytes 17-18: 距地高度
	height := float64(binary.LittleEndian.Uint16(payload[16:18])) * 0.5

	data["status"] = getGBStatusName(status)
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
	// byte 19: 精度
	data["h_accuracy"] = getGBHorizontalAccuracy(payload[18] & 0x0F)
	data["v_accuracy"] = getGBVerticalAccuracy((payload[18] >> 4) & 0x0F)
	// byte 20: 速度精度
	data["s_accuracy"] = getGBSpeedAccuracy(payload[19] & 0x0F)
	// bytes 21-22: 时间戳
	data["timestamp"] = fmt.Sprintf("%.1f", float64(binary.LittleEndian.Uint16(payload[20:22]))*0.1)
}

// decodeGB42590System 解码 GB42590 系统报文
//
// 参考 GB42590-2023 表6 及 博文文档:
//
//	byte 1: [7]预留 [6-5]坐标系类型 [4-2]等级分类归属区域 [1-0]控制站位置类型
//	bytes 2-5: 控制站纬度(int32 LE, /1e7)
//	bytes 6-9: 控制站经度(int32 LE, /1e7)
//	bytes 10-11: 运行区域计数(uint16 LE)
//	byte 12: 运行区域半径(值×10m)
//	bytes 13-14: 运行区域高度上限(uint16 LE, 几何高度, 0.5m - 1000)
//	bytes 15-16: 运行区域高度下限(uint16 LE, 几何高度, 0.5m - 1000)
//	byte 17: [7-4]UA运行类别 [3-0]UA等级
//	bytes 18-19: 控制站高度(uint16 LE, 几何高度, 0.5m - 1000)
//	bytes 20-23: 时间戳(uint32 LE, Unix秒, 自2019-01-01)
//	byte 24: 预留
func (p *RemoteIDParser) decodeGB42590System(payload []byte, data map[string]string) {
	if len(payload) < 24 {
		return
	}

	// byte 1
	flags := payload[0]
	coordSysType := (flags >> 5) & 0x03       // [6-5] 坐标系类型
	classRegion := (flags >> 2) & 0x07        // [4-2] 等级分类归属区域
	stationLocType := flags & 0x03            // [1-0] 控制站位置类型

	// bytes 2-5: 控制站纬度
	opLat := float64(int32(binary.LittleEndian.Uint32(payload[1:5]))) / 10000000.0
	// bytes 6-9: 控制站经度
	opLon := float64(int32(binary.LittleEndian.Uint32(payload[5:9]))) / 10000000.0
	// bytes 10-11: 运行区域计数
	areaCount := binary.LittleEndian.Uint16(payload[9:11])
	// byte 12: 运行区域半径 (×10m)
	areaRadius := int(payload[11]) * 10
	// bytes 13-14: 运行区域高度上限
	areaCeiling := float64(binary.LittleEndian.Uint16(payload[12:14]))*0.5 - 1000.0
	// bytes 15-16: 运行区域高度下限
	areaFloor := float64(binary.LittleEndian.Uint16(payload[14:16]))*0.5 - 1000.0
	// byte 17: [7-4]UA运行类别 [3-0]UA等级
	uaCategory := (payload[16] >> 4) & 0x0F
	uaClass := payload[16] & 0x0F
	// bytes 18-19: 控制站高度
	opAlt := float64(binary.LittleEndian.Uint16(payload[17:19]))*0.5 - 1000.0
	// bytes 20-23: 时间戳 (Unix秒, 自2019-01-01 00:00:00)
	ts := binary.LittleEndian.Uint32(payload[19:23])

	data["flags"] = fmt.Sprintf("0x%02X", flags)
	data["coord_sys_type"] = getGBCoordSysName(coordSysType)
	data["classification"] = getGBClassRegionName(classRegion)
	data["station_loc_type"] = getGBStationLocTypeName(stationLocType)
	if math.Abs(opLat) <= 90.0 && opLat != 0 {
		data["operator_lat"] = fmt.Sprintf("%.7f", opLat)
	}
	if math.Abs(opLon) <= 180.0 && opLon != 0 {
		data["operator_lon"] = fmt.Sprintf("%.7f", opLon)
	}
	data["area_count"] = fmt.Sprintf("%d", areaCount)
	data["area_radius_m"] = fmt.Sprintf("%d", areaRadius)
	data["area_ceiling"] = fmt.Sprintf("%.2f", areaCeiling)
	data["area_floor"] = fmt.Sprintf("%.2f", areaFloor)
	data["ua_category"] = getGBUACategoryName(uaCategory)
	data["ua_class"] = getGBUAClassName(uaClass)
	data["operator_alt"] = fmt.Sprintf("%.2f", opAlt)
	// Unix 时间戳转换 (自 2019-01-01)
	baseTime := int64(1546300800) // 2019-01-01 00:00:00 UTC
	data["timestamp"] = fmt.Sprintf("%d", int64(ts)+baseTime)
}

// parseGB42590Packed 解析 GB42590 Packed 格式 (报文类型 0xF)
//
// 参考 GB42590-2023 表7:
//
//	byte 1: 每条报文长度 (固定 0x19 = 25)
//	byte 2: 打包报文中包含的报文数量 (最多 10)
//	bytes 3+: N × 25 字节报文内容
func (p *RemoteIDParser) parseGB42590Packed(payload []byte) []types.DroneMessage {
	var messages []types.DroneMessage

	if len(payload) < 3 {
		return messages
	}

	// byte 1: 每条报文长度 (固定 0x19 = 25)
	msgLen := int(payload[1])
	if msgLen != msgSize {
		// 容错：如果长度不匹配，尝试按 25 解析
		if msgLen < 1 || msgLen > 255 {
			return messages
		}
	}

	// byte 2: 报文数量 (最多 10)
	msgCount := int(payload[2])
	if msgCount < 1 || msgCount > 10 {
		return messages
	}

	// bytes 3+: N × 25 字节报文
	offset := 3
	for i := 0; i < msgCount && offset+msgSize <= len(payload); i++ {
		msgData := payload[offset : offset+msgSize]
		msgType := (msgData[0] >> 4) & 0x0F

		messageType, data := p.decodeGB42590Message(msgData, msgType)
		if messageType != "" {
			messages = append(messages, types.DroneMessage{
				MessageType: messageType,
				Standard:    "GB42590-2023",
				Data:        data,
				Source:      "GB42590",
			})
		}
		offset += msgSize
	}

	return messages
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

// ========== GB42590-2023 辅助函数 ==========

// getGBUAtypeName 返回 GB42590 UA 类型名称
func getGBUAtypeName(uaType uint8) string {
	names := []string{
		"None/NotDeclared", "Aeroplane/FixedWing", "Helicopter/Multirotor",
		"Gyroplane", "HybridLift", "Ornithopter", "Glider", "Kite",
		"FreeBalloon", "CaptiveBalloon", "Airship", "Parachute",
		"Rocket", "TetheredPowered", "GroundObstacle", "Other",
	}
	if int(uaType) < len(names) {
		return names[uaType]
	}
	return "Unknown"
}

// getGBIDTypeName 返回 GB42590 ID 类型名称
func getGBIDTypeName(idType uint8) string {
	names := []string{
		"None", "SerialNumber", "CAARegistrationID",
		"UTMUUID", "SpecificSessionID",
	}
	if int(idType) < len(names) {
		return names[idType]
	}
	return "Unknown"
}

// getGBStatusName 返回 GB42590 运行状态名称
// 参考 GB42590-2023 表4: 0=未报告, 1=地面, 2=空中, 3=紧急, 4=识别功能失效(非紧急), 5=识别功能失效(紧急)
func getGBStatusName(status uint8) string {
	names := []string{
		"NotReported", "Ground", "Airborne",
		"Emergency", "RemoteIDFailure", "RemoteIDFailureEmergency",
	}
	if int(status) < len(names) {
		return names[status]
	}
	return fmt.Sprintf("Reserved(%d)", status)
}

// getGBDescTypeName 返回 GB42590 描述类型名称
func getGBDescTypeName(descType uint8) string {
	switch {
	case descType == 0:
		return "Text"
	case descType <= 200:
		return "Reserved"
	default:
		return "PrivateUse"
	}
}

// getGBCoordSysName 返回坐标系类型名称
// 参考 GB42590-2023 表6: 0=WGS-84, 1=CGCS2000
func getGBCoordSysName(coordSys uint8) string {
	switch coordSys {
	case 0:
		return "WGS-84"
	case 1:
		return "CGCS2000"
	default:
		return fmt.Sprintf("Reserved(%d)", coordSys)
	}
}

// getGBClassRegionName 返回等级分类归属区域名称
// 参考 GB42590-2023 表6: 0=未定义, 2=中国
func getGBClassRegionName(region uint8) string {
	switch region {
	case 0:
		return "Undefined"
	case 2:
		return "China"
	default:
		return fmt.Sprintf("Reserved(%d)", region)
	}
}

// getGBStationLocTypeName 返回控制站位置类型名称
// 参考 GB42590-2023 表6: 0=起飞点位置, 1=遥控站位置
func getGBStationLocTypeName(locType uint8) string {
	switch locType {
	case 0:
		return "TakeOff"
	case 1:
		return "Dynamic"
	case 2:
		return "Fixed"
	default:
		return fmt.Sprintf("Reserved(%d)", locType)
	}
}

// getGBUACategoryName 返回 UA 运行类别名称
// 参考 GB42590-2023 表6: 0=未定义, 1=开放类, 2=特许类, 3=审定类
func getGBUACategoryName(category uint8) string {
	switch category {
	case 0:
		return "Undefined"
	case 1:
		return "Open"
	case 2:
		return "Specific"
	case 3:
		return "Certified"
	default:
		return fmt.Sprintf("Reserved(%d)", category)
	}
}

// getGBUAClassName 返回 UA 等级名称
// 参考 GB42590-2023 表6: 0=微型, 1=轻型, 2=小型, 3=其他具备识别功能的
func getGBUAClassName(class uint8) string {
	switch class {
	case 0:
		return "Micro"
	case 1:
		return "Light"
	case 2:
		return "Small"
	case 3:
		return "OtherWithRemoteID"
	default:
		return fmt.Sprintf("Reserved(%d)", class)
	}
}

// getGBHorizontalAccuracy 返回 GB42590 水平精度
// 参考 GB46750-2025 表3: 0=>18.52km/未知, 1-12 递减
func getGBHorizontalAccuracy(acc uint8) string {
	names := []string{
		">=10NM/Unknown", "<10NM", "<4NM", "<2NM",
		"<1NM", "<0.5NM", "<0.3NM", "<0.1NM",
		"<92.6m", "<30m", "<10m", "<3m", "<1m",
	}
	if int(acc) < len(names) {
		return names[acc]
	}
	return fmt.Sprintf("Reserved(%d)", acc)
}

// getGBVerticalAccuracy 返回 GB42590 垂直精度
// 参考 GB46750-2025 表3: 0=>150m/未知, 1-6 递减
func getGBVerticalAccuracy(acc uint8) string {
	names := []string{
		">=150m/Unknown", "<150m", "<45m", "<25m",
		"<10m", "<3m", "<1m",
	}
	if int(acc) < len(names) {
		return names[acc]
	}
	return fmt.Sprintf("Reserved(%d)", acc)
}

// getGBSpeedAccuracy 返回 GB42590 速度精度
// 参考 GB46750-2025 表3: 0=>10m/s/未知, 1-4 递减
func getGBSpeedAccuracy(acc uint8) string {
	names := []string{
		">=10m/s/Unknown", "<10m/s", "<3m/s", "<1m/s", "<0.3m/s",
	}
	if int(acc) < len(names) {
		return names[acc]
	}
	return fmt.Sprintf("Reserved(%d)", acc)
}
