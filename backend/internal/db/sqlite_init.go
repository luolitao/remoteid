package db

import (
	"fmt"
)

// bootstrapDatabase 负责初始化目录、创建表及索引、处理版本迁移
func bootstrapDatabase() error {
	if err := createTables(); err != nil {
		return err
	}
	if err := migrateSchema(); err != nil {
		return err
	}
	if err := createIndexes(); err != nil {
		return err
	}
	return nil
}

func createTables() error {
	// 基础无人机静态表
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS drones (
			mac TEXT PRIMARY KEY,
			uas_id TEXT,
			standard TEXT,
			first_seen DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		return fmt.Errorf("创建 drones 表失败: %w", err)
	}

	// 动态遥测轨迹位置表
	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS positions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			mac TEXT,
			latitude REAL,
			longitude REAL,
			altitude REAL,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(mac) REFERENCES drones(mac)
		);
	`)
	if err != nil {
		return fmt.Errorf("创建 positions 表失败: %w", err)
	}
	return nil
}

func migrateSchema() error {
	// 留作未来升级字段时的迁移占位逻辑（防止破坏已有数据）
	// 例如：ALTER TABLE drones ADD COLUMN operator_id TEXT;
	return nil
}

func createIndexes() error {
	// 💡 针对高频的历史轨迹查询 `GetTrajectory` 和 `exportCSV` 建立复合覆盖索引
	_, err := db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_positions_mac_timestamp 
		ON positions(mac, timestamp DESC);
	`)
	if err != nil {
		return fmt.Errorf("创建位置索引失败: %w", err)
	}
	return nil
}
