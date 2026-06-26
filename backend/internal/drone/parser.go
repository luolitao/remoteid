package drone

import (
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"

	"remoteid-monitor/pkg/types"
)

const (
	// ASTM/ASD-STAN Vendor Specific IE OUI
	asdStanOUI     = "\xFA\x0B\xBC" // ASD-STAN OUI
	asdStanOUIType = 0x0D           // ASD-STAN OUI Type
	msgSize        = 25             // 每条 Remote ID 消息固定 25 字节

	// 旧版 ASTM / OpenDroneID 临时 OUI（部分 ESP32 早期实现使用）
	legacyASTMOUI     = "\x06\x05\x04" // 旧版 ASTM OUI
	legacyASTMOUIType = 0xFD           // 旧版 ASTM OUI Type

	// ASTM 专属常量
	astmProtocolVersion = 2 // ASTM F3411-22a 协议版本号（Message Header 的 低 4 位）

	// GB 42590-2023 常量
	gbProtocolVersion = 1 // GB 42590-2023 协议版本号（Message Header 的 低 4 位）

	// GB 46750-2023 常量
	gb46750Magic        = 0xFF // GB 46750 数据类型标识魔数（data[1]）
	gb46750MajorVersion = 0x1  // GB 46750 主版本号（版本字节高3位）
)

type RemoteIDParser struct{}

func NewParser() *RemoteIDParser { return &RemoteIDParser{} }

func (p *RemoteIDParser) ParseFrame(raw []byte) ([]types.DroneMessage, error) {
	return p.parseVendorIE(raw)
}

func (p *RemoteIDParser) ParseNANFrame(raw []byte) ([]types.DroneMessage, error) {
	return p.parseNANSDF(raw)
}

func (p *RemoteIDParser) parseNANSDF(raw []byte) ([]types.DroneMessage, error) {
	var messages []types.DroneMessage
	for idx := 0; idx <= len(raw)-4; idx++ {
		if raw[idx] != 0x50 || raw[idx+1] != 0x6F || raw[idx+2] != 0x9A || raw[idx+3] != 0x13 {
			continue
		}
		messages = append(messages, p.parseNANAttributes(raw, idx+4)...)
		if len(messages) == 0 {
			msgs, _ := p.parseVendorIE(raw)
			messages = append(messages, msgs...)
		}
		break
	}
	return messages, nil
}

// parseNANAttributes 解析 NAN Attributes 中的 Remote ID 数据
func (p *RemoteIDParser) parseNANAttributes(raw []byte, offset int) []types.DroneMessage {
	var messages []types.DroneMessage
	for offset+3 <= len(raw) {
		attrID := raw[offset]
		if attrID == 0x00 {
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

				// ✅ 核心修复：attrValue 已经是从 OUI 开始的 Vendor Specific Body
				payloadHex := fmt.Sprintf("%X", attrValue)
				if len(payloadHex) > 256 { // 128 字节 = 256 个 hex 字符
					payloadHex = payloadHex[:256] + "..."
				}

				// 扫描查找 ASTM 消息 Header（跳过 Message Counter + 可能的额外元数据）
				msgStart := p.findASTMMessageHeader(attrValue, 4)
				if msgStart >= 0 {
					msgs := p.parseASTMBeaconMessages(attrValue[msgStart:])
					for i := range msgs {
						msgs[i].RawHex = payloadHex // ✅ 强制覆盖为纯净的 Payload Hex
					}
					messages = append(messages, msgs...)
				}
			}
		}

		// 移动到下一个 Attribute
		offset += 3 + attrLen
	}

	return messages
}

// parseVendorIE 严格按照 802.11 TLV 格式遍历 Information Elements
func (p *RemoteIDParser) parseVendorIE(ieList []byte) ([]types.DroneMessage, error) {
	var messages []types.DroneMessage

	offset := 0
	// 严格遍历 IE 列表
	for offset < len(ieList) {
		// 1. 检查是否足够读取 Element ID 和 Length
		if offset+2 > len(ieList) {
			break
		}

		elementID := ieList[offset]
		length := int(ieList[offset+1])

		// 2. 检查 Length 是否越界 (数据损坏防护)
		if offset+2+length > len(ieList) {
			break
		}

		// 3. 只处理 Vendor Specific IE (0xDD)
		if elementID == 0xDD {
			// 至少需要 OUI(3) + Type(1) message counter(1)= 5 字节
			if length >= 5 {
				ieData := ieList[offset+2 : offset+2+length]

				// 检查 ASD-STAN OUI (FA:0B:BC) + Type (0x0D)
				if ieData[0] == 0xFA && ieData[1] == 0x0B && ieData[2] == 0xBC && ieData[3] == 0x0D {

					// 提取纯净 Hex (从 OUI 开始)
					payloadHex := fmt.Sprintf("%X", ieData)
					if len(payloadHex) > 256 {
						payloadHex = payloadHex[:256] + "..."
					}

					// 解析 Remote ID 消息 (跳过 OUI+Type)
					dataStart := 5

					if p.isGB46750Format(ieData, dataStart) {
						msgs := p.parseGB46750Payload(ieData[dataStart:])

						for i := range msgs {
							msgs[i].RawHex = payloadHex
						}
						messages = append(messages, msgs...)

						//slog.Info("解析 GB46750 协议", "messages", messages)

					} else {
						msgStart := p.findASTMMessageHeader(ieData, dataStart)
						if msgStart >= 0 {
							msgs := p.parseASTMBeaconMessages(ieData[msgStart:])
							for i := range msgs {
								msgs[i].RawHex = payloadHex
							}
							messages = append(messages, msgs...)
						}
					}
				}
			}
		}

		// 4. 严格跳转到下一个 IE
		offset += 2 + length
	}

	return messages, nil
}

func (p *RemoteIDParser) isGB46750Format(raw []byte, dataStart int) bool {
	// 载荷至少需要: DataType(1) + Version(1) + Length(1) + 至少1个Flags字节 = 4字节
	if dataStart+4 > len(raw) {
		return false
	}

	payload := raw[dataStart:]

	// 1. 严格匹配 GB46750 魔数：第1字节必须是 0xFF
	if payload[0] != gb46750Magic {
		return false
	}

	// 2. 严格匹配主版本号：第2字节的低3位 (Bits 0-2) 必须是 001 (十进制1)
	//    协议原文: "Bits 0-2: 001"
	if (payload[1] >> 5) != 0x01 {
		return false
	}

	// 3. 合理性校验：第3字节为 Length，协议规定 1~200，防止越界解析
	length := int(payload[2])
	if length < 1 || length > 200 {
		return false
	}

	return true
}

// findASTMMessageHeader 在 raw 中从 scanStart 开始扫描，查找 ASTM 或 GB 消息 Header
// Header 格式: 高 4 位 = Message Type (0-5 或 15), 低 4 位 = Protocol Version (1=GB, 2=ASTM)
// 即 byte 满足: msgType <= 5 (或 15) 且 (protoVer == 1 或 2)
// 最多扫描 8 字节（覆盖 Message Counter + 可能的额外元数据）
func (p *RemoteIDParser) findASTMMessageHeader(raw []byte, scanStart int) int {
	maxScan := scanStart + 2 // 最多允许 2 字节的偏移，应对非标准 padding
	if maxScan > len(raw) {
		maxScan = len(raw)
	}

	for i := scanStart; i < maxScan; i++ {
		b := raw[i]
		msgType := (b >> 4) & 0x0F
		protoVer := b & 0x0F // 提取低 4 位作为 Protocol Version

		// 标准格式：高4位=消息类型(0-5 或 15), 低4位=协议版本(1=GB, 2=ASTM)
		// 注意：容忍 protoVer == 0，因为某些固件在 Packed 内部消息中会错误地将 version 设为 0
		if (msgType <= 5 || msgType == 15) && (protoVer == 1 || protoVer == 2 || protoVer == 0) {
			return i
		}

		// 兼容极旧版格式：高4位=协议版本(1或2), 低4位=消息类型(0-5)
		if (protoVer == 1 || protoVer == 2) && msgType <= 5 {
			return i
		}
	}

	return -1
}

// parseASTMBeaconMessages 解析 Beacon 格式的单条/多条消息
// 支持 ASTM F3411-22a 和 GB 42590-2023，并完整支持 Packed Message (MsgType = 0xF)
func (p *RemoteIDParser) parseASTMBeaconMessages(payload []byte) []types.DroneMessage {
	var messages []types.DroneMessage
	offset := 0

	for offset+msgSize <= len(payload) {
		msgData := payload[offset : offset+msgSize]
		msgType := (msgData[0] >> 4) & 0x0F
		protoVer := msgData[0] & 0x0F

		// ========== 核心修复：处理 Packed Message (MsgType = 0xF) ==========
		if msgType == 0xF {
			if len(payload) < offset+3 {
				break
			}
			singleMsgSize := int(payload[offset+1]) // 通常为 25 (0x19)
			numMsgs := int(payload[offset+2])       // 消息包中的消息数量

			// 跳过 3 字节的 Pack Header，指向第一个实际消息
			offset += 3

			for i := 0; i < numMsgs && offset+singleMsgSize <= len(payload); i++ {
				innerMsgData := payload[offset : offset+singleMsgSize]
				innerMsgType := (innerMsgData[0] >> 4) & 0x0F
				innerProtoVer := innerMsgData[0] & 0x0F

				// 容错处理：如果内部消息的 version 为 0 (固件bug)，则继承外层 Packed Header 的 version
				effectiveProtoVer := innerProtoVer
				if effectiveProtoVer == 0 {
					effectiveProtoVer = protoVer
				}

				var messageType string
				var data map[string]any
				var standard string

				if effectiveProtoVer == gbProtocolVersion {
					messageType, data = p.decodeGBMessage(innerMsgData, innerMsgType)
					standard = "GB 42590-2023"
				} else {
					messageType, data = p.decodeASTMMessage(innerMsgData, innerMsgType)
					standard = "ASTM F3411-22a"
				}

				if messageType != "" {
					messages = append(messages, types.DroneMessage{
						MessageType: messageType,
						Standard:    standard,
						Data:        data,
						Source:      "ASTM",
						// 保留 RawHex 供调试
					})
				}
				offset += singleMsgSize
			}
			// Packed Message 处理完毕，退出当前循环
			break
		}
		// ==============================================================

		// 处理普通的单条消息 (MsgType 0-5)
		var messageType string
		var data map[string]any
		var standard string

		if protoVer == gbProtocolVersion || protoVer == 0 {
			messageType, data = p.decodeGBMessage(msgData, msgType)
			standard = "GB 42590-2023"
		} else {
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

// ================= 新增：数据合并方法 =================

// MergeMessages 将多条 DroneMessage 的数据合并更新到 DroneData 对象中
// 只负责字段映射，不包含业务日志或位置持久化逻辑（由调用方处理）
func (p *RemoteIDParser) oldMergeMessages(drone *types.DroneData, messages []types.DroneMessage) {
	for _, msg := range messages {
		msgType := strings.TrimSpace(msg.MessageType)

		// 清洗 Data map 的所有 key 和 value，转为 string 形式
		cleanData := make(map[string]string, len(msg.Data))
		for k, v := range msg.Data {
			strVal := fmt.Sprintf("%v", v)
			trimmedVal := strings.TrimSpace(strVal)
			cleanData[strings.TrimSpace(k)] = trimmedVal
		}

		// 辅助取值函数
		getStr := func(key string) string { return cleanData[key] }
		getFloat := func(key string) float64 {
			if f, err := strconv.ParseFloat(cleanData[key], 64); err == nil {
				return f
			}
			return 0
		}
		getInt := func(key string) int {
			if i, err := strconv.Atoi(cleanData[key]); err == nil {
				return i
			}
			return 0
		}

		switch msgType {
		case "basic_id":
			if uasID := getStr("uas_id"); uasID != "" && uasID != "UNKNOWN" {
				drone.UASID = uasID
			}
			if uaType := getStr("ua_type"); uaType != "" {
				drone.UAType = uaType
			}
			if idType := getStr("id_type"); idType != "" {
				drone.IDType = idType
			}
			drone.Standard = strings.TrimSpace(msg.Standard)
			drone.Source = strings.TrimSpace(msg.Source)

		case "location":
			if lat := getFloat("latitude"); lat != 0 {
				drone.Latitude = lat
			}
			if lon := getFloat("longitude"); lon != 0 {
				drone.Longitude = lon
			}
			// 高度优先级：baro > geo > height_m
			altBaro := getFloat("altitude_baro")
			altGeo := getFloat("altitude_geo")
			heightM := getFloat("height_m")
			if altBaro != 0 {
				drone.Altitude = altBaro
			} else if altGeo != 0 {
				drone.Altitude = altGeo
			} else if heightM != 0 && drone.Altitude == 0 {
				drone.Altitude = heightM
			}
			// 注意：高度合理性检查由 processor 负责，此处不处理

			if speed := getFloat("speed_h"); speed != 0 {
				drone.Speed = speed
			}
			if heading := getFloat("direction"); heading != 0 {
				drone.Heading = heading
			}
			if speedV := getFloat("speed_v"); speedV != 0 {
				drone.SpeedVertical = speedV
			}
			if status := getStr("status"); status != "" {
				drone.FlightStatus = status
			}
			if ht := getStr("height_type"); ht != "" {
				drone.HeightType = ht
			}
			if ha := getStr("h_accuracy"); ha != "" {
				drone.HAccuracy = ha
			}
			if va := getStr("v_accuracy"); va != "" {
				drone.VAccuracy = va
			}
			if sa := getStr("s_accuracy"); sa != "" {
				drone.SAccuracy = sa
			}
			if ts := getStr("timestamp"); ts != "" {
				drone.LocationTimestamp = ts
			}

		case "operator_id":
			if opID := getStr("operator_id"); opID != "" && opID != " " {
				drone.OperatorID = opID
			}

		case "system":
			if opLat := getFloat("operator_lat"); opLat != 0 {
				drone.OperatorLatitude = opLat
			}
			if opLon := getFloat("operator_lon"); opLon != 0 {
				drone.OperatorLongitude = opLon
			}
			if opAlt := getFloat("operator_alt"); opAlt != 0 {
				drone.OperatorAltitude = opAlt
			}
			if classification := getStr("classification"); classification != "" {
				drone.Classification = classification
			}
			if areaRadius := getInt("area_radius_m"); areaRadius > 0 {
				drone.AreaRadiusM = areaRadius
			}

		case "gb46750":
			drone.Standard = strings.TrimSpace(msg.Standard)
			drone.Source = strings.TrimSpace(msg.Source)
			if uasID := getStr("unique_id"); uasID != "" {
				drone.UASID = uasID
			}
			if realnameID := getStr("realname_id"); realnameID != "" {
				drone.OperatorID = realnameID
			}
			if uaCat := getStr("ua_category"); uaCat != "" {
				drone.UAType = uaCat
			}
			drone.IDType = "SerialNumber"

			if lat := getFloat("latitude"); lat != 0 {
				drone.Latitude = lat
			}
			if lon := getFloat("longitude"); lon != 0 {
				drone.Longitude = lon
			}
			altBaro := getFloat("altitude_baro")
			altGeo := getFloat("altitude_geo")
			heightM := getFloat("height_m")
			if altBaro != 0 {
				drone.Altitude = altBaro
			} else if altGeo != 0 {
				drone.Altitude = altGeo
			} else if heightM != 0 {
				drone.Altitude = heightM
			}
			// 高度合理性检查由 processor 负责

			if speed := getFloat("speed_h"); speed != 0 {
				drone.Speed = speed
			}
			if speedV := getFloat("speed_v"); speedV != 0 {
				drone.SpeedVertical = speedV
			}
			if heading := getFloat("direction"); heading != 0 {
				drone.Heading = heading
			}
			if status := getStr("status"); status != "" {
				drone.FlightStatus = status
			}
			if ht := getStr("height_type"); ht != "" {
				drone.HeightType = ht
			}
			if ha := getStr("h_accuracy"); ha != "" {
				drone.HAccuracy = ha
			}
			if va := getStr("v_accuracy"); va != "" {
				drone.VAccuracy = va
			}
			if sa := getStr("s_accuracy"); sa != "" {
				drone.SAccuracy = sa
			}
			if ts := getStr("timestamp"); ts != "" {
				drone.LocationTimestamp = ts
			}
			if rcsLat := getFloat("rcs_latitude"); rcsLat != 0 {
				drone.OperatorLatitude = rcsLat
			}
			if rcsLon := getFloat("rcs_longitude"); rcsLon != 0 {
				drone.OperatorLongitude = rcsLon
			}
			if rcsAlt := getFloat("rcs_altitude"); rcsAlt != 0 {
				drone.OperatorAltitude = rcsAlt
			}
			if opCat := getStr("operation_category"); opCat != "" {
				drone.Classification = "GB46750-Cat" + opCat
			}
		}
	}
}

// MergeMessages 合并所有消息，填充通用字段和协议特定数据
func (p *RemoteIDParser) MergeMessages(drone *types.DroneData, messages []types.DroneMessage) {
	if len(messages) == 0 {
		return
	}

	// 确定协议类型（取第一条非空）
	var protocol string
	for _, msg := range messages {
		if msg.Standard != "" {
			if strings.Contains(msg.Standard, "46750") {
				protocol = "gb46750"
			} else if strings.Contains(msg.Standard, "42590") {
				protocol = "gb42590"
			} else if strings.Contains(msg.Standard, "ASTM") {
				protocol = "astm"
			}
			break
		}
	}
	drone.Protocol = protocol

	// 先清空协议特定数据（避免残留）
	drone.GB46750 = nil
	drone.GB42590 = nil
	drone.ASTM = nil

	// 遍历所有消息，分别填充
	for _, msg := range messages {
		// 填充通用字段
		p.fillCommonFields(drone, msg)

		// 根据协议填充特定字段
		switch protocol {
		case "gb46750":
			p.fillGB46750Specific(drone, msg)
		case "gb42590":
			p.fillGB42590Specific(drone, msg)
		case "astm":
			p.fillASTMSpecific(drone, msg)
		}
	}

	// 在 MergeMessages 末尾
	if protocol == "gb46750" && drone.GB46750 != nil {
		if drone.UASID == "" && drone.GB46750.UniqueID != "" {
			drone.UASID = drone.GB46750.UniqueID
		}
		if drone.OperatorID == "" && drone.GB46750.RealNameID != "" {
			drone.OperatorID = drone.GB46750.RealNameID
		}
	}
}

// fillCommonFields 提取所有协议共有的字段
func (p *RemoteIDParser) fillCommonFields(drone *types.DroneData, msg types.DroneMessage) {
	// 清洗数据（同前）
	cleanData := make(map[string]string, len(msg.Data))
	for k, v := range msg.Data {
		strVal := fmt.Sprintf("%v", v)
		trimmedVal := strings.TrimSpace(strVal)
		cleanData[strings.TrimSpace(k)] = trimmedVal
	}

	getStr := func(key string) string { return cleanData[key] }
	getFloat := func(key string) float64 {
		if f, err := strconv.ParseFloat(cleanData[key], 64); err == nil {
			return f
		}
		return 0
	}
	getInt := func(key string) int {
		if i, err := strconv.Atoi(cleanData[key]); err == nil {
			return i
		}
		return 0
	}

	// 只提取通用字段（不包含协议特定字段）
	if uasID := getStr("uas_id"); uasID != "" && uasID != "UNKNOWN" {
		drone.UASID = uasID
	}
	if uaType := getStr("ua_type"); uaType != "" {
		drone.UAType = uaType
	}
	if idType := getStr("id_type"); idType != "" {
		drone.IDType = idType
	}
	if opID := getStr("operator_id"); opID != "" && opID != " " {
		drone.OperatorID = opID
	}

	// 位置
	if lat := getFloat("latitude"); lat != 0 {
		drone.Latitude = lat
	}
	if lon := getFloat("longitude"); lon != 0 {
		drone.Longitude = lon
	}

	// 高度（仅通用高度，不覆盖协议特定）
	altBaro := getFloat("altitude_baro")
	altGeo := getFloat("altitude_geo")
	heightM := getFloat("height_m")
	if altBaro != 0 {
		drone.Altitude = altBaro
	} else if altGeo != 0 {
		drone.Altitude = altGeo
	} else if heightM != 0 && drone.Altitude == 0 {
		drone.Altitude = heightM
	}

	if speed := getFloat("speed_h"); speed != 0 {
		drone.Speed = speed
	}
	if heading := getFloat("direction"); heading != 0 {
		drone.Heading = heading
	}
	if speedV := getFloat("speed_v"); speedV != 0 {
		drone.SpeedVertical = speedV
	}

	if status := getStr("status"); status != "" {
		drone.FlightStatus = status
	}
	if ht := getStr("height_type"); ht != "" {
		drone.HeightType = ht
	}
	if ha := getStr("h_accuracy"); ha != "" {
		drone.HAccuracy = ha
	}
	if va := getStr("v_accuracy"); va != "" {
		drone.VAccuracy = va
	}
	if sa := getStr("s_accuracy"); sa != "" {
		drone.SAccuracy = sa
	}
	if ts := getStr("timestamp"); ts != "" {
		drone.LocationTimestamp = ts
	}

	// 操作员信息（通用）
	if opLat := getFloat("operator_lat"); opLat != 0 {
		drone.OperatorLatitude = opLat
	}
	if opLon := getFloat("operator_lon"); opLon != 0 {
		drone.OperatorLongitude = opLon
	}
	if opAlt := getFloat("operator_alt"); opAlt != 0 {
		drone.OperatorAltitude = opAlt
	}
	if classification := getStr("classification"); classification != "" {
		drone.Classification = classification
	}
	if areaRadius := getInt("area_radius_m"); areaRadius > 0 {
		drone.AreaRadiusM = areaRadius
	}

	// 标准/Source（由上一层保留，此处不覆盖）
	// drone.Standard 在外部已设置
	// drone.Source 同理
}

// fillGB46750Specific 填充 GB46750 特有字段
func (p *RemoteIDParser) fillGB46750Specific(drone *types.DroneData, msg types.DroneMessage) {
	if drone.GB46750 == nil {
		drone.GB46750 = &types.GB46750Data{}
	}
	// 从 msg.Data 提取特有字段
	if v, ok := msg.Data["version"]; ok {
		drone.GB46750.Version = fmt.Sprintf("%v", v)
	}
	if v, ok := msg.Data["unique_id"]; ok {
		drone.GB46750.UniqueID = fmt.Sprintf("%v", v)
	}
	// 注意：realname_id 也已存在，继续保留
	if v, ok := msg.Data["ua_category"]; ok {
		drone.GB46750.UACategory = fmt.Sprintf("%v", v)
	}
	if v, ok := msg.Data["realname_id"]; ok {
		drone.GB46750.RealNameID = fmt.Sprintf("%v", v)
	}
	if v, ok := msg.Data["operation_category"]; ok {
		drone.GB46750.OperationCategory = fmt.Sprintf("%v", v)
	}
	if v, ok := msg.Data["rcs_loc_type"]; ok {
		drone.GB46750.RCSLocType = fmt.Sprintf("%v", v)
	}
	if v, ok := msg.Data["rcs_altitude"]; ok {
		if f, err := strconv.ParseFloat(fmt.Sprintf("%v", v), 64); err == nil {
			drone.GB46750.RCSAltitude = f
		}
	}
	if v, ok := msg.Data["status"]; ok {
		drone.GB46750.Status = fmt.Sprintf("%v", v)
	}
	if v, ok := msg.Data["coord_system"]; ok {
		drone.GB46750.CoordSystem = fmt.Sprintf("%v", v)
	}
	if v, ok := msg.Data["h_accuracy"]; ok {
		drone.GB46750.HAccuracy = fmt.Sprintf("%v", v)
	}
	if v, ok := msg.Data["v_accuracy"]; ok {
		drone.GB46750.VAccuracy = fmt.Sprintf("%v", v)
	}
	if v, ok := msg.Data["s_accuracy"]; ok {
		drone.GB46750.SAccuracy = fmt.Sprintf("%v", v)
	}
	if v, ok := msg.Data["timestamp_ms"]; ok {
		if ts, err := strconv.ParseUint(fmt.Sprintf("%v", v), 10, 64); err == nil {
			drone.GB46750.TimestampMS = ts
		}
	}
	if v, ok := msg.Data["ts_accuracy"]; ok {
		drone.GB46750.TSSAccuracy = fmt.Sprintf("%v", v)
	}

	// 遥控站位置（如果有）
	if lon, ok := msg.Data["rcs_longitude"]; ok {
		if lonF, err := strconv.ParseFloat(fmt.Sprintf("%v", lon), 64); err == nil {
			if drone.GB46750.RCSLocation == nil {
				drone.GB46750.RCSLocation = &types.Position{}
			}
			drone.GB46750.RCSLocation.Longitude = lonF
		}
	}
	if lat, ok := msg.Data["rcs_latitude"]; ok {
		if latF, err := strconv.ParseFloat(fmt.Sprintf("%v", lat), 64); err == nil {
			if drone.GB46750.RCSLocation == nil {
				drone.GB46750.RCSLocation = &types.Position{}
			}
			drone.GB46750.RCSLocation.Latitude = latF
		}
	}
	// 可添加高度等
}

// fillGB42590Specific 预留
func (p *RemoteIDParser) fillGB42590Specific(drone *types.DroneData, msg types.DroneMessage) {
	// 目前无特有字段，可留空
}

// fillASTMSpecific 预留
func (p *RemoteIDParser) fillASTMSpecific(drone *types.DroneData, msg types.DroneMessage) {
	// 目前无特有字段，可留空
}
