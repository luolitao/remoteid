package drone

import (
	"encoding/binary"
	"fmt"
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
