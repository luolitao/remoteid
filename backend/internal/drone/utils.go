package drone

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"
)

// cleanString 清理字符串：去除空字符和不可见字符
func cleanString(b []byte, defaultVal string) string {
	s := strings.TrimRightFunc(string(b), func(r rune) bool {
		return !utf8.ValidRune(r) || r == 0
	})
	s = strings.TrimSpace(s)
	if s == "" {
		return defaultVal
	}
	return s
}

// lookupName 通用查表函数，替代冗长的 switch-case
func lookupName(names []string, idx int) string {
	if idx >= 0 && idx < len(names) {
		return names[idx]
	}
	return fmt.Sprintf("Unknown(%d)", idx)
}

// parseCoordLE 解析小端序经纬度，返回 (值, 是否有效)
func parseCoordLE(b []byte, offset int, maxVal float64) (float64, bool) {
	if len(b) < offset+4 {
		return 0, false
	}
	raw := int32(binary.LittleEndian.Uint32(b[offset : offset+4]))
	if raw == 0x7FFFFFFF {
		return 0, false
	}
	val := float64(raw) / 1e7
	return val, math.Abs(val) <= maxVal
}

// parseAltitudeLE 解析小端序高度 (uint16 LE * 0.5 - 1000)
func parseAltitudeLE(b []byte, offset int) (float64, bool) {
	if len(b) < offset+2 {
		return 0, false
	}
	raw := binary.LittleEndian.Uint16(b[offset : offset+2])
	if raw == 0xFFFF {
		return 0, false
	}
	return float64(raw)*0.5 - 1000.0, true
}

// parseHeightLE 解析小端序相对高度 (uint16 LE * 0.5)
func parseHeightLE(b []byte, offset int) (float64, bool) {
	if len(b) < offset+2 {
		return 0, false
	}
	raw := binary.LittleEndian.Uint16(b[offset : offset+2])
	if raw == 0xFFFF {
		return 0, false
	}
	return float64(raw) * 0.5, true
}
