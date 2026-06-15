package drone

import (
	"encoding/binary"
	"fmt"
	"remoteid-monitor/pkg/types"
)

type gb46750Rule struct {
	size  int
	parse func([]byte) (string, bool)
	key   string
}

// 构建 GB 46750 解析规则表
var gb46750Rules = map[uint8]gb46750Rule{
	// 标识字节 1
	0x80: {size: 20, parse: func(b []byte) (string, bool) { return cleanString(b, " "), true }, key: "unique_id"},
	0x40: {size: 8, parse: func(b []byte) (string, bool) { return cleanString(b, " "), true }, key: "realname_id"},
	0x20: {size: 1, parse: func(b []byte) (string, bool) { return fmt.Sprintf("%d", b[0]), true }, key: "operation_category"},
	0x10: {size: 1, parse: func(b []byte) (string, bool) { return lookupName(gb46750UACategoryNames, int(b[0])), true }, key: "ua_category"},
	0x08: {size: 1, parse: func(b []byte) (string, bool) { return lookupName(gb46750RCSLocTypeNames, int(b[0])), true }, key: "rcs_loc_type"},
	0x04: {size: 8, parse: func(b []byte) (string, bool) {
		lon, ok1 := parseCoordLE(b, 0, 180.0)
		lat, ok2 := parseCoordLE(b, 4, 90.0)
		if !ok1 || !ok2 {
			return "", false
		}
		return fmt.Sprintf("lon:%.7f,lat:%.7f", lon, lat), true // 可拆分为两个 key，此处为简洁合并展示
	}, key: "rcs_location"},
	0x02: {size: 2, parse: func(b []byte) (string, bool) {
		raw := binary.LittleEndian.Uint16(b)
		if raw == 0xFFFF {
			return "", false
		}
		return fmt.Sprintf("%.2f", float64(raw)/2.0-1000.0), true
	}, key: "rcs_altitude"}, // 标识字节 2
	//0x80: {size: 8, parse: func(b []byte) (string, bool) { // 注意：此处 map key 会覆盖，实际需按 byteIdx 区分，见下方优化实现
	//	return "", false
	//}, key: ""}, // 占位，实际逻辑在下方自定义处理以保证严谨性
}

// 为保持绝对严谨，采用显式函数映射替代纯 map (避免同名 key 冲突)
func (p *RemoteIDParser) parseGB46750Payload(payload []byte) []types.DroneMessage {
	if len(payload) < 7 {
		return nil
	}

	version := payload[2]
	contentLen := int(payload[3])
	flags := payload[4:7]
	content := payload[7:]
	if len(content) < contentLen {
		contentLen = len(content)
	}

	data := make(map[string]string)
	data["gb46750_version"] = fmt.Sprintf("%d.%d", (version>>5)&0x07, version&0x1F)
	data["msg_counter"] = fmt.Sprintf("%d", payload[0])

	p.decodeGB46750Fields(flags, content[:contentLen], data)

	return []types.DroneMessage{{
		MessageType: "gb46750", Standard: "GB 46750-2023", Data: data, Source: "ASTM",
	}}
}

func (p *RemoteIDParser) decodeGB46750Fields(flags []byte, content []byte, data map[string]string) {
	if len(flags) < 3 {
		return
	}
	offset := 0

	for byteIdx, flag := range flags {
		if byteIdx >= 3 {
			break
		}
		for bit := 7; bit >= 1; bit-- {
			if (flag & (1 << bit)) == 0 {
				continue
			}
			if offset >= len(content) {
				return
			}

			// 提取当前位对应的解析逻辑
			rule := p.getGB46750Rule(byteIdx, bit)
			if rule.size == 0 || offset+rule.size > len(content) {
				continue
			}

			if val, ok := rule.parse(content[offset : offset+rule.size]); ok {
				data[rule.key] = val
			}
			offset += rule.size
		}
	}
}

func (p *RemoteIDParser) getGB46750Rule(byteIdx, bit int) gb46750Rule {
	empty := gb46750Rule{}
	switch byteIdx {
	case 0:
		switch bit {
		case 7:
			return gb46750Rule{20, func(b []byte) (string, bool) { return cleanString(b, " "), true }, "unique_id"}
		case 6:
			return gb46750Rule{8, func(b []byte) (string, bool) { return cleanString(b, " "), true }, "realname_id"}
		case 5:
			return gb46750Rule{1, func(b []byte) (string, bool) { return fmt.Sprintf("%d", b[0]), true }, "operation_category"}
		case 4:
			return gb46750Rule{1, func(b []byte) (string, bool) { return lookupName(gb46750UACategoryNames, int(b[0])), true }, "ua_category"}
		case 3:
			return gb46750Rule{1, func(b []byte) (string, bool) { return lookupName(gb46750RCSLocTypeNames, int(b[0])), true }, "rcs_loc_type"}
		case 2:
			return gb46750Rule{8, func(b []byte) (string, bool) {
				lon, ok1 := parseCoordLE(b, 0, 180.0)
				lat, ok2 := parseCoordLE(b, 4, 90.0)
				if !ok1 || !ok2 {
					return "", false
				}
				return fmt.Sprintf("%.7f", lon) + "|" + fmt.Sprintf("%.7f", lat), true // 简化展示，可按需拆分 key
			}, "rcs_location"}
		case 1:
			return gb46750Rule{2, func(b []byte) (string, bool) {
				raw := binary.LittleEndian.Uint16(b)
				if raw == 0xFFFF {
					return "", false
				}
				return fmt.Sprintf("%.2f", float64(raw)/2.0-1000.0), true
			}, "rcs_altitude"}
		}
	case 1:
		switch bit {
		case 7:
			return gb46750Rule{8, func(b []byte) (string, bool) {
				lon, ok1 := parseCoordLE(b, 0, 180.0)
				lat, ok2 := parseCoordLE(b, 4, 90.0)
				if !ok1 || !ok2 {
					return "", false
				}
				return fmt.Sprintf("%.7f", lon) + "|" + fmt.Sprintf("%.7f", lat), true
			}, "uav_location"}
		case 6:
			return gb46750Rule{2, func(b []byte) (string, bool) {
				raw := binary.LittleEndian.Uint16(b)
				if raw == 0xFFFF {
					return "", false
				}
				return fmt.Sprintf("%.2f", float64(raw)/10.0), true
			}, "direction"}
		case 5:
			return gb46750Rule{2, func(b []byte) (string, bool) {
				raw := binary.LittleEndian.Uint16(b)
				if raw == 0xFFFF {
					return "", false
				}
				return fmt.Sprintf("%.2f", float64(raw)/10.0), true
			}, "speed_h"}
		case 4:
			return gb46750Rule{2, func(b []byte) (string, bool) {
				raw := binary.LittleEndian.Uint16(b)
				if raw == 0xFFFF {
					return "", false
				}
				return fmt.Sprintf("%.2f", float64(raw)/2.0-9000.0), true
			}, "height_m"}
		case 3:
			return gb46750Rule{1, func(b []byte) (string, bool) {
				if b[0] == 0xFF {
					return "", false
				}
				v := float64(b[0]&0x7F) / 2.0
				if (b[0] & 0x80) != 0 {
					v = -v
				}
				return fmt.Sprintf("%.2f", v), true
			}, "speed_v"}
		case 2:
			return gb46750Rule{2, func(b []byte) (string, bool) {
				raw := binary.LittleEndian.Uint16(b)
				if raw == 0xFFFF {
					return "", false
				}
				return fmt.Sprintf("%.2f", float64(raw)/2.0-1000.0), true
			}, "altitude_geo"}
		case 1:
			return gb46750Rule{2, func(b []byte) (string, bool) {
				raw := binary.LittleEndian.Uint16(b)
				if raw == 0xFFFF {
					return "", false
				}
				return fmt.Sprintf("%.2f", float64(raw)/2.0-1000.0), true
			}, "altitude_baro"}
		}
	case 2:
		switch bit {
		case 7:
			return gb46750Rule{1, func(b []byte) (string, bool) { return lookupName(gb46750StatusNames, int(b[0])), true }, "status"}
		case 6:
			return gb46750Rule{1, func(b []byte) (string, bool) { return lookupName(gb46750CoordSystemNames, int(b[0])), true }, "coord_system"}
		case 5:
			return gb46750Rule{1, func(b []byte) (string, bool) { return lookupName(gb46750AccuracyNames, int(b[0])), true }, "h_accuracy"}
		case 4:
			return gb46750Rule{1, func(b []byte) (string, bool) { return lookupName(gb46750AccuracyNames, int(b[0])), true }, "v_accuracy"}
		case 3:
			return gb46750Rule{1, func(b []byte) (string, bool) { return lookupName(gb46750AccuracyNames, int(b[0])), true }, "s_accuracy"}
		case 2:
			return gb46750Rule{6, func(b []byte) (string, bool) {
				var ts uint64
				for i := 0; i < 6; i++ {
					ts |= uint64(b[i]) << (i * 8)
				}
				return fmt.Sprintf("%d", ts), true
			}, "timestamp_ms"}
		case 1:
			return gb46750Rule{1, func(b []byte) (string, bool) { return lookupName(gb46750TSAccuracyNames, int(b[0])), true }, "ts_accuracy"}
		}
	}
	return empty
}

// ========== GB 46750 查表常量 ==========
var gb46750UACategoryNames = []string{"微型(0)", "轻型(1)", "小型(2)", "中型(3)", "大型(4)"}
var gb46750RCSLocTypeNames = []string{"TakeOff", "Dynamic", "Fixed"}
var gb46750StatusNames = []string{"Undeclared", "Ground", "Airborne", "Emergency"}
var gb46750CoordSystemNames = []string{"WGS84", "CGCS2000"}
var gb46750AccuracyNames = []string{"Unknown", "<10m", "<3m", "<1m", "<0.3m"}
var gb46750TSAccuracyNames = []string{"Unknown", "<0.1s", "<0.2s", "<0.3s", "<0.4s", "<0.5s", "<1s", "<2s", "<3s", "<4s", "<5s"}
