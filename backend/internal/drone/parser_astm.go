package drone

import (
	"bytes"
	"encoding/binary"
)

type ASTMParser struct{}

func init() {
	DefaultRegistry.RegisterParser(&ASTMParser{})
}

func (p *ASTMParser) Name() string { return "ASTM-F3411-22a" }

func (p *ASTMParser) Match(payload []byte) bool {
	// 匹配前缀: 大包头 Type=0xF0, 长度=0x19 (25字节)
	// 注：这是你们网关/SDK的特定封装，标准 ASTM 裸包没有 0xF0 包头
	return len(payload) >= 3 && payload[0] == 0xF0 && payload[1] == 0x19
}

func (p *ASTMParser) Parse(payload []byte) (*UnpackedTelemetry, error) {
	// 💡 修正 1：放宽长度校验。ASTM 每个消息块 25 字节 + 3字节 Header = 28 字节。
	// 原代码 78 字节要求太严格，会丢弃只包含 1 个消息块的合法数据包。
	if len(payload) < 28 {
		return nil, ErrPacketTooShort
	}

	telemetry := &UnpackedTelemetry{Protocol: p.Name()}

	// 跳过 3 字节 Header
	msgPack := payload[3:]

	const msgBlockSize = 25

	// 💡 修正 2：使用循环遍历，按 25 字节安全切片。
	// 彻底消除 msgPack[25:50] 硬编码带来的 Panic 隐患，并支持解析任意数量的消息块。
	for i := 0; i+msgBlockSize <= len(msgPack); i += msgBlockSize {
		block := msgPack[i : i+msgBlockSize]
		msgType := block[0] // 第一个字节是 Message Type

		switch msgType {
		case 0x00: // 块 1: Basic ID (UAS ID)
			idBytes := block[2:22]
			idBytes = bytes.TrimRight(idBytes, "\x00")
			telemetry.UASID = string(idBytes)

		case 0x01: // 块 2: Location/Vector
			// 💡 修正 3：ASTM 标准中 Location 的 Message Type 是 0x01！原代码的 0x10 是错的。
			// (如果你们的硬件魔改成了 0x10，请改为 case 0x01, 0x10:)

			// 注意：保留了你们原有的偏移量逻辑 (block[2] 是 Heading)
			telemetry.Heading = float64(block[2]) * 1.5
			telemetry.Speed = float64(block[3]) * 0.25

			latRaw := int32(binary.LittleEndian.Uint32(block[5:9]))
			lngRaw := int32(binary.LittleEndian.Uint32(block[9:13]))
			telemetry.Latitude = float64(latRaw) / 1e7
			telemetry.Longitude = float64(lngRaw) / 1e7

			altGeoRaw := binary.LittleEndian.Uint16(block[15:17])
			heightRaw := binary.LittleEndian.Uint16(block[17:19])

			telemetry.Altitude = float64(altGeoRaw)*0.5 - 1000.0
			telemetry.Height = float64(heightRaw)*0.5 - 1000.0

		case 0x03: // 🆕 块 3: Self-ID (操作员 ID / 文本描述) - 【补齐】
			descBytes := block[2:25]
			descBytes = bytes.TrimRight(descBytes, "\x00")
			telemetry.OperatorID = string(descBytes) // 复用 OperatorID 字段，或新增 Description 字段

		case 0x04: // 🆕 块 4: System (操作员位置 / 起飞位置 / 时间戳) - 【补齐】
			// 1. 解析操作员位置 (Operator Location)
			opLatRaw := int32(binary.LittleEndian.Uint32(block[3:7]))
			opLngRaw := int32(binary.LittleEndian.Uint32(block[7:11]))
			telemetry.OperatorLat = float64(opLatRaw) / 1e7
			telemetry.OperatorLng = float64(opLngRaw) / 1e7

			// 2. 解析起飞位置 (Takeoff Location)
			takeoffLatRaw := int32(binary.LittleEndian.Uint32(block[11:15]))
			takeoffLngRaw := int32(binary.LittleEndian.Uint32(block[15:19]))
			telemetry.TakeoffLat = float64(takeoffLatRaw) / 1e7
			telemetry.TakeoffLng = float64(takeoffLngRaw) / 1e7

			// 3. 解析时间戳 (Timestamp, 从 UTC 午夜开始的秒数)
			timestampRaw := binary.LittleEndian.Uint32(block[19:23])
			telemetry.Timestamp = timestampRaw
		}
	}

	return telemetry, nil
}
