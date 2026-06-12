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
		// 不需要 if 判断，slog 会自动根据级别决定是否输出
		return false
	}

	// 从 offset 处开始判断国标固定长特有大包头标识: Type 0xF1 0x19
	return payload[offset] == 0xF1 && payload[offset+1] == 0x19
}

func (p *GB42590Parser) Parse(payload []byte) (*UnpackedTelemetry, error) {
	offset := p.getOffset(payload)
	cleanPayload := payload[offset:]

	// 基础长度校验 (至少需要 3字节Header + 1个25字节消息块)
	if len(cleanPayload) < 28 {
		return nil, ErrPacketTooShort
	}

	telemetry := &UnpackedTelemetry{Protocol: p.Name()}

	// 跳过前 3 字节 (PackType 等 Header 信息)
	msgPack := cleanPayload[3:]

	// 💡 优化：将 Warn 改为 Debug，避免正常解析时刷屏
	slog.Debug("GB42590 payload解析",
		"msgPack_len", len(msgPack),
		"raw_hex", hex.EncodeToString(payload[:min(25, len(payload))]),
	)

	// GB/T 42590 标准中，每个消息块固定为 25 字节
	const msgBlockSize = 25

	// 🔄 核心优化：使用循环遍历所有消息块，替代硬编码的 [25:50]，防止 Panic 并支持更多消息
	for i := 0; i+msgBlockSize <= len(msgPack); i += msgBlockSize {
		block := msgPack[i : i+msgBlockSize]
		msgType := block[0] // 第一个字节是消息类型

		switch msgType {
		case 0x00: // 块 1: Basic ID 消息 (UAS ID)
			// 兼容你原有的 0x22 判断，同时兼容标准定义的 0x00~0x05
			if block[1] == 0x22 || block[1] <= 0x05 {
				idBytes := block[2:22]
				idBytes = bytes.TrimRight(idBytes, "\x00")
				telemetry.UASID = string(idBytes)
			}

		case 0x01, 0x10: // 块 2: Location/Vector 消息 (兼容标准 0x01 和你的 0x10)
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

		case 0x03: // 🆕 块 3: Self-Ad 消息 (操作员 ID / 分类) - 【补齐】
			opIDBytes := block[2:22]
			opIDBytes = bytes.TrimRight(opIDBytes, "\x00")
			telemetry.OperatorID = string(opIDBytes) // ⚠️ 需在 UnpackedTelemetry 结构体中补充此字段

			// 可选：解析无人机分类信息
			// telemetry.ClassificationType = block[22]
			// telemetry.Category = block[23]
			// telemetry.Class = block[24]

		case 0x04: // 🆕 块 4: System 消息 (操作员位置 / 起飞位置 / 时间戳 / 紧急状态) - 【补齐】
			// 1. 解析操作员位置 (Operator Location)
			opLatRaw := int32(binary.LittleEndian.Uint32(block[3:7]))
			opLngRaw := int32(binary.LittleEndian.Uint32(block[7:11]))
			telemetry.OperatorLat = float64(opLatRaw) / 1e7 // ⚠️ 需补充字段
			telemetry.OperatorLng = float64(opLngRaw) / 1e7 // ⚠️ 需补充字段

			// 2. 解析起飞位置 (Takeoff Location)
			takeoffLatRaw := int32(binary.LittleEndian.Uint32(block[11:15]))
			takeoffLngRaw := int32(binary.LittleEndian.Uint32(block[15:19]))
			telemetry.TakeoffLat = float64(takeoffLatRaw) / 1e7 // ⚠️ 需补充字段
			telemetry.TakeoffLng = float64(takeoffLngRaw) / 1e7 // ⚠️ 需补充字段

			// 3. 解析时间戳 (Timestamp, 通常是从 UTC 午夜开始的秒数)
			timestampRaw := binary.LittleEndian.Uint32(block[19:23])
			telemetry.Timestamp = timestampRaw // ⚠️ 需补充字段

			// 4. 解析紧急状态 (Emergency Status, 通常在 Flags 字节中)
			// telemetry.Emergency = (block[1] & 0x01) != 0 // ⚠️ 需补充字段
		}
	}

	return telemetry, nil
}
