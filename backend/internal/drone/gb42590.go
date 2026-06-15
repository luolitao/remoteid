package drone

import (
	"encoding/binary"
	"fmt"
)

func (p *RemoteIDParser) decodeGBMessage(msgData []byte, msgType uint8) (string, map[string]string) {
	payload := msgData[1:]
	data := make(map[string]string)
	var messageType string

	switch msgType {
	case 0:
		messageType = "basic_id"
		data["uas_id"] = cleanString(payload[1:21], "UNKNOWN")
		data["ua_type"] = lookupName(gbUATypeNames, int(payload[0]&0x0F))
		data["id_type"] = lookupName(gbIDTypeNames, int((payload[0]>>4)&0x0F))
	case 1:
		messageType = "location"
		p.decodeGBLocation(payload, data)
	case 2:
		messageType = "authentication"
	case 3:
		messageType = "self_id"
		data["description"] = cleanString(payload[1:24], " ")
	case 4:
		messageType = "system"
		p.decodeGBSystem(payload, data)
	case 5:
		messageType = "operator_id"
		data["operator_id"] = cleanString(payload[1:21], " ")
	}
	return messageType, data
}

func (p *RemoteIDParser) decodeGBLocation(payload []byte, data map[string]string) {
	if len(payload) < 22 {
		return
	}

	status := (payload[0] >> 4) & 0x0F
	data["status"] = lookupName(gbStatusNames, int(status))

	// [修复 Bug] GB 方向编码：12位，无效值为 0x0FFF，分辨率 360.0 / 4096.0
	directionRaw := (uint16(payload[0]&0x0F) << 8) | uint16(payload[1])
	if directionRaw != 0x0FFF {
		data["direction"] = fmt.Sprintf("%.2f", float64(directionRaw)*360.0/4096.0)
	}

	if payload[2] != 255 {
		data["speed_h"] = fmt.Sprintf("%.2f", float64(payload[2])*0.25)
	}
	if payload[3] != 255 {
		data["speed_v"] = fmt.Sprintf("%.2f", float64(int16(payload[3])-128)*0.5)
	}

	if lat, ok := parseCoordLE(payload, 4, 90.0); ok {
		data["latitude"] = fmt.Sprintf("%.7f", lat)
	}
	if lon, ok := parseCoordLE(payload, 8, 180.0); ok {
		data["longitude"] = fmt.Sprintf("%.7f", lon)
	}
	if alt, ok := parseAltitudeLE(payload, 12); ok {
		data["altitude_baro"] = fmt.Sprintf("%.2f", alt)
	}
	if alt, ok := parseAltitudeLE(payload, 14); ok {
		data["altitude_geo"] = fmt.Sprintf("%.2f", alt)
	}
	if h, ok := parseHeightLE(payload, 16); ok {
		data["height_m"] = fmt.Sprintf("%.2f", h)
	}

	data["height_type"] = "AboveTakeoff"
	data["h_accuracy"] = lookupName(gbAccuracyNames, int(payload[18]&0x0F))
	data["v_accuracy"] = lookupName(gbAccuracyNames, int((payload[18]>>4)&0x0F))
	data["baro_accuracy"] = lookupName(gbAccuracyNames, int((payload[19]>>4)&0x0F))
	data["s_accuracy"] = lookupName(gbSpeedAccuracyNames, int(payload[19]&0x0F))
	data["timestamp"] = fmt.Sprintf("%.1f", float64(binary.LittleEndian.Uint16(payload[20:22]))*0.1)
}

func (p *RemoteIDParser) decodeGBSystem(payload []byte, data map[string]string) {
	if len(payload) < 18 {
		return
	}

	// [修复 Bug] GB 42590 中 Classification 占低 3 位，Operator Location Type 占接下来的 2 位
	opLocType := (payload[0] >> 3) & 0x03
	classification := payload[0] & 0x07

	data["flags"] = fmt.Sprintf("0x%02X", payload[0])
	data["operator_loc_type"] = lookupName(gbOpLocTypeNames, int(opLocType))
	data["classification"] = lookupName(gbClassificationNames, int(classification))

	if lat, ok := parseCoordLE(payload, 1, 90.0); ok {
		data["operator_lat"] = fmt.Sprintf("%.7f", lat)
	}
	if lon, ok := parseCoordLE(payload, 5, 180.0); ok {
		data["operator_lon"] = fmt.Sprintf("%.7f", lon)
	}

	data["area_count"] = fmt.Sprintf("%d", binary.LittleEndian.Uint16(payload[9:11]))
	data["area_radius_m"] = fmt.Sprintf("%d", payload[11])

	if alt, ok := parseAltitudeLE(payload, 12); ok {
		data["area_ceiling"] = fmt.Sprintf("%.2f", alt)
	}
	if alt, ok := parseAltitudeLE(payload, 14); ok {
		data["area_floor"] = fmt.Sprintf("%.2f", alt)
	}
	if alt, ok := parseAltitudeLE(payload, 16); ok {
		data["operator_alt"] = fmt.Sprintf("%.2f", alt)
	}
}

// ========== GB 42590 查表常量 ==========
var gbUATypeNames = []string{"None/NotDeclared", "Aeroplane/FixedWing", "Helicopter/Multirotor", "Gyroplane", "HybridLift", "Ornithopter", "Glider", "Kite", "FreeBalloon", "CaptiveBalloon", "Airship", "FreeFall/Parachute", "Rocket", "TetheredPowered", "GroundObstacle", "Other"}
var gbIDTypeNames = []string{"None", "SerialNumber", "CAARegistrationID", "UTMAssignedUUID", "SpecificSessionID"}
var gbStatusNames = []string{"Undeclared", "Ground", "Airborne", "Emergency", "RemoteIDSystemFailure"}
var gbClassificationNames = []string{"Undefined", "USA", "China", "EU", "UK", "Japan", "Australia", "Other"}
var gbOpLocTypeNames = []string{"TakeOff", "Dynamic", "Fixed"}
var gbAccuracyNames = []string{"Unknown", "<10m", "<3m", "<1m"}
var gbSpeedAccuracyNames = []string{"Unknown", "<10m/s", "<3m/s", "<1m/s", "<0.3m/s"}
