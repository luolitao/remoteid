package db

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func ExportData(format string) (string, error) {
	switch format {
	case "json":
		return exportJSON()
	case "csv":
		return exportCSV()
	default:
		return "", fmt.Errorf("不支持的导出格式: %s", format)
	}
}

func exportJSON() (string, error) {
	drones, err := GetActiveDrones()
	if err != nil {
		return "", err
	}
	alerts, _, err := GetAlerts(100, 0)
	if err != nil {
		return "", err
	}

	exportData := map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"drones":    drones,
		"alerts":    alerts,
		"system": map[string]interface{}{
			"total_drones":      len(drones),
			"unresolved_alerts": len(alerts),
		},
	}

	jsonData, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		return "", fmt.Errorf("JSON 编码失败: %w", err)
	}
	return string(jsonData), nil
}

func exportCSV() (string, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT p.timestamp, p.latitude, p.longitude, p.altitude, d.uas_id, d.mac, d.standard 
		FROM positions p JOIN drones d ON p.mac = d.mac 
		ORDER BY p.timestamp DESC LIMIT 10000
	`)
	if err != nil {
		return "", fmt.Errorf("查询位置数据失败: %w", err)
	}
	defer rows.Close()

	var csv strings.Builder
	csv.WriteString("timestamp,latitude,longitude,altitude,uas_id,mac,standard\n")

	for rows.Next() {
		var timestamp, uasID, mac, standard string
		var lat, lon, alt float64
		if err := rows.Scan(&timestamp, &lat, &lon, &alt, &uasID, &mac, &standard); err != nil {
			return "", fmt.Errorf("扫描位置数据失败: %w", err)
		}
		csv.WriteString(fmt.Sprintf("%s,%.6f,%.6f,%.1f,%s,%s,%s\n", timestamp, lat, lon, alt, uasID, mac, standard))
	}

	return csv.String(), nil
}
