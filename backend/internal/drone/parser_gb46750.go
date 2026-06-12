package drone

import (
	"bytes"
	"encoding/binary"
)

type GB46750Parser struct{}

func init() {
	DefaultRegistry.RegisterParser(&GB46750Parser{})
}

func (p *GB46750Parser) Name() string { return "GB46750-2025(变长)" }

func (p *GB46750Parser) Match(payload []byte) bool {
	// 新国标变长无线电特征：通常在厂商特定元素(IE 221)中包含特有魔数，如 payload[1] == 0xFF
	return len(payload) > 4 && payload[0] == 0xDD && payload[1] == 0xFF
}

func (p *GB46750Parser) Parse(payload []byte) (*UnpackedTelemetry, error) {
	// 安全拦截防止越界
	if len(payload) < 12 {
		return nil, ErrPacketTooShort
	}

	telemetry := &UnpackedTelemetry{Protocol: p.Name()}

	// 1. 读取 3 字节变长标志位掩码表 (Bitmask)
	// 每一个 Bit 代表空口中是否携带该数据项（1为携带，0为不携带，指针据此累加）
	bitmask := uint32(payload[2]) | uint32(payload[3])<<8 | uint32(payload[4])<<16

	offset := 5 // 数据解析游标

	// 2. 检查 Bit 0: 是否包含 UAS ID (通常国标为 20 或 32 字节变长串)
	if bitmask&0x000001 != 0 {
		idLen := 20 // 或者是根据标准约定的变长
		if len(payload) < offset+idLen {
			return nil, ErrPacketTooShort
		}
		telemetry.UASID = string(bytes.TrimRight(payload[offset:offset+idLen], "\x00"))
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

	// 4. 检查 Bit 2: 是否包含 气压/几何高度 (2 字节，新国标特有映射：真实高度 = 编码值 * 0.5 - 9000)
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

	return telemetry, nil
}
