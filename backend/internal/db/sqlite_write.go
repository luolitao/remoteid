package db

import (
	"fmt"
)

// SaveDrone 存储或更新空中出现的无人机静态档案
func SaveDrone(mac, uasID, standard string) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO drones (mac, uas_id, standard, first_seen)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(mac) DO UPDATE SET
			uas_id = CASE WHEN ? != '' THEN ? ELSE uas_id END,
			standard = ?
	`, mac, uasID, uasID, standard)

	if err != nil {
		return fmt.Errorf("持久化无人机档案失败: %w", err)
	}
	return nil
}

// SavePosition 存储一条通过了物理验证的合法遥测航迹点
func SavePosition(mac string, lat, lon, alt float64) error {
	// 物理边界看门狗：过滤掉经纬度全零或明显不合常理的仿真杂讯
	if !isValidLocation(lat, lon, alt) {
		return nil
	}

	_, err := db.ExecContext(ctx, `
		INSERT INTO positions (mac, latitude, longitude, altitude, timestamp)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, mac, lat, lon, alt)

	if err != nil {
		return fmt.Errorf("持久化动态遥测轨迹失败: %w", err)
	}
	return nil
}

// isValidLocation 地理信息合法性物理硬裁剪
func isValidLocation(lat, lon, alt float64) bool {
	if lat == 0.0 && lon == 0.0 {
		return false
	}
	if lat < -90.0 || lat > 90.0 || lon < -180.0 || lon > 180.0 {
		return false
	}
	// 粗略过滤掉超越民用飞行器升限的异常噪声数据（例如 50,000 米以上）
	if alt < -200.0 || alt > 50000.0 {
		return false
	}
	return true
}
