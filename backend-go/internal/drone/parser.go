// internal/drone/parser.go
package drone

// 需要导入的包
import (
	"encoding/binary"
	"fmt"
	"math"
	"remoteid/pkg/types"
	"strings"
	"unicode/utf8"
)

// ASTM F3411-22a 常量
// 注：06:05:04 + 0xFD 是 Wi-Fi NAN (Neighbor Awareness Networking) 的 OUI，
// ASTM F3411-22a 通过 Wi-Fi NAN 的 Vendor Specific Attribute 承载 Remote ID 数据
const (
	astmOUI        = "\x06\x05\x04" // Wi-Fi NAN OUI（ASTM Remote ID 借道 NAN 传输）
	astmVendorType = 0xFD           // Wi-Fi NAN Vendor Specific Attribute Type
	astmMsgSize    = 25             // ASTM 每条消息固定 25 字节
)

// GB42590-2023 常量
const (
	cridOUI        = "\xFA\x0B\xBC" // 中国 CRID OUI
	cridVendorType = 0x0D           // CRID Vendor Type
	cridPackedFlag = 0xF1           // Packed 格式标记
)

type RemoteIDParser struct{}

func NewParser() *RemoteIDParser {
	return &RemoteIDParser{}
}

// ParseFrame 解析数据帧中的 Remote ID 消息（ASTM + GB42590）
func (p *RemoteIDParser) ParseFrame(raw []byte) ([]types.DroneMessage, error) {
	var messages []types.DroneMessage

	// 解析 ASTM F3411-22a 消息
	astmMsgs, err := p.parseASTM(raw)
	if err == nil && len(astmMsgs) > 0 {
		messages = append(messages, astmMsgs...)
	}

	// 解析 GB42590 CRID 消息
	cridMsgs, err := p.parseCRID(raw)
	if err == nil && len(cridMsgs) > 0 {
		messages = append(messages, cridMsgs...)
	}

	return messages, nil
}

// parseASTM 解析 ASTM F3411-22a (OpenDroneID) 消息
func (p *RemoteIDParser) parseASTM(raw []byte) ([]types.DroneMessage, error) {
	var messages []types.DroneMessage

	// 在原始数据中搜索 ASTM OUI (06:05:04)
	for idx := 0; idx <= len(raw)-3; idx++ {
		if string(raw[idx:idx+3]) != astmOUI {
			continue
		}

		// ASTM 消息固定 25 字节，需要足够数据
		offset := idx + 3
		for offset+astmMsgSize <= len(raw) {
			msgData := raw[offset : offset+astmMsgSize]

			// 验证协议版本 = Vendor Type 0xFD
			if msgData[1] != astmVendorType {
				break
			}

			msgType := (msgData[0] >> 4) & 0x0F
			payload := msgData[2:] // 消息体 23 字节

			var messageType string
			standard := "ASTM F3411-22a"
			data := make(map[string]string)

			switch msgType {
			case 0: // Basic ID
				messageType = "basic_id"
				uaType := (payload[0] >> 4) & 0x0F
				idType := payload[0] & 0x0F
				uasIDBytes := payload[1:21]

				uasID := strings.TrimRightFunc(string(uasIDBytes), func(r rune) bool {
					return !utf8.ValidRune(r) || r == 0
				})
				uasID = strings.TrimSpace(uasID)
				if uasID == "" {
					uasID = "UNKNOWN"
				}

				data["uas_id"] = uasID
				data["ua_type"] = getASTMUATypeName(uaType)
				data["id_type"] = getASTMIDTypeName(idType)
				data["standard"] = standard

			case 1: // Location/Vector
				messageType = "location"
				status := (payload[0] >> 4) & 0x0F
				// direction: 2 字节大端 uint16，0-65535 映射到 0-360°
				direction := float64(binary.BigEndian.Uint16(payload[1:3])) * 360.0 / 65535.0
				// speed_h: 2 字节大端 uint16，单位 0.25 m/s
				speedH := float64(binary.BigEndian.Uint16(payload[3:5])) * 0.25
				// speed_v: 1 字节 int8，单位 0.5 m/s（减去 64）
				speedV := float64(int8(payload[5])-64) * 0.5
				// latitude: 3 字节大端 int24，单位 1e-7 度
				lat := float64(beInt24(payload[6:9])) / 10000000.0
				// longitude: 3 字节大端 int24，单位 1e-7 度
				lon := float64(beInt24(payload[9:12])) / 10000000.0
				// altitude: 2 字节小端 uint16，单位 0.5m，偏移 -1000m
				alt := float64(binary.LittleEndian.Uint16(payload[12:14]))*0.5 - 1000.0
				// height: 2 字节小端 uint16，单位 0.5m
				height := float64(binary.LittleEndian.Uint16(payload[14:16])) * 0.5
				// horizontal accuracy
				hAcc := getASTMHorizontalAccuracy(payload[16])
				// vertical accuracy
				vAcc := getASTMVerticalAccuracy(payload[17])
				// baro altitude: 2 字节小端 uint16
				baroAlt := float64(binary.LittleEndian.Uint16(payload[18:20]))*0.5 - 1000.0
				// speed accuracy
				sAcc := getASTMSpeedAccuracy(payload[20])

				data["status"] = getASTMStatusName(status)
				data["direction"] = fmt.Sprintf("%.2f", direction)
				data["speed_h"] = fmt.Sprintf("%.2f", speedH)
				data["speed_v"] = fmt.Sprintf("%.2f", speedV)
				// 只有当坐标有效时才写入（ASTM 中 90.0 表示未知纬度，180.0 表示未知经度）
				if math.Abs(lat) < 90.0 {
					data["latitude"] = fmt.Sprintf("%.7f", lat)
				}
				if math.Abs(lon) < 180.0 {
					data["longitude"] = fmt.Sprintf("%.7f", lon)
				}
				if alt > -999.0 {
					data["altitude"] = fmt.Sprintf("%.2f", alt)
				}
				data["height_m"] = fmt.Sprintf("%.2f", height)
				data["baro_altitude"] = fmt.Sprintf("%.2f", baroAlt)
				data["h_accuracy"] = hAcc
				data["v_accuracy"] = vAcc
				data["s_accuracy"] = sAcc
				data["standard"] = standard

			case 2: // Authentication (暂不解析详细内容，保留占位)
				messageType = "authentication"
				data["standard"] = standard

			case 3: // Self ID
				messageType = "self_id"
				descBytes := payload[0:23]
				desc := strings.TrimRightFunc(string(descBytes), func(r rune) bool {
					return !utf8.ValidRune(r) || r == 0
				})
				data["description"] = strings.TrimSpace(desc)
				data["standard"] = standard

			case 4: // System
				messageType = "system"
				flags := payload[0]
				opLat := float64(beInt24(payload[1:4])) / 10000000.0
				opLon := float64(beInt24(payload[4:7])) / 10000000.0
				areaCount := binary.BigEndian.Uint16(payload[7:9])
				areaRadius := payload[9]
				areaCeiling := float64(binary.LittleEndian.Uint16(payload[10:12])) * 0.5 - 1000.0
				areaFloor := float64(binary.LittleEndian.Uint16(payload[12:14])) * 0.5 - 1000.0
				classification := payload[17]

				data["flags"] = fmt.Sprintf("0x%02X", flags)
				data["operator_lat"] = fmt.Sprintf("%.7f", opLat)
				data["operator_lon"] = fmt.Sprintf("%.7f", opLon)
				data["area_count"] = fmt.Sprintf("%d", areaCount)
				data["area_radius_m"] = fmt.Sprintf("%d", areaRadius)
				data["area_ceiling"] = fmt.Sprintf("%.2f", areaCeiling)
				data["area_floor"] = fmt.Sprintf("%.2f", areaFloor)
				data["classification"] = getASTMClassificationName(classification)
				data["standard"] = standard

			case 5: // Operator ID
				messageType = "operator_id"
				opIDBytes := payload[0:20]
				opID := strings.TrimRightFunc(string(opIDBytes), func(r rune) bool {
					return !utf8.ValidRune(r) || r == 0
				})
				data["operator_id"] = strings.TrimSpace(opID)
				data["standard"] = standard

			default:
				// 未知消息类型，跳过
			}

			if messageType != "" {
				messages = append(messages, types.DroneMessage{
					MessageType: messageType,
					Standard:    standard,
					Data:        data,
					Source:      "ASTM",
				})
			}

			offset += astmMsgSize
		}

		// 找到一处 OUI 后不再继续搜索（避免重复匹配同一批消息）
		if len(messages) > 0 {
			return messages, nil
		}
	}

	return messages, nil
}

// beInt24 解析 3 字节大端有符号整数
func beInt24(b []byte) int32 {
	if len(b) < 3 {
		return 0
	}
	val := int32(b[0])<<16 | int32(b[1])<<8 | int32(b[2])
	// 符号扩展
	if val&0x800000 != 0 {
		val |= ^0xFFFFFF
	}
	return val
}

// parseCRID 解析 GB42590-2023 消息
func (p *RemoteIDParser) parseCRID(raw []byte) ([]types.DroneMessage, error) {
	var messages []types.DroneMessage
	idx := 0

	// 查找 China CRID OUI (FA 0B BC)
	for idx <= len(raw)-4 {
		if string(raw[idx:idx+3]) == cridOUI && idx+3 < len(raw) && raw[idx+3] == cridVendorType {
			payload := raw[idx+4:]

			// 检查是否为 Packed 格式 (0xF1)
			if len(payload) >= 3 && ((payload[0]>>4)&0x0F) == 0xF {
				msgCount := int(payload[3] & 0x0F)
				offset := 4

				// 解析打包的消息
				for i := 0; i < msgCount && offset+25 <= len(payload); i++ {
					msgData := payload[offset : offset+25]
					msgType := (msgData[0] >> 4) & 0x0F

					var messageType string
					var standard string
					data := make(map[string]string)

					switch msgType {
					case 0: // Basic ID
						messageType = "basic_id"
						standard = "GB42590-2023"
						idUaByte := msgData[1]
						idType := (idUaByte >> 4) & 0x07
						uaType := idUaByte & 0x0F
						uasIDBytes := msgData[2:22]

						uasID := strings.TrimRightFunc(string(uasIDBytes), func(r rune) bool {
							return !utf8.ValidRune(r) || r == 0
						})
						uasID = strings.TrimSpace(uasID)

						if uasID == "" {
							uasID = "CAAC-UNKNOWN"
						}

						data["uas_id"] = uasID
						data["ua_type"] = getUATypeName(uaType)
						data["id_type"] = getIDTypeName(idType)
						data["standard"] = standard

					case 1: // Location
						messageType = "location"
						standard = "GB42590-2023"
						flags := msgData[1]
						status := (flags >> 4) & 0x07
						directionHigh := msgData[1] & 0x0F // 1. +++ 修正：获取高8位部分 +++
						directionLow := msgData[2]
						// 2. +++ 修正：正确的位操作 +++
						direction := float64((int(directionHigh)<<8 + int(directionLow))) * 360.0 / 65535.0

						speedH := float64(msgData[3]) * 0.25
						speedV := float64(int8(msgData[4])-64) * 0.5

						lat := float64(int32(binary.LittleEndian.Uint32(msgData[5:9]))) / 10000000.0
						lon := float64(int32(binary.LittleEndian.Uint32(msgData[9:13]))) / 10000000.0
						alt := float64(binary.LittleEndian.Uint16(msgData[13:15]))*0.5 - 1000.0

						data["status"] = getStatusName(status)
						data["direction"] = fmt.Sprintf("%.2f", direction)
						data["speed_h"] = fmt.Sprintf("%.2f", speedH)
						data["speed_v"] = fmt.Sprintf("%.2f", speedV)
						data["latitude"] = fmt.Sprintf("%.7f", lat)
						data["longitude"] = fmt.Sprintf("%.7f", lon)
						data["altitude"] = fmt.Sprintf("%.2f", alt)
						data["standard"] = standard

					case 4: // System
						messageType = "system"
						standard = "GB42590-2023"
						classification := (msgData[1] >> 4) & 0x07
						opLat := float64(int32(binary.LittleEndian.Uint32(msgData[2:10]))) / 10000000.0
						opLon := float64(int32(binary.LittleEndian.Uint32(msgData[10:18]))) / 10000000.0

						data["classification"] = getClassificationName(classification)
						data["operator_lat"] = fmt.Sprintf("%.7f", opLat)
						data["operator_lon"] = fmt.Sprintf("%.7f", opLon)
						data["standard"] = standard
					}

					if messageType != "" {
						messages = append(messages, types.DroneMessage{
							MessageType: messageType,
							Standard:    standard,
							Data:        data,
							Source:      "GB42590",
						})
					}

					offset += 25
				}
				break
			}
		}
		idx++
	}

	return messages, nil
}

// 辅助函数
func getUATypeName(uaType uint8) string {
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

func getIDTypeName(idType uint8) string {
	names := []string{
		"None", "SerialNumber", "CAARegistrationID",
		"UTMUUID", "SpecificSessionID",
	}
	if int(idType) < len(names) {
		return names[idType]
	}
	return "Unknown"
}

func getStatusName(status uint8) string {
	names := []string{
		"Undeclared", "Ground", "Airborne",
		"Emergency", "RemoteIDSystemFailure",
	}
	if int(status) < len(names) {
		return names[status]
	}
	return "Unknown"
}

func getClassificationName(classification uint8) string {
	names := []string{
		"Undefined", "USA", "China", "EU", "UK", "Japan", "Australia", "Other",
	}
	if int(classification) < len(names) {
		return names[classification]
	}
	return "Other"
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
