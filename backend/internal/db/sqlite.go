package db

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"remoteid-monitor/pkg/types"

	_ "github.com/mattn/go-sqlite3"
)

// =================================================================
// 全局变量与常量配置
// =================================================================
var (
	db  *sql.DB
	ctx context.Context
)

const (
	dbMaxRetries    = 3
	dbRetryDelay    = 500 * time.Millisecond
	activeWindow    = -5 * time.Minute // 活跃窗口：最近5分钟
	maxConnLifetime = 30 * time.Minute
)

// =================================================================
// 初始化与关闭
// =================================================================

// Init 初始化数据库连接、配置高性能连接池、创建表结构与索引
func Init(dbPath string) error {
	// 1. 确保数据目录存在
	if err := createDataDir(dbPath); err != nil {
		return fmt.Errorf("创建数据目录失败: %w", err)
	}

	absolutePath := getDatabasePath(dbPath)
	// WAL 模式 + 5000ms 忙等待 + 缓存优化，彻底解决高并发写入锁冲突
	dsn := fmt.Sprintf("%s?_journal_mode=WAL&_cache_size=10000&_synchronous=NORMAL&_busy_timeout=5000", absolutePath)

	var err error
	db, err = sql.Open("sqlite3", dsn)
	if err != nil {
		return fmt.Errorf("打开数据库失败: %w", err)
	}

	// SQLite 单文件特性：严格限制单连接，配合 WAL 实现单写多读
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(maxConnLifetime)

	// 重试 Ping
	for i := 0; i < dbMaxRetries; i++ {
		if err := db.Ping(); err == nil {
			break
		}
		if i == dbMaxRetries-1 {
			db.Close()
			return fmt.Errorf("数据库连接测试失败: %w", err)
		}
		time.Sleep(dbRetryDelay)
	}

	// 初始化全局 Context
	ctx = context.Background()

	// 执行引导流程
	if err := createTables(); err != nil {
		db.Close()
		return fmt.Errorf("创建表结构失败: %w", err)
	}
	if err := migrateSchema(); err != nil {
		slog.Warn("数据库自动迁移存在警告", "error", err)
	}
	if err := createIndexes(); err != nil {
		return fmt.Errorf("创建索引失败: %w", err)
	}

	slog.Info("✅ SQLite 数据库初始化成功", "path", absolutePath)
	return nil
}

// Close 优雅关闭数据库连接
func Close() {
	if db != nil {
		db.Close()
		slog.Info("🔒 SQLite 数据库连接已安全关闭")
	}
}

// =================================================================
// 数据库结构引导与迁移
// =================================================================

func createTables() error {
	// 1. 独立表（无外键依赖）
	independent := []string{
		`CREATE TABLE IF NOT EXISTS drones (
			mac TEXT PRIMARY KEY,
			first_seen TEXT NOT NULL,
			last_seen TEXT NOT NULL,
			uas_id TEXT NOT NULL,
			operator_id TEXT,
			ua_type TEXT NOT NULL,
			id_type TEXT,
			latitude REAL, longitude REAL, altitude REAL, speed REAL, heading REAL,
			operator_latitude REAL, operator_longitude REAL, area_radius_m INTEGER,
			classification_region TEXT, standard TEXT NOT NULL,
			signal_strength TEXT, battery_level TEXT, flight_time TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS system_info (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			key TEXT NOT NULL UNIQUE, value TEXT NOT NULL, updated_at TEXT NOT NULL
		)`,
	}
	for _, sql := range independent {
		if _, err := db.ExecContext(ctx, sql); err != nil {
			return fmt.Errorf("创建独立表失败: %w", err)
		}
	}

	// 2. 依赖表（含外键）
	dependent := []string{
		`CREATE TABLE IF NOT EXISTS positions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			mac TEXT NOT NULL, timestamp TEXT NOT NULL,
			latitude REAL NOT NULL, longitude REAL NOT NULL, altitude REAL,
			speed REAL, heading REAL, standard TEXT NOT NULL,
			FOREIGN KEY(mac) REFERENCES drones(mac) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS alerts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			alert_type TEXT NOT NULL, message TEXT NOT NULL, timestamp TEXT NOT NULL,
			mac TEXT, resolved BOOLEAN DEFAULT 0, resolved_at TEXT,
			FOREIGN KEY(mac) REFERENCES drones(mac) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS trajectories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			mac TEXT NOT NULL, start_time TEXT NOT NULL, end_time TEXT NOT NULL,
			point_count INTEGER NOT NULL, max_altitude REAL, total_distance REAL,
			standard TEXT NOT NULL,
			FOREIGN KEY(mac) REFERENCES drones(mac) ON DELETE CASCADE
		)`,
	}
	for _, sql := range dependent {
		if _, err := db.ExecContext(ctx, sql); err != nil {
			time.Sleep(100 * time.Millisecond) // 外键约束延迟重试
			if _, err2 := db.ExecContext(ctx, sql); err2 != nil {
				return fmt.Errorf("创建依赖表失败: %w", err2)
			}
		}
	}
	return nil
}

func migrateSchema() error {
	type col struct{ table, name, spec string }
	migrations := []col{
		{"positions", "speed", "REAL"},
		{"positions", "heading", "REAL"},
		{"positions", "standard", "TEXT"},
		{"drones", "operator_id", "TEXT"},
		{"drones", "ua_type", "TEXT"},
		{"drones", "signal_strength", "TEXT"},
		{"drones", "battery_level", "TEXT"},
		{"drones", "flight_time", "TEXT"},
	}

	for _, m := range migrations {
		var count int
		q := fmt.Sprintf("SELECT COUNT(*) FROM pragma_table_info('%s') WHERE name = ?", m.table)
		if err := db.QueryRowContext(ctx, q, m.name).Scan(&count); err != nil {
			continue
		}
		if count > 0 {
			continue
		}
		alter := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", m.table, m.name, m.spec)
		if _, err := db.ExecContext(ctx, alter); err != nil {
			slog.Warn("迁移添加列失败", "table", m.table, "column", m.name, "error", err)
		} else {
			slog.Info("✅ 数据库迁移成功", "action", "add_column", "table", m.table, "column", m.name)
		}
	}
	return nil
}

func createIndexes() error {
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_positions_mac_timestamp ON positions(mac, timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_drones_last_seen ON drones(last_seen)`,
		`CREATE INDEX IF NOT EXISTS idx_alerts_timestamp ON alerts(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_alerts_resolved ON alerts(resolved)`,
	}
	for _, sql := range indexes {
		if _, err := db.ExecContext(ctx, sql); err != nil {
			return fmt.Errorf("创建索引失败: %w", err)
		}
	}
	slog.Info("✅ 数据库索引创建完成")
	return nil
}

// =================================================================
// 无人机核心操作
// =================================================================

// SaveDrone 保存或更新无人机静态档案
func SaveDrone(drone *types.DroneData) error {
	firstSeen := drone.FirstSeen.UTC().Format(time.RFC3339)
	lastSeen := drone.LastSeen.UTC().Format(time.RFC3339)

	_, err := db.ExecContext(ctx, `
		INSERT INTO drones (
			mac, first_seen, last_seen, uas_id, operator_id, ua_type, id_type,
			latitude, longitude, altitude, speed, heading,
			operator_latitude, operator_longitude, area_radius_m,
			classification_region, standard,
			signal_strength, battery_level, flight_time
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(mac) DO UPDATE SET
			last_seen = excluded.last_seen,
			uas_id = excluded.uas_id,
			operator_id = COALESCE(excluded.operator_id, drones.operator_id),
			ua_type = excluded.ua_type,
			id_type = COALESCE(excluded.id_type, drones.id_type),
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
	`,
		drone.MAC, firstSeen, lastSeen, drone.UASID, nullString(drone.OperatorID), drone.UAType, nullString(drone.IDType),
		nullFloat64(drone.Latitude), nullFloat64(drone.Longitude), nullFloat64(drone.Altitude),
		nullFloat64(drone.Speed), nullFloat64(drone.Heading),
		nullFloat64(drone.OperatorLatitude), nullFloat64(drone.OperatorLongitude), nullInt(drone.AreaRadiusM),
		nullString(drone.Classification), drone.Standard,
		nullString(drone.SignalStrength), nullString(drone.BatteryLevel), nullString(drone.FlightTime),
	)
	if err != nil {
		return fmt.Errorf("保存无人机数据失败: %w", err)
	}
	return nil
}

// GetActiveDrones 获取最近活跃的无人机列表
func GetActiveDrones() ([]*types.DroneData, error) {
	since := time.Now().UTC().Add(activeWindow).Format(time.RFC3339)
	rows, err := db.QueryContext(ctx, `
		SELECT mac, uas_id, operator_id, ua_type, id_type, latitude, longitude, altitude,
		       speed, heading, operator_latitude, operator_longitude,
		       classification_region, standard, signal_strength, battery_level, flight_time,
		       first_seen, last_seen
		FROM drones WHERE last_seen > ? ORDER BY last_seen DESC
	`, since)
	if err != nil {
		return nil, fmt.Errorf("查询活跃无人机失败: %w", err)
	}
	defer rows.Close()

	var drones []*types.DroneData
	for rows.Next() {
		d := &types.DroneData{}
		var fs, ls string
		var lat, lon, alt, spd, hdg sql.NullFloat64
		var opLat, opLon sql.NullFloat64
		var opID, idType, cls, sig, bat, flt sql.NullString

		if err := rows.Scan(
			&d.MAC, &d.UASID, &opID, &d.UAType, &idType, &lat, &lon, &alt, &spd, &hdg,
			&opLat, &opLon, &cls, &d.Standard, &sig, &bat, &flt, &fs, &ls,
		); err != nil {
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
		if spd.Valid {
			d.Speed = spd.Float64
		}
		if hdg.Valid {
			d.Heading = hdg.Float64
		}
		if opLat.Valid {
			d.OperatorLatitude = opLat.Float64
		}
		if opLon.Valid {
			d.OperatorLongitude = opLon.Float64
		}
		if cls.Valid {
			d.Classification = cls.String
		}
		if sig.Valid {
			d.SignalStrength = sig.String
		}
		if bat.Valid {
			d.BatteryLevel = bat.String
		}
		if flt.Valid {
			d.FlightTime = flt.String
		}

		if t, err := time.Parse(time.RFC3339, fs); err == nil {
			d.FirstSeen = t
		} else {
			d.FirstSeen = time.Now().UTC()
		}
		if t, err := time.Parse(time.RFC3339, ls); err == nil {
			d.LastSeen = t
		} else {
			d.LastSeen = time.Now().UTC()
		}

		drones = append(drones, d)
	}
	return drones, nil
}

// =================================================================
// 轨迹与位置操作
// =================================================================

// SavePosition 保存无人机位置记录（统一使用 types.Position 结构体）
func SavePosition(mac string, pos *types.Position, standard string) error {
	if pos == nil || !isValidLocation(pos.Latitude, pos.Longitude, pos.Altitude) {
		return nil
	}
	timestamp := pos.Timestamp.UTC().Format(time.RFC3339)

	_, err := db.ExecContext(ctx, `
		INSERT INTO positions (mac, timestamp, latitude, longitude, altitude, speed, heading, standard)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, mac, timestamp, pos.Latitude, pos.Longitude, pos.Altitude,
		nullFloat64(pos.Speed), nullFloat64(pos.Heading), standard)
	if err != nil {
		return fmt.Errorf("保存位置数据失败: %w", err)
	}
	return nil
}

// GetTrajectory 获取指定无人机的历史轨迹（按时间正序）
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
		FROM positions WHERE mac = ? AND timestamp > ?
		ORDER BY timestamp ASC
	`, mac, since)
	if err != nil {
		return nil, fmt.Errorf("查询轨迹失败: %w", err)
	}
	defer rows.Close()

	var positions []types.Position
	for rows.Next() {
		var p types.Position
		var tsStr string
		var alt, spd, hdg sql.NullFloat64
		if err := rows.Scan(&tsStr, &p.Latitude, &p.Longitude, &alt, &spd, &hdg); err != nil {
			return nil, fmt.Errorf("扫描轨迹数据失败: %w", err)
		}
		if t, err := time.Parse(time.RFC3339, tsStr); err == nil {
			p.Timestamp = t
		}
		if alt.Valid {
			p.Altitude = alt.Float64
		}
		if spd.Valid {
			p.Speed = spd.Float64
		}
		if hdg.Valid {
			p.Heading = hdg.Float64
		}
		positions = append(positions, p)
	}
	return positions, nil
}

// =================================================================
// 告警系统操作
// =================================================================

func CreateAlert(alertType, message, mac string) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := db.ExecContext(ctx, `
		INSERT INTO alerts (alert_type, message, timestamp, mac) VALUES (?, ?, ?, ?)
	`, alertType, message, now, mac)
	if err != nil {
		return 0, fmt.Errorf("创建警报失败: %w", err)
	}
	return res.LastInsertId()
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
		a := types.Alert{}
		var tsStr string
		if err := rows.Scan(&a.ID, &a.Type, &a.Message, &tsStr, &a.MAC, &a.Resolved); err != nil {
			return nil, 0, fmt.Errorf("扫描警报数据失败: %w", err)
		}
		if t, err := time.Parse(time.RFC3339, tsStr); err == nil {
			a.Timestamp = t
		}
		alerts = append(alerts, a)
	}
	return alerts, total, nil
}

func ResolveAlert(id string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := db.ExecContext(ctx, `UPDATE alerts SET resolved = 1, resolved_at = ? WHERE id = ?`, now, id)
	if err != nil {
		return fmt.Errorf("标记警报失败: %w", err)
	}
	if affected, _ := res.RowsAffected(); affected == 0 {
		return fmt.Errorf("警报未找到或已解决")
	}
	return nil
}

func GetAlertStatistics() (map[string]interface{}, error) {
	stats := make(map[string]interface{})
	var total, unresolved int
	db.QueryRowContext(ctx, `SELECT COUNT(*) FROM alerts`).Scan(&total)
	db.QueryRowContext(ctx, `SELECT COUNT(*) FROM alerts WHERE resolved = 0`).Scan(&unresolved)
	stats["total_alerts"] = total
	stats["unresolved_alerts"] = unresolved

	rows, err := db.QueryContext(ctx, `SELECT alert_type, COUNT(*) as count FROM alerts GROUP BY alert_type ORDER BY count DESC LIMIT 5`)
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
	return stats, nil
}

func SearchAlerts(query string, limit int) ([]types.Alert, error) {
	if limit < 1 || limit > 100 {
		limit = 10
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
		a := types.Alert{}
		var tsStr string
		rows.Scan(&a.ID, &a.Type, &a.Message, &tsStr, &a.MAC, &a.Resolved)
		if t, err := time.Parse(time.RFC3339, tsStr); err == nil {
			a.Timestamp = t
		}
		alerts = append(alerts, a)
	}
	return alerts, nil
}

// =================================================================
// 系统统计与导出
// =================================================================

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
	drones, _ := GetActiveDrones()
	alerts, _, _ := GetAlerts(100, 0)
	data := map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"drones":    drones, "alerts": alerts,
		"system": map[string]interface{}{"total_drones": len(drones), "unresolved_alerts": len(alerts)},
	}
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("JSON 编码失败: %w", err)
	}
	return string(jsonBytes), nil
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

	var buf strings.Builder
	w := csv.NewWriter(&buf)
	w.Write([]string{"timestamp", "latitude", "longitude", "altitude", "uas_id", "mac", "standard"})

	rec := make([]string, 7)
	for rows.Next() {
		var ts, uas, mac, std string
		var lat, lon, alt float64
		rows.Scan(&ts, &lat, &lon, &alt, &uas, &mac, &std)
		rec[0], rec[1], rec[2], rec[3] = ts, fmt.Sprintf("%.6f", lat), fmt.Sprintf("%.6f", lon), fmt.Sprintf("%.1f", alt)
		rec[4], rec[5], rec[6] = uas, mac, std
		w.Write(rec)
	}
	w.Flush()
	return buf.String(), w.Error()
}

// =================================================================
// 辅助工具函数
// =================================================================

func isValidLocation(lat, lon, alt float64) bool {
	if lat == 0 && lon == 0 {
		return false
	}
	if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return false
	}
	if alt < -200 || alt > 50000 {
		return false
	}
	return true
}

func nullFloat64(v float64) interface{} { return v }
func nullInt(v int) interface{}         { return v }
func nullString(v string) interface{} {
	if v == "" {
		return nil
	}
	return v
}

func sanitizeSearchQuery(query string) string {
	return strings.Map(func(r rune) rune {
		if r == '%' || r == '_' || r == '\'' {
			return -1
		}
		return r
	}, query)
}

func createDataDir(dbPath string) error {
	dir := filepath.Dir(dbPath)
	if dir == "" || dir == "." {
		return nil
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return os.MkdirAll(dir, 0755)
	}
	return nil
}

func getDatabasePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "/data"
	}
	return filepath.Join(cwd, path)
}
