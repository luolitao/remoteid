// internal/drone/parser.go
package drone

// 需要导入的包
import (
	"encoding/binary"
	"fmt"
	"remoteid/pkg/types"
	"strings"
	"unicode/utf8"
)

type RemoteIDParser struct{}

func NewParser() *RemoteIDParser {
	return &RemoteIDParser{}
}

// ParseFrame 解析数据帧中的 CRID 消息
func (p *RemoteIDParser) ParseFrame(raw []byte) ([]types.DroneMessage, error) {
	var messages []types.DroneMessage

	// 解析 GB42590 CRID 消息
	cridMsgs, err := p.parseCRID(raw)
	if err == nil && len(cridMsgs) > 0 {
		messages = append(messages, cridMsgs...)
	}

	return messages, nil
}

// parseCRID 解析 GB42590-2023 消息
func (p *RemoteIDParser) parseCRID(raw []byte) ([]types.DroneMessage, error) {
	var messages []types.DroneMessage
	idx := 0

	// 查找 China CRID OUI (FA 0B BC)
	for idx <= len(raw)-4 {
		if string(raw[idx:idx+3]) == "\xFA\x0B\xBC" && idx+3 < len(raw) && raw[idx+3] == 0x0D {
			payload := raw[idx+4:]

			// 检查是否为 Packed 格式 (0xF1)
			if len(payload) >= 3 && ((payload[1]>>4)&0x0F) == 0xF {
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
