package drone

import (
	"encoding/binary"
	"fmt"
	"remoteid-monitor/pkg/types"
)

const (
	asdStanOUI          = "\xFA\x0B\xBC"
	asdStanOUIType      = 0x0D
	legacyASTMOUI       = "\x06\x05\x04"
	legacyASTMOUIType   = 0xFD
	msgSize             = 25
	astmProtocolVersion = 2
	gbProtocolVersion   = 1
	gb46750Magic        = 0xFF
	gb46750MajorVersion = 0x1
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

func (p *RemoteIDParser) parseNANAttributes(raw []byte, offset int) []types.DroneMessage {
	var messages []types.DroneMessage
	for offset+3 <= len(raw) {
		attrID := raw[offset]
		if attrID == 0x00 {
			break
		}

		attrLen := int(binary.LittleEndian.Uint16(raw[offset+1 : offset+3])) // 需导入 encoding/binary
		if attrLen == 0 || offset+3+attrLen > len(raw) {
			break
		}

		attrValue := raw[offset+3 : offset+3+attrLen]
		if attrID == 0xDD && len(attrValue) >= 5 && attrValue[0] == 0xFA && attrValue[1] == 0x0B && attrValue[2] == 0xBC && attrValue[3] == 0x0D {
			if msgStart := p.findASTMMessageHeader(attrValue, 4); msgStart >= 0 {
				messages = append(messages, p.parseASTMBeaconMessages(attrValue[msgStart:])...)
			}
		}
		offset += 3 + attrLen
	}
	return messages
}

func (p *RemoteIDParser) parseVendorIE(raw []byte) ([]types.DroneMessage, error) {
	var messages []types.DroneMessage

	// 1. 搜索标准 ASD-STAN OUI
	for idx := 0; idx <= len(raw)-5; idx++ {
		if string(raw[idx:idx+3]) != asdStanOUI || raw[idx+3] != asdStanOUIType {
			continue
		}
		dataStart := idx + 4
		if dataStart >= len(raw) {
			continue
		}

		if p.isGB46750Format(raw, dataStart) {
			if msgs := p.parseGB46750Payload(raw[dataStart:]); len(msgs) > 0 {
				return append(messages, msgs...), nil
			}
		}
		if msgStart := p.findASTMMessageHeader(raw, dataStart); msgStart >= 0 {
			if msgs := p.parseASTMBeaconMessages(raw[msgStart:]); len(msgs) > 0 {
				return append(messages, msgs...), nil
			}
		}
	}

	// 2. 搜索旧版 ASTM OUI
	for idx := 0; idx <= len(raw)-5; idx++ {
		if string(raw[idx:idx+3]) != legacyASTMOUI || raw[idx+3] != legacyASTMOUIType {
			continue
		}
		if msgStart := p.findASTMMessageHeader(raw, idx+4); msgStart >= 0 {
			if msgs := p.parseASTMBeaconMessages(raw[msgStart:]); len(msgs) > 0 {
				return append(messages, msgs...), nil
			}
		}
	}
	return messages, nil
}

func (p *RemoteIDParser) isGB46750Format(raw []byte, dataStart int) bool {
	if dataStart+7 > len(raw) {
		return false
	}
	if raw[dataStart+1] != gb46750Magic {
		return false
	}
	return ((raw[dataStart+2] >> 5) & 0x07) == gb46750MajorVersion
}

// findASTMMessageHeader 在 raw 中查找 ASTM 或 GB 消息 Header
// scanStart 通常指向 OUI+Type 之后的第一个字节 (即 MsgCounter)。
// Header 紧跟在 MsgCounter 之后，因此我们从 scanStart + 1 开始扫描。
func (p *RemoteIDParser) findASTMMessageHeader(raw []byte, scanStart int) int {
	start := scanStart + 1 // 跳过 Message Counter
	maxScan := start + 2   // 最多允许 2 字节的偏移，应对非标准 padding
	if maxScan > len(raw) {
		maxScan = len(raw)
	}

	for i := start; i < maxScan; i++ {
		b := raw[i]
		msgType := (b >> 4) & 0x0F
		protoVer := b & 0x0F

		// 1. 标准格式：高4位=消息类型(0-5 或 15(Packed)), 低4位=协议版本(1=GB, 2=ASTM)
		// 注意：容忍 protoVer == 0，因为某些固件在 Packed 内部消息中会错误地将 version 设为 0
		if (msgType <= 5 || msgType == 15) && (protoVer == 1 || protoVer == 2 || protoVer == 0) {
			return i
		}

		// 2. 兼容极旧版格式：高4位=协议版本(1或2), 低4位=消息类型(0-5)
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
				var data map[string]string
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
						RawHex:      fmt.Sprintf("%X", innerMsgData), // 保留调试信息
					})
				}
				offset += singleMsgSize
			}
			// Packed Message 处理完毕，通常一个 IE 中只有一个 Pack，退出循环
			break
		}
		// ==============================================================

		// 处理普通的单条消息 (MsgType 0-5)
		var messageType string
		var data map[string]string
		var standard string

		if protoVer == gbProtocolVersion {
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
				RawHex:      fmt.Sprintf("%X", msgData),
			})
		}
		offset += msgSize
	}

	return messages
}
