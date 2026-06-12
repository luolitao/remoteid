package drone

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"log/slog"
)

type GB46750Parser struct{}

func init() {
	DefaultRegistry.RegisterParser(&GB46750Parser{})
}

func (p *GB46750Parser) Name() string { return "GB46750-2025(变长)" }

func (p *GB46750Parser) Match(payload []byte) bool {
	// 新国标变长无线电特征：通常在厂商特定元素(IE 221)中包含特有魔数
	return len(payload) > 4 && payload[0] == 0xDD && payload[1] == 0xFF
}

func (p *GB46750Parser) Parse(payload []byte) (*UnpackedTelemetry, error) {
	// 💡 修正 1：放宽初始长度校验。Header(2字节) + Bitmask(3字节) = 5 字节。
	// 原代码 12 字节过严，如果无人机只广播了 ID 或只广播了位置，会被误判为 TooShort。
	if len(payload) < 5 {
		return nil, ErrPacketTooShort
	}

	telemetry := &UnpackedTelemetry{Protocol: p.Name()}

	// 1. 读取 3 字节变长标志位掩码表 (Bitmask) - 小端序
	// 每一个 Bit 代表空口中是否携带该数据项（1为携带，0为不携带，指针据此累加）
	bitmask := uint32(payload[2]) | uint32(payload[3])<<8 | uint32(payload[4])<<16

	// 💡 新增：打印 Bitmask 方便调试变长协议
	slog.Debug("GB46750 payload解析",
		"bitmask_hex", hex.EncodeToString(payload[2:5]),
		"payload_len", len(payload),
	)

	offset := 5 // 数据解析游标

	// 2. 检查 Bit 0: 是否包含 UAS ID
	if bitmask&0x000001 != 0 {
		// ⚠️ 注意：这里假设 UAS ID 固定为 20 字节（与 ASTM 保持一致）。
		// 如果你们的新国标是“真变长”（即前面有 1 字节表示长度），需要改为：
		// idLen := int(payload[offset]); offset++;
		idLen := 20
		if len(payload) < offset+idLen {
			return nil, ErrPacketTooShort
		}
		idBytes := payload[offset : offset+idLen]
		telemetry.UASID = string(bytes.TrimRight(idBytes, "\x00"))
		offset += idLen
	}

	// 3. 检查 Bit 1: 是否包含 经纬度 坐标 (4 + 4 = 8 字节)
	if bitmask&0x000002 != 0 {
		if len(payload) < offset+8 {
			return nil, ErrPacketTooShort
		}
		latRaw := int32(binary.LittleEndian.Uint32(payload[offset : offset+4]))
		lngRaw := int32(binary.LittleEndian.Uint32(payload[offset+4 : offset+8]))
		telemetry.Latitude = float64(latRaw) / 1e7
		telemetry.Longitude = float64(lngRaw) / 1e7
		offset += 8
	}

	// 4. 检查 Bit 2: 是否包含 气压/几何高度 (2 字节)
	if bitmask&0x000004 != 0 {
		if len(payload) < offset+2 {
			return nil, ErrPacketTooShort
		}
		altRaw := binary.LittleEndian.Uint16(payload[offset : offset+2])
		telemetry.Altitude = float64(altRaw)*0.5 - 9000.0 // 新国标大动态高度映射公式
		offset += 2
	}

	// 5. 检查 Bit 3: 是否包含 航向与速度 (1 + 1 = 2 字节)
	if bitmask&0x000008 != 0 {
		if len(payload) < offset+2 {
			return nil, ErrPacketTooShort
		}
		telemetry.Heading = float64(payload[offset]) * 1.5
		telemetry.Speed = float64(payload[offset+1]) * 0.25
		offset += 2
	}

	// 🆕 6. 检查 Bit 4: 是否包含 相对高度/离地高度 (2 字节) - 【补齐】
	if bitmask&0x000010 != 0 {
		if len(payload) < offset+2 {
			return nil, ErrPacketTooShort
		}
		heightRaw := binary.LittleEndian.Uint16(payload[offset : offset+2])
		telemetry.Height = float64(heightRaw)*0.5 - 1000.0 // 假设映射公式与 ASTM 一致，如有不同请调整
		offset += 2
	}

	// 🆕 7. 检查 Bit 5: 是否包含 时间戳 (4 字节) - 【补齐】
	if bitmask&0x000020 != 0 {
		if len(payload) < offset+4 {
			return nil, ErrPacketTooShort
		}
		telemetry.Timestamp = binary.LittleEndian.Uint32(payload[offset : offset+4])
		offset += 4
	}

	// 🆕 8. 检查 Bit 6: 是否包含 操作员位置 (4 + 4 = 8 字节) - 【补齐】
	if bitmask&0x000040 != 0 {
		if len(payload) < offset+8 {
			return nil, ErrPacketTooShort
		}
		opLatRaw := int32(binary.LittleEndian.Uint32(payload[offset : offset+4]))
		opLngRaw := int32(binary.LittleEndian.Uint32(payload[offset+4 : offset+8]))
		telemetry.OperatorLat = float64(opLatRaw) / 1e7
		telemetry.OperatorLng = float64(opLngRaw) / 1e7
		offset += 8
	}

	return telemetry, nil
}
