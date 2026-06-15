package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"remoteid-monitor/pkg/types"
)

func CreateAlert(alertType, message, mac string) (int64, error) {
	result, err := db.ExecContext(ctx, `
		INSERT INTO alerts (alert_type, message, timestamp, mac) VALUES (?, ?, ?, ?)
	`, alertType, message, time.Now().UTC().Format(time.RFC3339), mac)
	if err != nil {
		return 0, fmt.Errorf("创建警报失败: %w", err)
	}
	return result.LastInsertId()
}

func GetAlertByID(id int) (*types.Alert, error) {
	alert := &types.Alert{}
	err := db.QueryRowContext(ctx, `
		SELECT id, alert_type, message, timestamp, mac, resolved FROM alerts WHERE id = ?
	`, id).Scan(&alert.ID, &alert.Type, &alert.Message, &alert.Timestamp, &alert.MAC, &alert.Resolved)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("警报未找到")
		}
		return nil, fmt.Errorf("查询警报失败: %w", err)
	}
	return alert, nil
}

func ResolveAlert(id int) error {
	result, err := db.ExecContext(ctx, `
		UPDATE alerts SET resolved = 1, resolved_at = ? WHERE id = ?
	`, time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		return fmt.Errorf("标记警报失败: %w", err)
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return fmt.Errorf("警报未找到")
	}
	return nil
}

func GetAlerts(limit, offset int) ([]types.Alert, int, error) {
	var total int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM alerts`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("获取警报总数失败: %w", err)
	}

	rows, err := db.QueryContext(ctx, `
		SELECT id, alert_type, message, timestamp, mac, resolved FROM alerts
		ORDER BY timestamp DESC LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("查询警报列表失败: %w", err)
	}
	defer rows.Close()

	var alerts []types.Alert
	for rows.Next() {
		var a types.Alert
		if err := rows.Scan(&a.ID, &a.Type, &a.Message, &a.Timestamp, &a.MAC, &a.Resolved); err != nil {
			return nil, 0, fmt.Errorf("扫描警报数据失败: %w", err)
		}
		alerts = append(alerts, a)
	}
	return alerts, total, nil
}

func GetAlertStatistics() (map[string]interface{}, error) {
	stats := make(map[string]interface{})
	var total, unresolved int
	db.QueryRowContext(ctx, `SELECT COUNT(*) FROM alerts`).Scan(&total)
	db.QueryRowContext(ctx, `SELECT COUNT(*) FROM alerts WHERE resolved = 0`).Scan(&unresolved)

	stats["total_alerts"] = total
	stats["unresolved_alerts"] = unresolved

	rows, err := db.QueryContext(ctx, `
		SELECT alert_type, COUNT(*) as count FROM alerts GROUP BY alert_type ORDER BY count DESC LIMIT 5
	`)
	if err == nil {
		defer rows.Close()
		typeCounts := make(map[string]int)
		for rows.Next() {
			var t string
			var c int
			rows.Scan(&t, &c)
			typeCounts[t] = c
		}
		stats["alert_types"] = typeCounts
	}

	if trend, err := getAlertTrend(24); err == nil {
		stats["trend_24h"] = trend
	}
	if recent, _, err := GetAlerts(5, 0); err == nil {
		stats["recent_alerts"] = recent
	}
	return stats, nil
}

func getAlertTrend(hours int) ([]map[string]interface{}, error) {
	since := time.Now().UTC().Add(-time.Duration(hours) * time.Hour).Format(time.RFC3339)
	rows, err := db.QueryContext(ctx, `
		SELECT strftime('%Y-%m-%d %H:00', timestamp) as hour, COUNT(*) as count
		FROM alerts WHERE timestamp > ? GROUP BY hour ORDER BY hour ASC
	`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trend []map[string]interface{}
	for rows.Next() {
		var hour string
		var count int
		if err := rows.Scan(&hour, &count); err != nil {
			return nil, err
		}
		trend = append(trend, map[string]interface{}{"hour": hour, "count": count})
	}
	return trend, nil
}

func ClearOldAlerts(hours int) (int64, error) {
	if hours < 1 {
		hours = 1
	}
	since := time.Now().UTC().Add(-time.Duration(hours) * time.Hour).Format(time.RFC3339)
	result, err := db.ExecContext(ctx, `DELETE FROM alerts WHERE timestamp < ? AND resolved = 1`, since)
	if err != nil {
		return 0, fmt.Errorf("清除旧警报失败: %w", err)
	}
	return result.RowsAffected()
}

func SearchAlerts(query string, limit int) ([]types.Alert, error) {
	if limit < 1 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	pattern := "%" + sanitizeSearchQuery(query) + "%"

	rows, err := db.QueryContext(ctx, `
		SELECT id, alert_type, message, timestamp, mac, resolved FROM alerts
		WHERE message LIKE ? OR alert_type LIKE ? OR mac LIKE ?
		ORDER BY timestamp DESC LIMIT ?
	`, pattern, pattern, pattern, limit)
	if err != nil {
		return nil, fmt.Errorf("搜索警报失败: %w", err)
	}
	defer rows.Close()

	var alerts []types.Alert
	for rows.Next() {
		var a types.Alert
		if err := rows.Scan(&a.ID, &a.Type, &a.Message, &a.Timestamp, &a.MAC, &a.Resolved); err != nil {
			return nil, fmt.Errorf("扫描搜索结果失败: %w", err)
		}
		alerts = append(alerts, a)
	}
	return alerts, nil
}

func sanitizeSearchQuery(query string) string {
	return strings.Map(func(r rune) rune {
		if r == '%' || r == '_' || r == '\'' {
			return -1
		}
		return r
	}, query)
}
