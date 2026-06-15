package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"remoteid-monitor/internal/config"

	_ "github.com/mattn/go-sqlite3"
)

const (
	dbMaxRetries    = 3
	dbRetryDelay    = 500 * time.Millisecond
	activeWindow    = -5 * time.Minute
	maxConnLifetime = 30 * time.Minute
)

var (
	db  *sql.DB
	ctx context.Context
)

// Init 初始化数据库连接
func Init() error {
	cfg := config.Get()
	if err := createDataDir(cfg.Database.Path); err != nil {
		return fmt.Errorf("创建数据目录失败: %w", err)
	}

	dbPath := getDatabasePath(cfg.Database.Path)
	var err error
	db, err = sql.Open("sqlite3", fmt.Sprintf("%s?_journal_mode=WAL&_cache_size=%d&_synchronous=NORMAL&_timeout=5000",
		dbPath, cfg.Database.CacheSize))
	if err != nil {
		return fmt.Errorf("打开数据库失败: %w", err)
	}

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

	db.SetMaxOpenConns(cfg.Database.MaxConnections)
	db.SetMaxIdleConns(cfg.Database.MaxConnections / 2)
	db.SetConnMaxLifetime(maxConnLifetime)

	ctx = context.Background()

	if err := createTables(); err != nil {
		db.Close()
		return fmt.Errorf("创建表结构失败: %w", err)
	}
	if err := createIndexes(); err != nil {
		db.Close()
		return fmt.Errorf("创建索引失败: %w", err)
	}

	slog.Info("SQLite 数据库初始化成功", "path", dbPath)
	return nil
}

// Close 关闭数据库连接
func Close() {
	if db != nil {
		db.Close()
		slog.Info("SQLite 数据库连接已关闭")
	}
}

// --- 辅助函数 ---

func nullFloat64(v float64) interface{} { return v }
func nullInt(v int) interface{}         { return v }
func nullString(v string) interface{} {
	if v == "" {
		return nil
	}
	return v
}

func createDataDir(dbPath string) error {
	dir := filepath.Dir(dbPath)
	if dir == "" || dir == "." {
		return nil
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建目录失败: %w", err)
		}
		slog.Info("创建数据目录", "dir", dir)
	}
	return nil
}

func getDatabasePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	cwd, err := os.Getwd()
	if err != nil {
		slog.Warn("获取当前工作目录失败", "error", err)
		cwd = "/data"
	}
	return filepath.Join(cwd, path)
}
