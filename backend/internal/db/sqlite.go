// internal/db/sqlite.go
package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"remoteid-monitor/internal/config"
	"remoteid-monitor/pkg/types"

	_ "github.com/mattn/go-sqlite3"
)

const (
	dbMaxRetries      = 3
	dbRetryDelay      = 500 * time.Millisecond
	activeWindow      = -5 * time.Minute
	maxConnLifetime   = 30 * time.Minute
)

var (
	db  *sql.DB
	ctx context.Context // +++ 确保全局上下文初始化 +++
)

// Init 初始化数据库连接
func Init() error {
	cfg := config.Get()

	// 1. 创建数据目录
	if err := createDataDir(cfg.Database.Path); err != nil {
		return fmt.Errorf("创建数据目录失败: %w", err)
	}

	// 2. 获取数据库路径
	dbPath := getDatabasePath(cfg.Database.Path)

	// 3. 打开数据库连接 - 修正：使用正确的连接参数
	var err error
	db, err = sql.Open("sqlite3", fmt.Sprintf("%s?_journal_mode=WAL&_cache_size=%d&_synchronous=NORMAL&_timeout=5000",
		dbPath, cfg.Database.CacheSize))
	if err != nil {
		return fmt.Errorf("打开数据库失败: %w", err)
	}

	// 4. 测试连接 - 修正：添加重试逻辑
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

	// 5. 设置连接池
	db.SetMaxOpenConns(cfg.Database.MaxConnections)
	db.SetMaxIdleConns(cfg.Database.MaxConnections / 2)
	db.SetConnMaxLifetime(maxConnLifetime)

	// 6. 创建上下文 - +++ 关键修复：初始化全局上下文 +++
	ctx = context.Background()

	// 7. 创建表结构
	if err := createTables(); err != nil {
		db.Close()
		return fmt.Errorf("创建表结构失败: %w", err)
	}

	// 8. 创建索引
	if err := createIndexes(); err != nil {
		db.Close()
		return fmt.Errorf("创建索引失败: %w", err)
	}

	slog.Info("SQLite 数据库初始化成功", "path", dbPath)
	return nil
}

// createTables 创建所有表 - 修正：添加上下文检查
func createTables() error {
	if db == nil {
		return fmt.Errorf("数据库连接未初始化")
	}
	if ctx == nil {
		ctx = context.Background() // +++ 确保上下文不为 nil +++
	}

	// 1. 先创建独立表（无外键依赖）
	tables := []string{
		`
		CREATE TABLE IF NOT EXISTS drones (
			mac TEXT PRIMARY KEY,
			first_seen TEXT NOT NULL,
			last_seen TEXT NOT NULL,
			uas_id TEXT NOT NULL,
			ua_type TEXT NOT NULL,
			id_type TEXT,
			latitude REAL,
			longitude REAL,
			altitude REAL,
			speed REAL,
			heading REAL,
			operator_latitude REAL,
			operator_longitude REAL,
			area_radius_m INTEGER,
			classification_region TEXT,
			china_compliant BOOLEAN DEFAULT 0,
			standard TEXT NOT NULL,
			signal_strength TEXT,
			battery_level TEXT,
			flight_time TEXT
		)
		`,

		`
		CREATE TABLE IF NOT EXISTS system_info (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			key TEXT NOT NULL UNIQUE,
			value TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)
		`,
	}

	for _, tableSQL := range tables {
		if _, err := db.ExecContext(ctx, tableSQL); err != nil {
			return fmt.Errorf("创建表失败: %w", err)
		}
	}

	// 2. 再创建依赖表
	dependentTables := []string{
		`
		CREATE TABLE IF NOT EXISTS positions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			mac TEXT NOT NULL,
			timestamp TEXT NOT NULL,
			latitude REAL NOT NULL,
			longitude REAL NOT NULL,
			altitude REAL,
			speed REAL,
			heading REAL,
			standard TEXT NOT NULL,
			FOREIGN KEY(mac) REFERENCES drones(mac) ON DELETE CASCADE
		)
		`,

		`
		CREATE TABLE IF NOT EXISTS alerts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			alert_type TEXT NOT NULL,
			message TEXT NOT NULL,
			timestamp TEXT NOT NULL,
			mac TEXT,
			resolved BOOLEAN DEFAULT 0,
			resolved_at TEXT,
			FOREIGN KEY(mac) REFERENCES drones(mac) ON DELETE CASCADE
		)
		`,

		`
		CREATE TABLE IF NOT EXISTS trajectories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			mac TEXT NOT NULL,
			start_time TEXT NOT NULL,
			end_time TEXT NOT NULL,
			point_count INTEGER NOT NULL,
			max_altitude REAL,
			total_distance REAL,
			standard TEXT NOT NULL,
			FOREIGN KEY(mac) REFERENCES drones(mac) ON DELETE CASCADE
		)
		`,
	}

	for _, tableSQL := range dependentTables {
		if _, err := db.ExecContext(ctx, tableSQL); err != nil {
			// 重试（可能由于外键约束）
			time.Sleep(100 * time.Millisecond)
			if _, err2 := db.ExecContext(ctx, tableSQL); err2 != nil {
				return fmt.Errorf("创建依赖表失败: %w", err2)
			}
		}
	}

	slog.Info("数据库表结构创建成功")

	// 自动迁移：为旧数据库添加缺失的列
	if err := migrateSchema(); err != nil {
		return fmt.Errorf("数据库迁移失败: %w", err)
	}

	return nil
}

// migrateSchema 自动迁移：为旧版本数据库添加缺失的列
func migrateSchema() error {
	type columnAdd struct {
		table  string
		column string
		spec   string
	}

	migrations := []columnAdd{
		{"drones", "id_type", "TEXT"},
		{"drones", "operator_latitude", "REAL"},
		{"drones", "operator_longitude", "REAL"},
		{"drones", "area_radius_m", "INTEGER"},
		{"drones", "classification_region", "TEXT"},
		{"drones", "signal_strength", "TEXT"},
		{"drones", "battery_level", "TEXT"},
		{"drones", "flight_time", "TEXT"},
	}

	for _, m := range migrations {
		// 检查列是否已存在
		query := fmt.Sprintf("SELECT COUNT(*) FROM pragma_table_info('%s') WHERE name = ?", m.table)
		var count int
		err := db.QueryRowContext(ctx, query, m.column).Scan(&count)
		if err != nil {
			slog.Warn("检查列存在性失败", "table", m.table, "column", m.column, "error", err)
			continue
		}
		if count > 0 {
			continue // 列已存在，跳过
		}

		// 添加缺失的列
		alterSQL := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", m.table, m.column, m.spec)
		if _, err := db.ExecContext(ctx, alterSQL); err != nil {
			slog.Warn("添加列失败", "table", m.table, "column", m.column, "error", err)
			continue
		}
		slog.Info("数据库迁移：添加列", "table", m.table, "column", m.column)
	}

	return nil
}

// Close 关闭数据库连接
func Close() {
	if db != nil {
		db.Close()
		slog.Info("SQLite 数据库连接已关闭")
	}
}

// createDataDir 创建数据目录
func createDataDir(dbPath string) error {
	dir := filepath.Dir(dbPath)
	if dir == "" || dir == "." {
		return nil
	}

	// 检查目录是否存在
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		// 创建目录（包括父目录）
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建目录失败: %w", err)
		}
		slog.Info("创建数据目录", "dir", dir)
	}
	return nil
}

// getDatabasePath 获取数据库绝对路径
func getDatabasePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}

	// 获取当前工作目录
	cwd, err := os.Getwd()
	if err != nil {
		slog.Warn("获取当前工作目录失败", "error", err)
		cwd = "/data"
	}

	return filepath.Join(cwd, path)
}

// createIndexes 创建索引优化查询性能
func createIndexes() error {
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_drones_last_seen ON drones(last_seen)`,
		`CREATE INDEX IF NOT EXISTS idx_positions_mac_timestamp ON positions(mac, timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_alerts_timestamp ON alerts(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_alerts_resolved ON alerts(resolved)`,
		`CREATE INDEX IF NOT EXISTS idx_positions_timestamp ON positions(timestamp)`,
	}

	for _, indexSQL := range indexes {
		if _, err := db.ExecContext(ctx, indexSQL); err != nil {
			return fmt.Errorf("创建索引失败: %w", err)
		}
	}

	slog.Info("数据库索引创建成功")
	return nil
}

// getFirstSeen 获取无人机首次出现时间
func getFirstSeen(tx *sql.Tx, mac, now string) string {
	var firstSeen string
	err := tx.QueryRowContext(ctx, `
		SELECT first_seen FROM drones WHERE mac = ?
	`, mac).Scan(&firstSeen)

	if err == sql.ErrNoRows {
		return now // 新无人机，使用当前时间
	}

	if err != nil {
		slog.Warn("查询首次出现时间失败", "error", err)
		return now
	}

	return firstSeen
}

// isValidLocation 检查位置是否有效
func isValidLocation(lat, lon float64) bool {
	// 检查是否为有效坐标（非零，且在合理范围内）
	if lat == 0 && lon == 0 {
		return false
	}

	// 检查纬度范围 (-90 到 90)
	if lat < -90 || lat > 90 {
		return false
	}

	// 检查经度范围 (-180 到 180)
	if lon < -180 || lon > 180 {
		return false
	}

	return true
}

// boolToSQLiteBool 转换布尔值为 SQLite 布尔值
func boolToSQLiteBool(b bool) int {
	if b {
		return 1
	}
	return 0
}

// getManufacturerFromUAType 从无人机类型推断制造商
func getManufacturerFromUAType(uaType string) string {
	uaTypeLower := strings.ToLower(uaType)

	switch {
	case strings.Contains(uaTypeLower, "dji"):
		return "DJI"
	case strings.Contains(uaTypeLower, "autel"):
		return "Autel Robotics"
	case strings.Contains(uaTypeLower, "parrot"):
		return "Parrot"
	case strings.Contains(uaTypeLower, "yuneec"):
		return "Yuneec"
	case strings.Contains(uaTypeLower, "skydio"):
		return "Skydio"
	default:
		return "Unknown"
	}
}

// GetTrajectory 获取无人机轨迹
func GetTrajectory(mac string, hours int) ([]types.Position, error) {
	if hours < 1 {
		hours = 1
	}
	if hours > 720 { // 30天
		hours = 720
	}

	since := time.Now().UTC().Add(-time.Duration(hours) * time.Hour).Format(time.RFC3339)

	rows, err := db.QueryContext(ctx, `
		SELECT timestamp, latitude, longitude, altitude, speed, heading
		FROM positions
		WHERE mac = ? AND timestamp > ?
		ORDER BY timestamp ASC
	`, mac, since)
	if err != nil {
		return nil, fmt.Errorf("查询轨迹失败: %w", err)
	}
	defer rows.Close()

	var positions []types.Position
	for rows.Next() {
		pos := types.Position{}
		if err := rows.Scan(&pos.Timestamp, &pos.Latitude, &pos.Longitude, &pos.Altitude, &pos.Speed, &pos.Heading); err != nil {
			return nil, fmt.Errorf("扫描轨迹数据失败: %w", err)
		}
		positions = append(positions, pos)
	}

	return positions, nil
}

// GetActiveDrones 获取活跃无人机
func GetActiveDrones() ([]*types.DroneData, error) {
	since := time.Now().UTC().Add(activeWindow).Format(time.RFC3339)

	rows, err := db.QueryContext(ctx, `
		SELECT mac, uas_id, ua_type, id_type, latitude, longitude, altitude,
		       speed, heading, operator_latitude, operator_longitude,
		       classification_region, china_compliant, standard,
		       signal_strength, battery_level, flight_time,
		       first_seen, last_seen
		FROM drones
		WHERE last_seen > ?
		ORDER BY last_seen DESC
	`, since)
	if err != nil {
		return nil, fmt.Errorf("查询活跃无人机失败: %w", err)
	}
	defer rows.Close()

	var drones []*types.DroneData
	for rows.Next() {
		drone := &types.DroneData{}
		var firstSeenStr, lastSeenStr string
		var lat, lon, alt, speed, heading sql.NullFloat64
		var opLat, opLon sql.NullFloat64
		var idType, classification, signalStr, battery, flight sql.NullString

		if err := rows.Scan(
			&drone.MAC, &drone.UASID, &drone.UAType, &idType,
			&lat, &lon, &alt, &speed, &heading,
			&opLat, &opLon, &classification, &drone.ChinaCompliant, &drone.Standard,
			&signalStr, &battery, &flight,
			&firstSeenStr, &lastSeenStr,
		); err != nil {
			return nil, fmt.Errorf("扫描无人机数据失败: %w", err)
		}

		if idType.Valid {
			drone.IDType = idType.String
		}
		if lat.Valid {
			drone.Latitude = lat.Float64
		}
		if lon.Valid {
			drone.Longitude = lon.Float64
		}
		if alt.Valid {
			drone.Altitude = alt.Float64
		}
		if speed.Valid {
			drone.Speed = speed.Float64
		}
		if heading.Valid {
			drone.Heading = heading.Float64
		}
		if opLat.Valid {
			drone.OperatorLatitude = opLat.Float64
		}
		if opLon.Valid {
			drone.OperatorLongitude = opLon.Float64
		}
		if classification.Valid {
			drone.Classification = classification.String
		}
		if signalStr.Valid {
			drone.SignalStrength = signalStr.String
		}
		if battery.Valid {
			drone.BatteryLevel = battery.String
		}
		if flight.Valid {
			drone.FlightTime = flight.String
		}

		// 解析时间
		firstSeen, err := time.Parse(time.RFC3339, firstSeenStr)
		if err != nil {
			firstSeen = time.Now().UTC()
		}
		drone.FirstSeen = firstSeen

		lastSeen, err := time.Parse(time.RFC3339, lastSeenStr)
		if err != nil {
			lastSeen = time.Now().UTC()
		}
		drone.LastSeen = lastSeen

		drones = append(drones, drone)
	}

	return drones, nil
}

// SaveDrone 保存或更新无人机数据
func SaveDrone(drone *types.DroneData) error {
	firstSeen := drone.FirstSeen.UTC().Format(time.RFC3339)
	lastSeen := drone.LastSeen.UTC().Format(time.RFC3339)

	_, err := db.ExecContext(ctx, `
		INSERT INTO drones (
			mac, first_seen, last_seen, uas_id, ua_type, id_type,
			latitude, longitude, altitude, speed, heading,
			operator_latitude, operator_longitude, area_radius_m,
			classification_region, china_compliant, standard,
			signal_strength, battery_level, flight_time
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(mac) DO UPDATE SET
			last_seen = excluded.last_seen,
			uas_id = excluded.uas_id,
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
			china_compliant = excluded.china_compliant,
			standard = excluded.standard,
			signal_strength = COALESCE(excluded.signal_strength, drones.signal_strength),
			battery_level = COALESCE(excluded.battery_level, drones.battery_level),
			flight_time = COALESCE(excluded.flight_time, drones.flight_time)
	`,
		drone.MAC, firstSeen, lastSeen, drone.UASID, drone.UAType, nullString(drone.IDType),
		nullFloat64(drone.Latitude), nullFloat64(drone.Longitude), nullFloat64(drone.Altitude),
		nullFloat64(drone.Speed), nullFloat64(drone.Heading),
		nullFloat64(drone.OperatorLatitude), nullFloat64(drone.OperatorLongitude), nullInt(drone.AreaRadiusM),
		nullString(drone.Classification), boolToSQLiteBool(drone.ChinaCompliant), drone.Standard,
		nullString(drone.SignalStrength), nullString(drone.BatteryLevel), nullString(drone.FlightTime),
	)
	if err != nil {
		return fmt.Errorf("保存无人机数据失败: %w", err)
	}

	return nil
}

// SavePosition 保存无人机位置记录
func SavePosition(mac string, pos *types.Position, standard string) error {
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

// nullFloat64 将 float64 转为可空值（0 视为有效值，如速度为 0 表示悬停）
func nullFloat64(v float64) interface{} {
	return v
}

// nullInt 将 int 转为可空值（0 视为有效值）
func nullInt(v int) interface{} {
	return v
}

// nullString 将 string 转为可空值
func nullString(v string) interface{} {
	if v == "" {
		return nil
	}
	return v
}

// GetActiveDroneCount 获取活跃无人机数量
func GetActiveDroneCount() (int, error) {
	since := time.Now().UTC().Add(activeWindow).Format(time.RFC3339)

	var count int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM drones WHERE last_seen > ?
	`, since).Scan(&count)

	return count, err
}

// GetTotalDroneCount 获取总无人机数量
func GetTotalDroneCount() (int, error) {
	var count int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM drones
	`).Scan(&count)

	return count, err
}

// GetCompliantDroneCount 获取合规无人机数量
func GetCompliantDroneCount() (int, error) {
	var count int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM drones WHERE china_compliant = 1
	`).Scan(&count)

	return count, err
}

// CreateAlert 创建新警报
func CreateAlert(alertType, message, mac string) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	result, err := db.ExecContext(ctx, `
		INSERT INTO alerts (alert_type, message, timestamp, mac)
		VALUES (?, ?, ?, ?)
	`, alertType, message, now, mac)

	if err != nil {
		return 0, fmt.Errorf("创建警报失败: %w", err)
	}

	return result.LastInsertId()
}

// GetAlertByID 根据ID获取警报
func GetAlertByID(id int) (*types.Alert, error) {
	alert := &types.Alert{}
	err := db.QueryRowContext(ctx, `
		SELECT id, alert_type, message, timestamp, mac, resolved
		FROM alerts
		WHERE id = ?
	`, id).Scan(
		&alert.ID, &alert.Type, &alert.Message, &alert.Timestamp, &alert.MAC, &alert.Resolved,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("警报未找到")
		}
		return nil, fmt.Errorf("查询警报失败: %w", err)
	}

	return alert, nil
}

// ResolveAlert 标记警报为已解决
func ResolveAlert(id int) error {
	now := time.Now().UTC().Format(time.RFC3339)

	result, err := db.ExecContext(ctx, `
		UPDATE alerts
		SET resolved = 1, resolved_at = ?
		WHERE id = ?
	`, now, id)

	if err != nil {
		return fmt.Errorf("标记警报失败: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取影响行数失败: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("警报未找到")
	}

	return nil
}

// GetAlerts 获取警报列表
func GetAlerts(limit, offset int) ([]types.Alert, int, error) {
	// 获取总数量
	var total int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM alerts
	`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("获取警报总数失败: %w", err)
	}

	// 获取分页数据
	rows, err := db.QueryContext(ctx, `
		SELECT id, alert_type, message, timestamp, mac, resolved
		FROM alerts
		ORDER BY timestamp DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("查询警报列表失败: %w", err)
	}
	defer rows.Close()

	var alerts []types.Alert
	for rows.Next() {
		alert := types.Alert{}
		if err := rows.Scan(
			&alert.ID, &alert.Type, &alert.Message, &alert.Timestamp, &alert.MAC, &alert.Resolved,
		); err != nil {
			return nil, 0, fmt.Errorf("扫描警报数据失败: %w", err)
		}
		alerts = append(alerts, alert)
	}

	return alerts, total, nil
}

// GetAlertStatistics 获取警报统计
func GetAlertStatistics() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 1. 总警报数
	var totalAlerts int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM alerts
	`).Scan(&totalAlerts); err != nil {
		return nil, fmt.Errorf("获取总警报数失败: %w", err)
	}
	stats["total_alerts"] = totalAlerts

	// 2. 未解决警报数
	var unresolvedAlerts int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM alerts WHERE resolved = 0
	`).Scan(&unresolvedAlerts); err != nil {
		return nil, fmt.Errorf("获取未解决警报数失败: %w", err)
	}
	stats["unresolved_alerts"] = unresolvedAlerts

	// 3. 按类型统计
	rows, err := db.QueryContext(ctx, `
		SELECT alert_type, COUNT(*) as count
		FROM alerts
		GROUP BY alert_type
		ORDER BY count DESC
		LIMIT 5
	`)
	if err != nil {
		return nil, fmt.Errorf("获取警报类型统计失败: %w", err)
	}
	defer rows.Close()

	typeCounts := make(map[string]int)
	for rows.Next() {
		var alertType string
		var count int
		if err := rows.Scan(&alertType, &count); err != nil {
			return nil, fmt.Errorf("扫描警报类型数据失败: %w", err)
		}
		typeCounts[alertType] = count
	}
	stats["alert_types"] = typeCounts

	// 4. 24小时警报趋势
	trend, err := getAlertTrend(24)
	if err != nil {
		return nil, fmt.Errorf("获取警报趋势失败: %w", err)
	}
	stats["trend_24h"] = trend

	// 5. 最近警报
	recentAlerts, _, err := GetAlerts(5, 0)
	if err != nil {
		return nil, fmt.Errorf("获取最近警报失败: %w", err)
	}
	stats["recent_alerts"] = recentAlerts

	return stats, nil
}

// getAlertTrend 获取警报趋势
func getAlertTrend(hours int) ([]map[string]interface{}, error) {
	since := time.Now().UTC().Add(-time.Duration(hours) * time.Hour).Format(time.RFC3339)

	rows, err := db.QueryContext(ctx, `
		SELECT 
			strftime('%Y-%m-%d %H:00', timestamp) as hour,
			COUNT(*) as count
		FROM alerts
		WHERE timestamp > ?
		GROUP BY hour
		ORDER BY hour ASC
	`, since)
	if err != nil {
		return nil, fmt.Errorf("获取警报趋势失败: %w", err)
	}
	defer rows.Close()

	var trend []map[string]interface{}
	for rows.Next() {
		var hour string
		var count int
		if err := rows.Scan(&hour, &count); err != nil {
			return nil, fmt.Errorf("扫描警报趋势数据失败: %w", err)
		}
		trend = append(trend, map[string]interface{}{
			"hour":  hour,
			"count": count,
		})
	}

	return trend, nil
}

// ClearOldAlerts 清除旧警报
func ClearOldAlerts(hours int) (int64, error) {
	if hours < 1 {
		hours = 1
	}

	since := time.Now().UTC().Add(-time.Duration(hours) * time.Hour).Format(time.RFC3339)

	result, err := db.ExecContext(ctx, `
		DELETE FROM alerts
		WHERE timestamp < ? AND resolved = 1
	`, since)

	if err != nil {
		return 0, fmt.Errorf("清除旧警报失败: %w", err)
	}

	return result.RowsAffected()
}

// SearchAlerts 搜索警报
func SearchAlerts(query string, limit int) ([]types.Alert, error) {
	if limit < 1 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	// 构建安全的搜索条件
	searchPattern := "%" + sanitizeSearchQuery(query) + "%"

	rows, err := db.QueryContext(ctx, `
		SELECT id, alert_type, message, timestamp, mac, resolved
		FROM alerts
		WHERE message LIKE ? OR alert_type LIKE ? OR mac LIKE ?
		ORDER BY timestamp DESC
		LIMIT ?
	`, searchPattern, searchPattern, searchPattern, limit)
	if err != nil {
		return nil, fmt.Errorf("搜索警报失败: %w", err)
	}
	defer rows.Close()

	var alerts []types.Alert
	for rows.Next() {
		alert := types.Alert{}
		if err := rows.Scan(
			&alert.ID, &alert.Type, &alert.Message, &alert.Timestamp, &alert.MAC, &alert.Resolved,
		); err != nil {
			return nil, fmt.Errorf("扫描搜索结果失败: %w", err)
		}
		alerts = append(alerts, alert)
	}

	return alerts, nil
}

// sanitizeSearchQuery 清理搜索查询
func sanitizeSearchQuery(query string) string {
	// 移除特殊字符
	return strings.Map(func(r rune) rune {
		if r == '%' || r == '_' || r == '\\' {
			return -1
		}
		return r
	}, query)
}

// updateSystemStats 更新系统统计信息
func updateSystemStats(tx *sql.Tx) error {
	// 1. 更新无人机总数
	var droneCount int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM drones
	`).Scan(&droneCount); err != nil {
		return err
	}

	if err := updateSystemValue(tx, "total_drones", strconv.Itoa(droneCount)); err != nil {
		return err
	}

	// 2. 更新活跃无人机数
	since := time.Now().UTC().Add(activeWindow).Format(time.RFC3339)
	var activeCount int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM drones WHERE last_seen > ?
	`, since).Scan(&activeCount); err != nil {
		return err
	}

	if err := updateSystemValue(tx, "active_drones", strconv.Itoa(activeCount)); err != nil {
		return err
	}

	// 3. 更新最后更新时间
	if err := updateSystemValue(tx, "last_updated", time.Now().UTC().Format(time.RFC3339)); err != nil {
		return err
	}

	return nil
}

// updateSystemValue 更新系统值
func updateSystemValue(tx *sql.Tx, key, value string) error {
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := tx.ExecContext(ctx, `
		INSERT INTO system_info (key, value, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET
			value = excluded.value,
			updated_at = excluded.updated_at
	`, key, value, now)

	return err
}

// ExportData 导出所有数据
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

// exportJSON 导出为 JSON
func exportJSON() (string, error) {
	// 获取所有无人机
	drones, err := GetActiveDrones()
	if err != nil {
		return "", err
	}

	// 获取未解决警报
	alerts, _, err := GetAlerts(100, 0)
	if err != nil {
		return "", err
	}

	// 构建导出数据
	exportData := map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"drones":    drones,
		"alerts":    alerts,
		"system": map[string]interface{}{
			"total_drones":      len(drones),
			"unresolved_alerts": len(alerts),
		},
	}

	// 转换为 JSON
	jsonData, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		return "", fmt.Errorf("JSON 编码失败: %w", err)
	}

	return string(jsonData), nil
}

// exportCSV 导出为 CSV
func exportCSV() (string, error) {
	// 获取所有位置数据
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

	// 构建 CSV
	var csv strings.Builder
	csv.WriteString("timestamp,latitude,longitude,altitude,uas_id,mac,standard\n")

	for rows.Next() {
		var timestamp, uasID, mac, standard string
		var lat, lon, alt float64

		if err := rows.Scan(&timestamp, &lat, &lon, &alt, &uasID, &mac, &standard); err != nil {
			return "", fmt.Errorf("扫描位置数据失败: %w", err)
		}

		csv.WriteString(fmt.Sprintf("%s,%.6f,%.6f,%.1f,%s,%s,%s\n",
			timestamp, lat, lon, alt, uasID, mac, standard))
	}

	return csv.String(), nil
}
