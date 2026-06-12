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
	return len(payload) >= 3 && payload[0] == 0xF0 && payload[1] == 0x19
}

func (p *ASTMParser) Parse(payload []byte) (*UnpackedTelemetry, error) {
	if len(payload) < 78 { // 3(Header) + 25*3 = 78 字节
		return nil, ErrPacketTooShort
	}

	telemetry := &UnpackedTelemetry{Protocol: p.Name()}
	msgPack := payload[3:]

	// 块 1: Basic ID 消息 (偏移 0 - 25)
	if msgPack[0] == 0x00 {
		idBytes := msgPack[2:22]
		idBytes = bytes.TrimRight(idBytes, "\x00")
		telemetry.UASID = string(idBytes)
	}

	// 块 2: Location 消息 (偏移 25 - 50)
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

	return telemetry, nil
}
