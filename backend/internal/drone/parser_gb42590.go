package drone

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"log"
)

type GB42590Parser struct{}

func init() {
	DefaultRegistry.RegisterParser(&GB42590Parser{})
}

func (p *GB42590Parser) Name() string { return "GB42590-2023 + IB-TM-2024-01" }

// 辅助函数：计算有效载荷的起始偏移量
func (p *GB42590Parser) getOffset(payload []byte) int {
	// 判断是否包含标准的 FA 0B BC 0D + 1字节Counter OUI 头
	if len(payload) >= 8 && payload[0] == 0xFA && payload[1] == 0x0B && payload[2] == 0xBC {
		return 5 // 跳过前 5 个字节的壳
	}

	return 0 // 假设已经是纯净数据
}

func (p *GB42590Parser) Match(payload []byte) bool {
	offset := p.getOffset(payload)
	// 确保切片不会越界
	if len(payload) < offset+3 {
		return false
	}
	if DebugMode {
		log.Printf("[DEBUG-PARSER] GB42590 payload解析: 长度: %d 字节 | 原始Hex: %s", len(payload), hex.EncodeToString(payload))
	}

	// 从 offset 处开始判断国标固定长特有大包头标识: Type 0xF1 0x19
	return payload[offset] == 0xF1 && payload[offset+1] == 0x19
}

func (p *GB42590Parser) Parse(payload []byte) (*UnpackedTelemetry, error) {
	offset := p.getOffset(payload)

	// 将 payload 指针直接移到纯净数据层
	cleanPayload := payload[offset:]

	// 重新评估长度限制（去壳后，纯净负载至少需要足够长）
	if len(cleanPayload) < 75 { // 注意：原始是 78，去壳 5 字节后需调整长度校验，这里假设个安全值
		return nil, ErrPacketTooShort
	}

	telemetry := &UnpackedTelemetry{Protocol: p.Name()}

	// 由于 cleanPayload 已经是 [F1, 19, XX, ...]，所以 msgPack 从索引 3 开始
	msgPack := cleanPayload[3:]

	// 块 1: Basic ID 消息 (ID Type 0x22 代表国标唯一产品识别码 UIN)
	if len(msgPack) >= 22 && msgPack[0] == 0x00 && msgPack[1] == 0x22 {
		idBytes := msgPack[2:22]
		idBytes = bytes.TrimRight(idBytes, "\x00")
		telemetry.UASID = string(idBytes)
	}

	// 块 2: Location 消息
	// 确保 msgPack 长度足够取 25:50
	if len(msgPack) >= 50 {
		locMsg := msgPack[25:50]
		if locMsg[0] == 0x10 {
			telemetry.Heading = float64(locMsg[2]) * 1.5
			telemetry.Speed = float64(locMsg[3]) * 0.25

			latRaw := int32(binary.LittleEndian.Uint32(locMsg[5:9]))
			lngRaw := int32(binary.LittleEndian.Uint32(locMsg[9:13]))
			telemetry.Latitude = float64(latRaw) / 1e7
			telemetry.Longitude = float64(lngRaw) / 1e7

			altGeoRaw := binary.LittleEndian.Uint16(locMsg[15:17])
			heightRaw := binary.LittleEndian.Uint16(locMsg[17:19])
			telemetry.Altitude = float64(altGeoRaw)*0.5 - 1000.0
			telemetry.Height = float64(heightRaw)*0.5 - 1000.0
		}
	}

	return telemetry, nil
}
