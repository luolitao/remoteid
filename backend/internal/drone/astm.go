package drone

import (
	"encoding/binary"
	"fmt"
)

func (p *RemoteIDParser) decodeASTMMessage(msgData []byte, msgType uint8) (string, map[string]string) {
	payload := msgData[1:]
	data := make(map[string]string)
	var messageType string

	switch msgType {
	case 0:
		messageType = "basic_id"
		data["uas_id"] = cleanString(payload[1:21], "UNKNOWN")
		data["ua_type"] = lookupName(astmUATypeNames, int(payload[0]&0x0F))
		data["id_type"] = lookupName(astmIDTypeNames, int((payload[0]>>4)&0x0F))
	case 1:
		messageType = "location"
		p.decodeASTMLocation(payload, data)
	case 2:
		messageType = "authentication"
	case 3:
		messageType = "self_id"
		data["description"] = cleanString(payload[1:24], " ")
	case 4:
		messageType = "system"
		p.decodeASTMSystem(payload, data)
	case 5:
		messageType = "operator_id"
		data["operator_id"] = cleanString(payload[1:21], " ")
	}
	return messageType, data
}

func (p *RemoteIDParser) decodeASTMLocation(payload []byte, data map[string]string) {
	if len(payload) < 22 {
		return
	}

	speedMult := payload[0] & 0x01
	ewDirection := (payload[0] >> 1) & 0x01
	heightType := (payload[0] >> 2) & 0x01
	status := (payload[0] >> 4) & 0x0F

	direction := float64(payload[1])
	if ewDirection == 1 {
		direction += 180.0
	}

	speedFactor := 0.25
	if speedMult == 1 {
		speedFactor = 0.75
	}
	speedH := float64(payload[2]) * speedFactor
	speedV := float64(int8(payload[3])-64) * 0.5

	data["status"] = lookupName(astmStatusNames, int(status))
	data["direction"] = fmt.Sprintf("%.2f", direction)
	data["speed_h"] = fmt.Sprintf("%.2f", speedH)
	data["speed_v"] = fmt.Sprintf("%.2f", speedV)

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
	if heightType == 1 {
		data["height_type"] = "AboveGround"
	}

	data["h_accuracy"] = lookupName(astmAccuracyNames, int(payload[18]&0x0F))
	data["v_accuracy"] = lookupName(astmAccuracyNames, int((payload[18]>>4)&0x0F))
	data["baro_accuracy"] = lookupName(astmAccuracyNames, int((payload[19]>>4)&0x0F))
	data["s_accuracy"] = lookupName(astmSpeedAccuracyNames, int(payload[19]&0x0F))
	data["timestamp"] = fmt.Sprintf("%.1f", float64(binary.LittleEndian.Uint16(payload[20:22]))*0.1)
}

func (p *RemoteIDParser) decodeASTMSystem(payload []byte, data map[string]string) {
	if len(payload) < 18 {
		return
	}

	opLocType := (payload[0] >> 2) & 0x03
	classification := payload[0] & 0x03

	data["flags"] = fmt.Sprintf("0x%02X", payload[0])
	data["operator_loc_type"] = lookupName(astmOpLocTypeNames, int(opLocType))
	data["classification"] = lookupName(astmClassificationNames, int(classification))

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

// ========== ASTM 查表常量 ==========
var astmUATypeNames = []string{"None", "Aeroplane", "Helicopter/Multirotor", "Gyroplane", "VTOL", "Ornithopter", "Glider", "Kite", "FreeBalloon", "CaptiveBalloon", "Airship", "FreeFall/Parachute", "Rocket", "TetheredPowered", "GroundObstacle", "Other"}
var astmIDTypeNames = []string{"None", "SerialNumber", "CAARegistrationID", "UTMID", "SpecificSessionID", "Other"}
var astmStatusNames = []string{"Undeclared", "Ground", "Airborne", "Emergency", "RemoteIDSystemFailure", "Emergency(EU)", "Reserved6", "Reserved7"}
var astmClassificationNames = []string{"Undeclared", "EU", "Reserved2", "Reserved3", "Reserved4", "Reserved5", "Reserved6", "Reserved7"}
var astmOpLocTypeNames = []string{"TakeOff", "Dynamic", "Fixed"}
var astmAccuracyNames = []string{"Unknown", "<10m", "<3m", "<1m"}
var astmSpeedAccuracyNames = []string{"Unknown", "<10m/s", "<3m/s", "<1m/s", "<0.3m/s"}
