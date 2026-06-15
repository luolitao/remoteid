package drone

import (
	"bytes"
	"strings"
)

// DeviceClassifier 负责根据数据包特征对设备进行分类
type DeviceClassifier struct{}

func NewClassifier() *DeviceClassifier { return &DeviceClassifier{} }

// ClassifyPacket 根据帧类型和原始数据分类设备
func (c *DeviceClassifier) ClassifyPacket(frameSubtype uint8, raw []byte) string {
	if frameSubtype == 13 { // Action Frame (NAN)
		if c.isValidNANRemoteID(raw) {
			return "drone"
		}
	} else {
		if c.isValidRemoteID(raw) {
			return "drone"
		}
	}
	return "normal"
}

// isValidRemoteID 检查 Beacon 中是否包含有效的 Remote ID OUI 及消息头
func (c *DeviceClassifier) isValidRemoteID(raw []byte) bool {
	for i := 0; i < len(raw)-4; i++ {
		// 检查 ASD-STAN OUI
		if raw[i] == 0xFA && raw[i+1] == 0x0B && raw[i+2] == 0xBC && raw[i+3] == 0x0D {
			if c.checkGB46750OrASTMHeader(raw, i+4) {
				return true
			}
		}
		// 检查 Legacy ASTM OUI
		if raw[i] == 0x06 && raw[i+1] == 0x05 && raw[i+2] == 0x04 && raw[i+3] == 0xFD {
			if c.checkASTMHeader(raw, i+4) {
				return true
			}
		}
	}
	return false
}

func (c *DeviceClassifier) checkGB46750OrASTMHeader(raw []byte, dataStart int) bool {
	if dataStart+7 <= len(raw) && raw[dataStart+1] == 0xFF && ((raw[dataStart+2]>>5)&0x07) == 0x1 {
		return true // GB 46750
	}
	return c.checkASTMHeader(raw, dataStart)
}

func (c *DeviceClassifier) checkASTMHeader(raw []byte, start int) bool {
	maxScan := start + 8
	if maxScan > len(raw) {
		maxScan = len(raw)
	}
	for j := start; j < maxScan; j++ {
		b := raw[j]
		msgType, protoVer := (b>>4)&0x0F, b&0x0F
		if msgType <= 5 && (protoVer == 1 || protoVer == 2) {
			return true
		}
	}
	return false
}

// isValidNANRemoteID 检查 Action Frame 中是否包含 NAN Remote ID 特征
func (c *DeviceClassifier) isValidNANRemoteID(raw []byte) bool {
	hasWiFiAlliance, hasRemoteID := false, false
	for i := 0; i < len(raw)-3; i++ {
		if raw[i] == 0x50 && raw[i+1] == 0x6F && raw[i+2] == 0x9A && i+3 < len(raw) && raw[i+3] == 0x13 {
			hasWiFiAlliance = true
		}
		if raw[i] == 0xFA && raw[i+1] == 0x0B && raw[i+2] == 0xBC && i+3 < len(raw) && raw[i+3] == 0x0D {
			hasRemoteID = true
		}
	}
	return hasWiFiAlliance && hasRemoteID
}

// ExtractSSID 从数据包中提取 SSID (简化版逻辑)
func (c *DeviceClassifier) ExtractSSID(raw []byte) string {
	if idx := bytes.Index(raw, []byte("DJI_")); idx != -1 {
		end := idx + 32
		if end > len(raw) {
			end = len(raw)
		}
		candidate := string(raw[idx:end])
		if nullIdx := strings.Index(candidate, "\x00"); nullIdx != -1 {
			candidate = candidate[:nullIdx]
		}
		if candidate != "" && len(candidate) > 4 {
			return candidate
		}
	}
	if bytes.Contains(raw, []byte("CAAC")) || bytes.Contains(raw, []byte("GB425")) || bytes.Contains(raw, []byte{0xFA, 0x0B, 0xBC}) {
		return "CAAC_Device"
	}
	return "Unknown"
}
