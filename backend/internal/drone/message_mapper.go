package drone

import (
	"log/slog"
	"remoteid-monitor/internal/db"
	"remoteid-monitor/pkg/types"
	"strconv"
	"time"
)

// updateFloat 安全地解析并更新 float64 字段，消除重复代码
func updateFloat(valStr string, target *float64) {
	if valStr != "" {
		if f, err := strconv.ParseFloat(valStr, 64); err == nil {
			*target = f
		}
	}
}

// updateInt 安全地解析并更新 int 字段
func updateInt(valStr string, target *int) {
	if valStr != "" {
		if i, err := strconv.Atoi(valStr); err == nil {
			*target = i
		}
	}
}

// MapMessageToDrone 将解析出的 DroneMessage 映射到 DroneData 结构体
func MapMessageToDrone(drone *types.DroneData, msg types.DroneMessage, positions *map[string][]*types.Position) {
	switch msg.MessageType {
	case "basic_id":
		drone.UASID = msg.Data["uas_id"]
		drone.UAType = msg.Data["ua_type"]
		drone.IDType = msg.Data["id_type"]
		drone.Standard = msg.Standard
		drone.Source = msg.Source

	case "location":
		updateFloat(msg.Data["latitude"], &drone.Latitude)
		updateFloat(msg.Data["longitude"], &drone.Longitude)

		// 高度优先级：baro > geo > height_m
		if msg.Data["altitude_baro"] != "" {
			updateFloat(msg.Data["altitude_baro"], &drone.Altitude)
		} else if msg.Data["altitude_geo"] != "" {
			updateFloat(msg.Data["altitude_geo"], &drone.Altitude)
		} else if msg.Data["height_m"] != "" && drone.Altitude == 0 {
			updateFloat(msg.Data["height_m"], &drone.Altitude)
		}

		validateAltitude(drone, msg)

		updateFloat(msg.Data["speed_h"], &drone.Speed)
		updateFloat(msg.Data["speed_v"], &drone.SpeedVertical)
		updateFloat(msg.Data["direction"], &drone.Heading)

		drone.FlightStatus = msg.Data["status"]
		drone.HeightType = msg.Data["height_type"]
		drone.HAccuracy = msg.Data["h_accuracy"]
		drone.VAccuracy = msg.Data["v_accuracy"]
		drone.SAccuracy = msg.Data["s_accuracy"]
		drone.LocationTimestamp = msg.Data["timestamp"]

		savePosition(drone, positions)

	case "operator_id":
		if opID := msg.Data["operator_id"]; opID != "" && opID != " " {
			drone.OperatorID = opID
		}

	case "system":
		updateFloat(msg.Data["operator_lat"], &drone.OperatorLatitude)
		updateFloat(msg.Data["operator_lon"], &drone.OperatorLongitude)
		updateFloat(msg.Data["operator_alt"], &drone.OperatorAltitude)
		drone.Classification = msg.Data["classification"]
		updateInt(msg.Data["area_radius_m"], &drone.AreaRadiusM)

	case "gb46750":
		mapGB46750Message(drone, msg, positions)
	}
}

func mapGB46750Message(drone *types.DroneData, msg types.DroneMessage, positions *map[string][]*types.Position) {
	drone.Standard = msg.Standard
	drone.Source = msg.Source
	drone.IDType = "SerialNumber"

	if uasID := msg.Data["unique_id"]; uasID != "" && uasID != " " {
		drone.UASID = uasID
	}
	if realnameID := msg.Data["realname_id"]; realnameID != "" && realnameID != " " {
		drone.OperatorID = realnameID
	}
	if uaCat := msg.Data["ua_category"]; uaCat != "" {
		drone.UAType = uaCat
	}
	if opCat := msg.Data["operation_category"]; opCat != "" {
		drone.Classification = "GB46750-Cat" + opCat
	}

	updateFloat(msg.Data["latitude"], &drone.Latitude)
	updateFloat(msg.Data["longitude"], &drone.Longitude)

	if msg.Data["altitude_baro"] != "" {
		updateFloat(msg.Data["altitude_baro"], &drone.Altitude)
	} else if msg.Data["altitude_geo"] != "" {
		updateFloat(msg.Data["altitude_geo"], &drone.Altitude)
	} else {
		updateFloat(msg.Data["height_m"], &drone.Altitude)
	}
	validateAltitude(drone, msg)

	updateFloat(msg.Data["speed_h"], &drone.Speed)
	updateFloat(msg.Data["speed_v"], &drone.SpeedVertical)
	updateFloat(msg.Data["direction"], &drone.Heading)

	drone.FlightStatus = msg.Data["status"]
	drone.HeightType = msg.Data["height_type"]
	drone.HAccuracy = msg.Data["h_accuracy"]
	drone.VAccuracy = msg.Data["v_accuracy"]
	drone.SAccuracy = msg.Data["s_accuracy"]
	drone.LocationTimestamp = msg.Data["timestamp"]

	updateFloat(msg.Data["rcs_latitude"], &drone.OperatorLatitude)
	updateFloat(msg.Data["rcs_longitude"], &drone.OperatorLongitude)
	updateFloat(msg.Data["rcs_altitude"], &drone.OperatorAltitude)

	if drone.Latitude != 0 || drone.Longitude != 0 {
		savePosition(drone, positions)
	}
}

func validateAltitude(drone *types.DroneData, msg types.DroneMessage) {
	// 高度合理性检查：无人机限高通常不超过 500 米（模拟器信号不超过 120 米）
	// 如果高度超过 5000 米或低于 -500 米，很可能是解析错误，标记为无效
	// 在 updateDroneFromMessage 的 "location" case 中：
	if drone.Altitude > 5000 || drone.Altitude < -500 {
		slog.Warn("高度数据异常，可能是协议解析错位或固件错误",
			"mac", drone.MAC,
			"altitude", drone.Altitude,
			"standard", msg.Standard,
			"msg_type", msg.MessageType,
			"raw_hex", msg.RawHex) // 确保 DroneMessage 结构体中有 RawHex 字段
		drone.Altitude = 0
	}
}

func savePosition(drone *types.DroneData, positions *map[string][]*types.Position) {
	pos := &types.Position{
		Latitude:  drone.Latitude,
		Longitude: drone.Longitude,
		Altitude:  drone.Altitude,
		Speed:     drone.Speed,
		Heading:   drone.Heading,
		Timestamp: time.Now(),
	}
	(*positions)[drone.MAC] = append((*positions)[drone.MAC], pos)

	if err := db.SavePosition(drone.MAC, pos, drone.Standard); err != nil {
		slog.Warn("保存位置数据失败", "mac", drone.MAC, "error", err)
	}
}
