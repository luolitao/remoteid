package drone

import (
	"encoding/binary"
	"fmt"
	"math"
	"remoteid-monitor/pkg/types"
	"strconv"
	"strings"
)

type gb46750Rule struct {
	size  int
	parse func([]byte) (string, bool)
	key   string
}

// ========== GB 46750 严格对齐国标的查表常量 (已修复混用和缺失问题) ==========
var gb46750UACategoryNames = []string{"微型(0)", "轻型(1)", "小型(2)", "中型(3)", "大型(4)"}
var gb46750RCSLocTypeNames = []string{"起飞点(0)", "遥控站(1)"}
var gb46750StatusNames = []string{"未报告(0)", "地面(1)", "空中(2)", "紧急状态(3)", "ID失效-非紧急(4)", "ID失效-紧急(5)"}
var gb46750CoordSystemNames = []string{"WGS-84(0)", "CGCS2000(1)"}

// 精度字典必须分开，因为枚举数量完全不同，防止 Panic！
var gb46750HAccuracyNames = []string{"≥18.52km/未知", "<18.52km", "<7.41km", "<3.70km", "<1852m", "<926m", "<556m", "<185m", "<92.6m", "<30m", "<10m", "<3m", "<1m"}
var gb46750VAccuracyNames = []string{"≥150m/未知", "<150m", "<45m", "<25m", "<10m", "<3m", "<1m"}
var gb46750SAccuracyNames = []string{"≥10m/s/未知", "<10m/s", "<3m/s", "<1m/s", "<0.3m/s"}
var gb46750TSAccuracyNames = []string{">0.5s/未知", "≤0.5s", "≤0.4s", "≤0.3s", "≤0.2s", "≤0.1s", "≤50ms", "≤20ms", "≤10ms"}

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

func (p *RemoteIDParser) parseGB46750Payload(payload []byte) []types.DroneMessage {
	// 1. 严格的基础长度校验：DataType(1) + Version(1) + Length(1) + 至少1个Flag(1) = 4 字节
	if len(payload) < 4 {
		return nil
	}

	// 2. 校验 DataType 必须为 0xFF
	if payload[0] != 0xFF {
		return nil
	}

	// 3. 解析 Version (Bits 0-2: Base, Bits 3-7: Minor)
	versionByte := payload[1]
	major := (versionByte >> 5) & 0x07
	minor := versionByte & 0x1F

	// 4. 解析 Length
	contentLen := int(payload[2])

	// 5. 动态解析 Flags 区 (根据 Bit 0 扩展位判断结束)
	flagsStart := 3
	flagsEnd := flagsStart
	for flagsEnd < len(payload) {
		flagByte := payload[flagsEnd]
		flagsEnd++
		// Bit 0 == 0 表示这是最后一个标识字节
		if (flagByte & 0x01) == 0 {
			break
		}
	}

	flags := payload[flagsStart:flagsEnd]
	content := payload[flagsEnd:]

	// 6. 容错处理：如果包头声明的 contentLen 大于实际剩余数据长度，以实际为准，防止越界 Panic
	if len(content) < contentLen {
		contentLen = len(content)
	}

	data := make(map[string]any)
	data["gb46750_version"] = fmt.Sprintf("%d.%d", major, minor)

	// 7. 开始解析字段
	p.decodeGB46750Fields(flags, content[:contentLen], data)

	// ✅ 核心修复 1：自动拆分经纬度，转换为 float64，并清理中间字段
	if uavLocAny, ok := data["uav_location"]; ok {
		// 1. 安全类型断言：将 any 转为 string
		if uavLoc, isStr := uavLocAny.(string); isStr {
			if parts := strings.Split(uavLoc, "|"); len(parts) == 2 {
				// 2. 转换为 float64，直接喂给前端，免除前端二次转换
				if lon, err := strconv.ParseFloat(parts[0], 64); err == nil {
					data["longitude"] = lon
				}
				if lat, err := strconv.ParseFloat(parts[1], 64); err == nil {
					data["latitude"] = lat
				}
			}
		}
		// 3. 清理中间拼接字段，保持最终 JSON 数据的干净
		// slog.Info("坐标", "uav_location", data["uav_location"], "longitude", data["longitude"], "latitude", data["latitude"])
		delete(data, "uav_location")
	}

	if rcsLocAny, ok := data["rcs_location"]; ok {
		if rcsLoc, isStr := rcsLocAny.(string); isStr {
			if parts := strings.Split(rcsLoc, "|"); len(parts) == 2 {
				if lon, err := strconv.ParseFloat(parts[0], 64); err == nil {
					data["rcs_longitude"] = lon
				}
				if lat, err := strconv.ParseFloat(parts[1], 64); err == nil {
					data["rcs_latitude"] = lat
				}
			}
		}
		// slog.Info("坐标", "rcs_location", data["rcs_location"], "operator_longitude", data["operator_longitude"], "operator_latitude", data["operator_latitude"])
		delete(data, "rcs_location") // 清理中间字段
	}

	return []types.DroneMessage{{
		MessageType: "gb46750",
		Standard:    "GB 46750-2025",
		Data:        data,
		Source:      "Wifi Beacon",
	}}
}

func (p *RemoteIDParser) decodeGB46750Fields(flags []byte, content []byte, data map[string]any) {
	offset := 0
	for byteIdx, flag := range flags {
		if byteIdx >= 3 {
			break // 协议最多 3+N 字节，防止异常长包
		}

		// 解析 bit 7 到 bit 1 (对应数据项)
		for bit := 7; bit >= 1; bit-- {
			if (flag & (1 << bit)) == 0 {
				continue
			}

			// 防止 content 越界
			if offset >= len(content) {
				return
			}

			rule := p.getGB46750Rule(byteIdx, bit)
			if rule.size == 0 || offset+rule.size > len(content) {
				continue // 数据截断，跳过该项
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
	case 0: // 字节 1: Items 1-7
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
		case 2: // 006: 遥控站位置
			return gb46750Rule{8, func(b []byte) (string, bool) {
				return parseCoordAdaptive(b)
			}, "rcs_location"}
		case 1:
			return gb46750Rule{2, func(b []byte) (string, bool) {
				raw := binary.LittleEndian.Uint16(b)
				if raw == 0xFFFF {
					return " ", false
				}
				return fmt.Sprintf("%.2f", float64(raw)/2.0-1000.0), true
			}, "rcs_altitude"}
		}
	case 1: // 字节 2: Items 8-14
		switch bit {
		case 7: // 008: UA位置
			// ✅ 修复：统一使用自适应解析，返回 "lon|lat" 格式，与 rcs_location 保持一致
			return gb46750Rule{8, func(b []byte) (string, bool) {
				return parseCoordAdaptive(b)
			}, "uav_location"}
		case 6:
			return gb46750Rule{2, func(b []byte) (string, bool) {
				raw := binary.LittleEndian.Uint16(b)
				if raw == 0xFFFF {
					return " ", false
				}
				return fmt.Sprintf("%.2f", float64(raw)/10.0), true
			}, "direction"}
		case 5:
			return gb46750Rule{2, func(b []byte) (string, bool) {
				raw := binary.LittleEndian.Uint16(b)
				if raw == 0xFFFF {
					return " ", false
				}
				return fmt.Sprintf("%.2f", float64(raw)/10.0), true
			}, "speed_h"}
		case 4:
			return gb46750Rule{2, func(b []byte) (string, bool) {
				raw := binary.LittleEndian.Uint16(b)
				if raw == 0xFFFF {
					return " ", false
				}
				return fmt.Sprintf("%.2f", float64(raw)/2.0-9000.0), true
			}, "height_m"}
		case 3:
			return gb46750Rule{1, func(b []byte) (string, bool) {
				if b[0] == 0xFF {
					return " ", false
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
					return " ", false
				}
				return fmt.Sprintf("%.2f", float64(raw)/2.0-1000.0), true
			}, "altitude_geo"}
		case 1:
			return gb46750Rule{2, func(b []byte) (string, bool) {
				raw := binary.LittleEndian.Uint16(b)
				if raw == 0xFFFF {
					return " ", false
				}
				return fmt.Sprintf("%.2f", float64(raw)/2.0-1000.0), true
			}, "altitude_baro"}
		}
	case 2: // 字节 3: Items 15-21
		switch bit {
		case 7:
			return gb46750Rule{1, func(b []byte) (string, bool) { return lookupName(gb46750StatusNames, int(b[0])), true }, "status"}
		case 6:
			return gb46750Rule{1, func(b []byte) (string, bool) { return lookupName(gb46750CoordSystemNames, int(b[0])), true }, "coord_system"}
		case 5:
			return gb46750Rule{1, func(b []byte) (string, bool) { return lookupName(gb46750HAccuracyNames, int(b[0])), true }, "h_accuracy"}
		case 4:
			return gb46750Rule{1, func(b []byte) (string, bool) { return lookupName(gb46750VAccuracyNames, int(b[0])), true }, "v_accuracy"}
		case 3:
			return gb46750Rule{1, func(b []byte) (string, bool) { return lookupName(gb46750SAccuracyNames, int(b[0])), true }, "s_accuracy"}
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

// ✅ 核心修复 2：自适应经纬度解析器
// 彻底解决固件字节序不一致（先Lat后Lng 或 先Lng后Lat）导致越界被丢弃的问题
func parseCoordAdaptive(b []byte) (string, bool) {
	if len(b) < 8 {
		return " ", false
	}

	v1 := int32(binary.LittleEndian.Uint32(b[0:4]))
	v2 := int32(binary.LittleEndian.Uint32(b[4:8]))

	val1 := float64(v1) / 1e7
	val2 := float64(v2) / 1e7

	var lat, lon float64

	// 智能判断：绝对值 <= 90 的是纬度，<= 180 的是经度
	if math.Abs(val1) <= 90 && math.Abs(val2) <= 180 {
		lat, lon = val1, val2 // 固件顺序：Lat | Lng
	} else if math.Abs(val2) <= 90 && math.Abs(val1) <= 180 {
		lat, lon = val2, val1 // 固件顺序：Lng | Lat (符合国标字面)
	} else {
		return " ", false // 两个值都越界，判定为无效数据
	}

	// 统一返回 "lon|lat" 格式，供上层拆分
	return fmt.Sprintf("%.7f|%.7f", lon, lat), true
}
