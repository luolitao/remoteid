// internal/config/config.go
package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

var cfg *Config

// Config 应用配置结构体
type Config struct {
	Database struct {
		Path           string `json:"path"            yaml:"path"`
		MaxConnections int    `json:"max_connections" yaml:"max_connections"`
		CacheSize      int    `json:"cache_size"      yaml:"cache_size"`
	} `json:"database" yaml:"database"`

	Network struct {
		Interface   string `json:"interface"    yaml:"interface"`
		Channel     int    `json:"channel"      yaml:"channel"`
		MonitorMode bool   `json:"monitor_mode" yaml:"monitor_mode"`
	} `json:"network" yaml:"network"`

	API struct {
		Port string   `json:"port" yaml:"port"`
		CORS []string `json:"cors" yaml:"cors"`
	} `json:"api" yaml:"api"`

	Logging struct {
		Level string `json:"level" yaml:"level"`
		File  string `json:"file"  yaml:"file"`
	} `json:"logging" yaml:"logging"`

	Debug bool `json:"debug" yaml:"debug"`
}

// Load 从配置文件加载配置
func Load(configFile string) error {
	// 1. 检查文件是否存在
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		slog.Warn("配置文件不存在，使用默认配置", "file", configFile)
		cfg = getDefaultConfig()
		return nil
	}

	// 2. 读取配置文件
	data, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 3. 检查文件是否为空
	if len(data) == 0 {
		slog.Warn("配置文件为空，使用默认配置", "file", configFile)
		cfg = getDefaultConfig()
		return nil
	}

	// 4. 确定配置文件类型
	ext := filepath.Ext(configFile)
	switch strings.ToLower(ext) {
	case ".json":
		return loadJSON(data)
	case ".yaml", ".yml":
		return loadYAML(data)
	default:
		slog.Warn("不支持的配置文件类型，使用默认配置", "ext", ext)
		cfg = getDefaultConfig()
		return nil
	}
}

// +++ 新增：loadJSON 函数实现 +++
func loadJSON(data []byte) error {
	cfg = &Config{}

	// 解析 JSON
	if err := json.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("解析 JSON 配置失败: %w", err)
	}

	// 验证和设置默认值
	validateConfig()

	slog.Info("JSON 配置加载成功")
	return nil
}

// +++ 新增：loadYAML 函数实现 +++
func loadYAML(data []byte) error {
	cfg = &Config{}

	// 解析 YAML
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("解析 YAML 配置失败: %w", err)
	}

	// 验证和设置默认值
	validateConfig()

	slog.Info("YAML 配置加载成功")
	return nil
}

// validateConfig 验证配置有效性
func validateConfig() {
	if cfg == nil {
		cfg = getDefaultConfig()
		return
	}

	// 1. 验证数据库路径
	if cfg.Database.Path == "" {
		cfg.Database.Path = "/tmp/remoteid.db"
	}

	// 2. 验证网络接口
	if cfg.Network.Interface == "" {
		cfg.Network.Interface = "wlan1"
	}

	// 3. 验证 API 端口
	if cfg.API.Port == "" {
		cfg.API.Port = "8000"
	}

	// 4. 验证 CORS
	if len(cfg.API.CORS) == 0 {
		cfg.API.CORS = []string{"*"}
	}

	// 5. 验证日志级别
	switch strings.ToLower(cfg.Logging.Level) {
	case "debug", "info", "warn", "error", "fatal":
		// 有效级别
	default:
		cfg.Logging.Level = "info"
	}

	// 6. 树莓派优化
	if cfg.Database.MaxConnections == 0 {
		cfg.Database.MaxConnections = 5
	}
	if cfg.Database.CacheSize == 0 {
		cfg.Database.CacheSize = 512
	}
}

// getDefaultConfig 获取默认配置
func getDefaultConfig() *Config {
	defaultCfg := &Config{
		Database: struct {
			Path           string `json:"path"            yaml:"path"`
			MaxConnections int    `json:"max_connections" yaml:"max_connections"`
			CacheSize      int    `json:"cache_size"      yaml:"cache_size"`
		}{
			Path:           "/tmp/remoteid.db",
			MaxConnections: 5,
			CacheSize:      512,
		},
		Network: struct {
			Interface   string `json:"interface"    yaml:"interface"`
			Channel     int    `json:"channel"      yaml:"channel"`
			MonitorMode bool   `json:"monitor_mode" yaml:"monitor_mode"`
		}{
			Interface:   "wlan1",
			Channel:     157,
			MonitorMode: true,
		},
		API: struct {
			Port string   `json:"port" yaml:"port"`
			CORS []string `json:"cors" yaml:"cors"`
		}{
			Port: "8000",
			CORS: []string{"*"},
		},
		Logging: struct {
			Level string `json:"level" yaml:"level"`
			File  string `json:"file"  yaml:"file"`
		}{
			Level: "info",
			File:  "/tmp/remoteid.log",
		},
		Debug: true,
	}

	// 创建数据目录
	dataDir := filepath.Dir(defaultCfg.Database.Path)
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		os.MkdirAll(dataDir, 0755)
	}

	// 检查目录权限
	if info, err := os.Stat(dataDir); err == nil {
		if info.Mode().Perm()&0200 == 0 {
			os.Chmod(dataDir, 0755)
		}
	}

	return defaultCfg
}

// Get 获取配置
func Get() *Config {
	if cfg == nil {
		cfg = getDefaultConfig()
	}
	return cfg
}

// SetLogLevel 根据配置设置全局日志级别
func SetLogLevel() {
	var level slog.Level
	switch strings.ToLower(cfg.Logging.Level) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, opts))
	slog.SetDefault(logger)
}
