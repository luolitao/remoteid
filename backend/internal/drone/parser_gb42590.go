package drone

import (
	"bytes"
	"encoding/binary"
)

type GB42590Parser struct{}

func init() {
	DefaultRegistry.RegisterParser(&GB42590Parser{})
}

func (p *GB42590Parser) Name() string { return "GB42590-2023" }

func (p *GB42590Parser) Match(payload []byte) bool {
	// 国标固定长特有大包头标识: Type 0xF1
	return len(payload) >= 3 && payload[0] == 0xF1 && payload[1] == 0x19
}

func (p *GB42590Parser) Parse(payload []byte) (*UnpackedTelemetry, error) {
	if len(payload) < 78 {
		return nil, ErrPacketTooShort
	}

	telemetry := &UnpackedTelemetry{Protocol: p.Name()}
	msgPack := payload[3:]

	// 块 1: Basic ID 消息 (ID Type 0x22 代表国标唯一产品识别码 UIN)
	if msgPack[0] == 0x00 && msgPack[1] == 0x22 {
		idBytes := msgPack[2:22]
		idBytes = bytes.TrimRight(idBytes, "\x00")
		telemetry.UASID = string(idBytes)
	}

	// 块 2: Location 消息
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
