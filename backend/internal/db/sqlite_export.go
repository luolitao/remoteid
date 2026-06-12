package db

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
)

// InvokeExportCSV 导出最近 10000 条全量融合遥测航迹（提供给外围路由层调用）
func InvokeExportCSV() (string, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT p.timestamp, p.latitude, p.longitude, p.altitude,
		       d.uas_id, d.mac, d.standard
		FROM positions p
		JOIN drones d ON p.mac = d.mac
		ORDER BY p.timestamp DESC
		LIMIT 10000
	`)
	if err != nil {
		return "", fmt.Errorf("查询位置数据失败: %w", err)
	}
	defer rows.Close()

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	header := []string{"timestamp", "latitude", "longitude", "altitude", "uas_id", "mac", "standard"}
	if err := writer.Write(header); err != nil {
		return "", err
	}

	// 栈内存复用，避免 10000 次循环高频引发 GC
	record := make([]string, 7)

	for rows.Next() {
		var timestamp, uasID, mac, standard string
		var lat, lon, alt float64

		if err := rows.Scan(&timestamp, &lat, &lon, &alt, &uasID, &mac, &standard); err != nil {
			return "", err
		}

		record[0] = timestamp
		record[1] = strconv.FormatFloat(lat, 'f', 6, 64)
		record[2] = strconv.FormatFloat(lon, 'f', 6, 64)
		record[3] = strconv.FormatFloat(alt, 'f', 1, 64)
		record[4] = uasID
		record[5] = mac
		record[6] = standard

		if err := writer.Write(record); err != nil {
			return "", err
		}
	}

	writer.Flush()
	return buf.String(), writer.Error()
}
