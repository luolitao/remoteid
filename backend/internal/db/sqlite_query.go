package db

import (
	"fmt"
	"time"
)

// PositionRecord 承载单点轨迹返回数据
type PositionRecord struct {
	Latitude  float64   `json:"latitude"`
	Longitude float64   `json:"longitude"`
	Altitude  float64   `json:"altitude"`
	Timestamp time.Time `json:"timestamp"`
}

// GetTrajectory 获取指定网卡或硬件特征在特定时间窗口内的完整移动轨迹
func GetTrajectory(mac string, limit int) ([]PositionRecord, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT latitude, longitude, altitude, timestamp 
		FROM positions 
		WHERE mac = ? 
		ORDER BY timestamp DESC 
		LIMIT ?
	`, mac, limit)
	if err != nil {
		return nil, fmt.Errorf("查询轨迹失败: %w", err)
	}
	defer rows.Close()

	var trajectory []PositionRecord
	for rows.Next() {
		var p PositionRecord
		var tStr string
		if err := rows.Scan(&p.Latitude, &p.Longitude, &p.Altitude, &tStr); err != nil {
			return nil, err
		}
		// 转换 SQLite 的时间文本
		p.Timestamp, _ = time.Parse("2006-01-02 15:04:05", tStr)
		trajectory = append(trajectory, p)
	}
	return trajectory, nil
}
