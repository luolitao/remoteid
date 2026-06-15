package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"remoteid-monitor/pkg/types"
)

func SaveDrone(drone *types.DroneData) error {
	firstSeen := drone.FirstSeen.UTC().Format(time.RFC3339)
	lastSeen := drone.LastSeen.UTC().Format(time.RFC3339)

	_, err := db.ExecContext(ctx, `
		INSERT INTO drones (mac, first_seen, last_seen, uas_id, operator_id, ua_type, id_type,
			latitude, longitude, altitude, speed, heading, operator_latitude, operator_longitude,
			area_radius_m, classification_region, standard, signal_strength, battery_level, flight_time)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(mac) DO UPDATE SET
			last_seen = excluded.last_seen, uas_id = excluded.uas_id,
			operator_id = COALESCE(excluded.operator_id, drones.operator_id),
			ua_type = excluded.ua_type, id_type = COALESCE(excluded.id_type, drones.id_type),
			latitude = COALESCE(excluded.latitude, drones.latitude),
			longitude = COALESCE(excluded.longitude, drones.longitude),
			altitude = COALESCE(excluded.altitude, drones.altitude),
			speed = COALESCE(excluded.speed, drones.speed),
			heading = COALESCE(excluded.heading, drones.heading),
			operator_latitude = COALESCE(excluded.operator_latitude, drones.operator_latitude),
			operator_longitude = COALESCE(excluded.operator_longitude, drones.operator_longitude),
			area_radius_m = COALESCE(excluded.area_radius_m, drones.area_radius_m),
			classification_region = COALESCE(excluded.classification_region, drones.classification_region),
			standard = excluded.standard,
			signal_strength = COALESCE(excluded.signal_strength, drones.signal_strength),
			battery_level = COALESCE(excluded.battery_level, drones.battery_level),
			flight_time = COALESCE(excluded.flight_time, drones.flight_time)
	`, drone.MAC, firstSeen, lastSeen, drone.UASID, nullString(drone.OperatorID), drone.UAType, nullString(drone.IDType),
		nullFloat64(drone.Latitude), nullFloat64(drone.Longitude), nullFloat64(drone.Altitude),
		nullFloat64(drone.Speed), nullFloat64(drone.Heading), nullFloat64(drone.OperatorLatitude),
		nullFloat64(drone.OperatorLongitude), nullInt(drone.AreaRadiusM), nullString(drone.Classification),
		drone.Standard, nullString(drone.SignalStrength), nullString(drone.BatteryLevel), nullString(drone.FlightTime))

	if err != nil {
		return fmt.Errorf("保存无人机数据失败: %w", err)
	}
	return nil
}

func SavePosition(mac string, pos *types.Position, standard string) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO positions (mac, timestamp, latitude, longitude, altitude, speed, heading, standard)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, mac, pos.Timestamp.UTC().Format(time.RFC3339), pos.Latitude, pos.Longitude, pos.Altitude,
		nullFloat64(pos.Speed), nullFloat64(pos.Heading), standard)

	if err != nil {
		return fmt.Errorf("保存位置数据失败: %w", err)
	}
	return nil
}

func GetActiveDrones() ([]*types.DroneData, error) {
	since := time.Now().UTC().Add(activeWindow).Format(time.RFC3339)
	rows, err := db.QueryContext(ctx, `
		SELECT mac, uas_id, operator_id, ua_type, id_type, latitude, longitude, altitude,
		       speed, heading, operator_latitude, operator_longitude, classification_region, standard,
		       signal_strength, battery_level, flight_time, first_seen, last_seen
		FROM drones WHERE last_seen > ? ORDER BY last_seen DESC
	`, since)
	if err != nil {
		return nil, fmt.Errorf("查询活跃无人机失败: %w", err)
	}
	defer rows.Close()

	var drones []*types.DroneData
	for rows.Next() {
		d := &types.DroneData{}
		var firstSeenStr, lastSeenStr string
		var lat, lon, alt, speed, heading, opLat, opLon sql.NullFloat64
		var opID, idType, classification, signalStr, battery, flight sql.NullString

		if err := rows.Scan(&d.MAC, &d.UASID, &opID, &d.UAType, &idType, &lat, &lon, &alt, &speed, &heading,
			&opLat, &opLon, &classification, &d.Standard, &signalStr, &battery, &flight, &firstSeenStr, &lastSeenStr); err != nil {
			return nil, fmt.Errorf("扫描无人机数据失败: %w", err)
		}

		if opID.Valid {
			d.OperatorID = opID.String
		}
		if idType.Valid {
			d.IDType = idType.String
		}
		if lat.Valid {
			d.Latitude = lat.Float64
		}
		if lon.Valid {
			d.Longitude = lon.Float64
		}
		if alt.Valid {
			d.Altitude = alt.Float64
		}
		if speed.Valid {
			d.Speed = speed.Float64
		}
		if heading.Valid {
			d.Heading = heading.Float64
		}
		if opLat.Valid {
			d.OperatorLatitude = opLat.Float64
		}
		if opLon.Valid {
			d.OperatorLongitude = opLon.Float64
		}
		if classification.Valid {
			d.Classification = classification.String
		}
		if signalStr.Valid {
			d.SignalStrength = signalStr.String
		}
		if battery.Valid {
			d.BatteryLevel = battery.String
		}
		if flight.Valid {
			d.FlightTime = flight.String
		}

		if t, err := time.Parse(time.RFC3339, firstSeenStr); err == nil {
			d.FirstSeen = t
		} else {
			d.FirstSeen = time.Now().UTC()
		}
		if t, err := time.Parse(time.RFC3339, lastSeenStr); err == nil {
			d.LastSeen = t
		} else {
			d.LastSeen = time.Now().UTC()
		}

		drones = append(drones, d)
	}
	return drones, nil
}

func GetTrajectory(mac string, hours int) ([]types.Position, error) {
	if hours < 1 {
		hours = 1
	}
	if hours > 720 {
		hours = 720
	}
	since := time.Now().UTC().Add(-time.Duration(hours) * time.Hour).Format(time.RFC3339)

	rows, err := db.QueryContext(ctx, `
		SELECT timestamp, latitude, longitude, altitude, speed, heading
		FROM positions WHERE mac = ? AND timestamp > ? ORDER BY timestamp ASC
	`, mac, since)
	if err != nil {
		return nil, fmt.Errorf("查询轨迹失败: %w", err)
	}
	defer rows.Close()

	var positions []types.Position
	for rows.Next() {
		var pos types.Position
		if err := rows.Scan(&pos.Timestamp, &pos.Latitude, &pos.Longitude, &pos.Altitude, &pos.Speed, &pos.Heading); err != nil {
			return nil, fmt.Errorf("扫描轨迹数据失败: %w", err)
		}
		positions = append(positions, pos)
	}
	return positions, nil
}

func GetActiveDroneCount() (int, error) {
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM drones WHERE last_seen > ?`, time.Now().UTC().Add(activeWindow).Format(time.RFC3339)).Scan(&count)
	return count, err
}

func GetTotalDroneCount() (int, error) {
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM drones`).Scan(&count)
	return count, err
}

// getManufacturerFromUAType 辅助函数，可根据需要保留或移除
func getManufacturerFromUAType(uaType string) string {
	lower := strings.ToLower(uaType)
	if strings.Contains(lower, "dji") {
		return "DJI"
	}
	if strings.Contains(lower, "autel") {
		return "Autel Robotics"
	}
	if strings.Contains(lower, "parrot") {
		return "Parrot"
	}
	return "Unknown"
}
