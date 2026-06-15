package db

import (
	"context"
	"fmt"
	"log/slog"
)

func createTables() error {
	if db == nil {
		return fmt.Errorf("数据库连接未初始化")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	tables := []string{
		`CREATE TABLE IF NOT EXISTS drones (
			mac TEXT PRIMARY KEY, first_seen TEXT NOT NULL, last_seen TEXT NOT NULL,
			uas_id TEXT NOT NULL, operator_id TEXT, ua_type TEXT NOT NULL, id_type TEXT,
			latitude REAL, longitude REAL, altitude REAL, speed REAL, heading REAL,
			operator_latitude REAL, operator_longitude REAL, area_radius_m INTEGER,
			classification_region TEXT, standard TEXT NOT NULL,
			signal_strength TEXT, battery_level TEXT, flight_time TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS system_info (
			id INTEGER PRIMARY KEY AUTOINCREMENT, key TEXT NOT NULL UNIQUE, value TEXT NOT NULL, updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS positions (
			id INTEGER PRIMARY KEY AUTOINCREMENT, mac TEXT NOT NULL, timestamp TEXT NOT NULL,
			latitude REAL NOT NULL, longitude REAL NOT NULL, altitude REAL, speed REAL, heading REAL, standard TEXT NOT NULL,
			FOREIGN KEY(mac) REFERENCES drones(mac) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS alerts (
			id INTEGER PRIMARY KEY AUTOINCREMENT, alert_type TEXT NOT NULL, message TEXT NOT NULL,
			timestamp TEXT NOT NULL, mac TEXT, resolved BOOLEAN DEFAULT 0, resolved_at TEXT,
			FOREIGN KEY(mac) REFERENCES drones(mac) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS trajectories (
			id INTEGER PRIMARY KEY AUTOINCREMENT, mac TEXT NOT NULL, start_time TEXT NOT NULL,
			end_time TEXT NOT NULL, point_count INTEGER NOT NULL, max_altitude REAL, total_distance REAL, standard TEXT NOT NULL,
			FOREIGN KEY(mac) REFERENCES drones(mac) ON DELETE CASCADE
		)`,
	}

	for _, sql := range tables {
		if _, err := db.ExecContext(ctx, sql); err != nil {
			return fmt.Errorf("创建表失败: %w", err)
		}
	}

	if err := migrateSchema(); err != nil {
		return fmt.Errorf("数据库迁移失败: %w", err)
	}
	slog.Info("数据库表结构创建成功")
	return nil
}

func createIndexes() error {
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_drones_last_seen ON drones(last_seen)`,
		`CREATE INDEX IF NOT EXISTS idx_positions_mac_timestamp ON positions(mac, timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_alerts_timestamp ON alerts(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_alerts_resolved ON alerts(resolved)`,
	}
	for _, sql := range indexes {
		if _, err := db.ExecContext(ctx, sql); err != nil {
			return fmt.Errorf("创建索引失败: %w", err)
		}
	}
	slog.Info("数据库索引创建成功")
	return nil
}

func migrateSchema() error {
	type columnAdd struct {
		table, column, spec string
	}
	migrations := []columnAdd{
		{"drones", "id_type", "TEXT"}, {"drones", "operator_id", "TEXT"},
		{"drones", "operator_latitude", "REAL"}, {"drones", "operator_longitude", "REAL"},
		{"drones", "area_radius_m", "INTEGER"}, {"drones", "classification_region", "TEXT"},
		{"drones", "signal_strength", "TEXT"}, {"drones", "battery_level", "TEXT"}, {"drones", "flight_time", "TEXT"},
	}

	for _, m := range migrations {
		var count int
		query := fmt.Sprintf("SELECT COUNT(*) FROM pragma_table_info('%s') WHERE name = ?", m.table)
		if err := db.QueryRowContext(ctx, query, m.column).Scan(&count); err != nil {
			continue
		}
		if count > 0 {
			continue
		}
		alterSQL := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", m.table, m.column, m.spec)
		if _, err := db.ExecContext(ctx, alterSQL); err != nil {
			slog.Warn("添加列失败", "table", m.table, "column", m.column, "error", err)
		} else {
			slog.Info("数据库迁移：添加列", "table", m.table, "column", m.column)
		}
	}
	return nil
}
