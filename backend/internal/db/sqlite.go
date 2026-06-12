package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3" // 引入 SQLite 驱动
)

var (
	db  *sql.DB
	ctx = context.Background()
)

const (
	dbMaxRetries    = 3
	dbRetryDelay    = 1 * time.Second
	activeWindow    = 30 * time.Second
	maxConnLifetime = 1 * time.Hour
)

// Init 建立连接并配置高性能连接池
func Init(dbPath string) error {
	var err error

	// 💡 优化点：开启 WAL 模式 (Write-Ahead Logging) 并设置 5000ms 忙等待超时，解决高频并发写入锁冲突
	dsn := fmt.Sprintf("%s?_journal_mode=WAL&_busy_timeout=5000", dbPath)

	db, err = sql.Open("sqlite3", dsn)
	if err != nil {
		return fmt.Errorf("打开数据库失败: %w", err)
	}

	// 💡 优化点：SQLite 作为单文件数据库，严格限制最大打开连接数为 1
	// 配合 WAL 模式可以实现单写多读，彻底避免 "database is locked" 异常
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(maxConnLifetime)

	if err = db.Ping(); err != nil {
		return fmt.Errorf("数据库 Ping 失败: %w", err)
	}

	// 调用初始化结构表
	if err = bootstrapDatabase(); err != nil {
		return err
	}

	log.Println("[Database] SQLite 高性能 WAL 模式连接池初始化成功")
	return nil
}

// Close 优雅关闭数据库连接，确保退出时缓冲区数据完整刷入磁盘
func Close() error {
	if db != nil {
		log.Println("[Database] 正在关闭数据库连接...")
		return db.Close()
	}
	return nil
}
