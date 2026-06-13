package drone

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"log/slog"
)

type GB42590Parser struct{}

func init() {
	DefaultRegistry.RegisterParser(&GB42590Parser{})
}

func (p *GB42590Parser) Name() string {
	return "GB42590-2023 + IB-TM-2024-01"
}

// getOffset 计算有效载荷的起始偏移量（跳过 OUI+Counter 外壳）
func (p *GB42590Parser) getOffset(payload []byte) int {
	if len(payload) >= 5 && payload[0] == 0xFA && payload[1] == 0x0B && payload[2] == 0xBC && payload[3] == 0x0D {
		return 5
	}
	return 0
}

// Match 检查是否匹配 GB42590 协议特征头
func (p *GB42590Parser) Match(payload []byte) bool {
	offset := p.getOffset(payload)
	if len(payload) < offset+3 {
		return false
	}
	// 国标固定大包头标识: 0xF1 0x19
	return payload[offset] == 0xF1 && payload[offset+1] == 0x19
}

// Parse 深度解析 GB42590 数据包（已修正偏移与字节序）
func (p *GB42590Parser) Parse(payload []byte) (*UnpackedTelemetry, error) {
	offset := p.getOffset(payload)
	cleanPayload := payload[offset:]

	// 基础长度校验: 3字节Header + 至少1个25字节消息块
	if len(cleanPayload) < 28 {
		return nil, ErrPacketTooShort
	}

	telemetry := &UnpackedTelemetry{Protocol: p.Name()}
	msgPack := cleanPayload[3:] // 跳过 0xF1 0x19 0xXX

	if DebugMode {
		slog.Debug("🔍 GB42590 开始解析",
			"msgPack_len", len(msgPack),
			"raw_hex_prefix", hex.EncodeToString(msgPack[:min(10, len(msgPack))]),
		)
	}

	const msgBlockSize = 25

	for i := 0; i+msgBlockSize <= len(msgPack); i += msgBlockSize {
		block := msgPack[i : i+msgBlockSize]
		msgType := block[0]

		switch msgType {
		case 0x00, 0x01: // Basic ID 消息 (UAS ID)
			// 结构: [Type(1)][ID_Type(1)][UAS_ID(20)][保留(3)]
			idBytes := block[2:22]
			idBytes = bytes.TrimRight(idBytes, "\x00")
			if len(idBytes) > 0 {
				telemetry.UASID = string(idBytes)
			}

		case 0x10, 0x11: // Location/Vector 消息 🎯 核心修复
			// ✅ 速度: block[2]，步长 0.25 m/s
			telemetry.Speed = float64(block[2]) * 0.25

			// ✅ 航向: block[3:5]，Big-Endian UInt16，单位: 度 (0-360)
			telemetry.Heading = float64(binary.BigEndian.Uint16(block[3:5]))

			// ✅ 纬度: block[5:9]，Little-Endian Int32，缩放 1e-7
			latRaw := int32(binary.LittleEndian.Uint32(block[5:9]))
			telemetry.Latitude = float64(latRaw) / 1e7

			// ✅ 经度: block[9:13]，Little-Endian Int32，缩放 1e-7
			lngRaw := int32(binary.LittleEndian.Uint32(block[9:13]))
			telemetry.Longitude = float64(lngRaw) / 1e7

			// ✅ 海拔: block[13:15]，Little-Endian UInt16，步长 0.5m，偏移 -1000m
			altRaw := binary.LittleEndian.Uint16(block[13:15])
			telemetry.Altitude = float64(altRaw)*0.5 - 1000.0

			// ✅ 相对高度: block[15:17]，Little-Endian UInt16，步长 0.5m，偏移 -1000m
			heightRaw := binary.LittleEndian.Uint16(block[15:17])
			telemetry.Height = float64(heightRaw)*0.5 - 1000.0

			if DebugMode {
				slog.Debug("📍 GB42590 Location 解析结果",
					"speed", telemetry.Speed,
					"heading", telemetry.Heading,
					"lat", telemetry.Latitude,
					"lng", telemetry.Longitude,
					"alt", telemetry.Altitude,
					"height", telemetry.Height,
				)
			}

		case 0x30, 0x31: // Self-Ad / Operator ID 消息
			opIDBytes := block[2:22]
			opIDBytes = bytes.TrimRight(opIDBytes, "\x00")
			if len(opIDBytes) > 0 {
				telemetry.OperatorID = string(opIDBytes)
			}

		case 0x40, 0x41: // System / Operator Location 消息
			if len(block) >= 19 {
				opLatRaw := int32(binary.LittleEndian.Uint32(block[3:7]))
				opLngRaw := int32(binary.LittleEndian.Uint32(block[7:11]))
				telemetry.OperatorLat = float64(opLatRaw) / 1e7
				telemetry.OperatorLng = float64(opLngRaw) / 1e7
			}
			if len(block) >= 23 {
				telemetry.Timestamp = binary.LittleEndian.Uint32(block[19:23])
			}
		}
	}

	return telemetry, nil
}
