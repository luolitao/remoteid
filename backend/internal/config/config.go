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

// defaultCORSOrigins 当配置文件未指定 CORS 允许的来源时，使用的安全默认列表
var defaultCORSOrigins = []string{
	"http://localhost:8080",
	"http://127.0.0.1:8080",
	"http://localhost:3000",
	"http://127.0.0.1:3000",
	"http://192.168.6.30:8080",
	"http://192.168.6.30",
}

// Config 应用配置结构体
type Config struct {
	Database DatabaseConfig `json:"database" yaml:"database"`
	Network  NetworkConfig  `json:"network" yaml:"network"`
	API      APIConfig      `json:"api" yaml:"api"`
	Logging  LoggingConfig  `json:"logging" yaml:"logging"`
	Debug    bool           `json:"debug" yaml:"debug"`
}

type DatabaseConfig struct {
	Path           string `json:"path" yaml:"path"`
	MaxConnections int    `json:"max_connections" yaml:"max_connections"`
	CacheSize      int    `json:"cache_size" yaml:"cache_size"`
}

type NetworkConfig struct {
	Interface   string `json:"interface" yaml:"interface"`
	Channel     int    `json:"channel" yaml:"channel"`
	MonitorMode bool   `json:"monitor_mode" yaml:"monitor_mode"`
}

type APIConfig struct {
	Host             string   `json:"host" yaml:"host"`                             // 监听地址，默认 "0.0.0.0"
	Port             string   `json:"port" yaml:"port"`                             // 监听端口
	CORSAllowOrigins []string `json:"cors_allow_origins" yaml:"cors_allow_origins"` // CORS 和 WebSocket 允许的 Origin 列表
}

type LoggingConfig struct {
	Level string `json:"level" yaml:"level"`
	File  string `json:"file" yaml:"file"`
}

// Load 从配置文件加载配置
func Load(configFile string) error {
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		slog.Warn("配置文件不存在，使用默认配置", "file", configFile)
		cfg = getDefaultConfig()
		return nil
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	if len(data) == 0 {
		slog.Warn("配置文件为空，使用默认配置", "file", configFile)
		cfg = getDefaultConfig()
		return nil
	}

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

func loadJSON(data []byte) error {
	cfg = &Config{}
	if err := json.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("解析 JSON 配置失败: %w", err)
	}
	validateConfig()
	slog.Info("JSON 配置加载成功")
	return nil
}

func loadYAML(data []byte) error {
	cfg = &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("解析 YAML 配置失败: %w", err)
	}
	validateConfig()
	slog.Info("YAML 配置加载成功")
	return nil
}

// validateConfig 验证配置有效性并填充默认值
func validateConfig() {
	if cfg == nil {
		cfg = getDefaultConfig()
		return
	}

	if cfg.Database.Path == "" {
		cfg.Database.Path = "/tmp/remoteid-monitor.db"
	}
	if cfg.Network.Interface == "" {
		cfg.Network.Interface = "wlan1"
	}
	if cfg.API.Host == "" {
		cfg.API.Host = "0.0.0.0" // 默认监听所有网络接口
	}
	if cfg.API.Port == "" {
		cfg.API.Port = "8080"
	}

	// ✅ 核心优化：如果未配置 CORS，使用安全的默认白名单，而不是 "*"
	if len(cfg.API.CORSAllowOrigins) == 0 {
		cfg.API.CORSAllowOrigins = defaultCORSOrigins
	}

	switch strings.ToLower(cfg.Logging.Level) {
	case "debug", "info", "warn", "error", "fatal":
		// 有效级别
	default:
		cfg.Logging.Level = "info"
	}

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
		Database: DatabaseConfig{
			Path:           "/tmp/remoteid-monitor.db",
			MaxConnections: 5,
			CacheSize:      512,
		},
		Network: NetworkConfig{
			Interface:   "wlan1",
			Channel:     6, // 2.4GHz 默认社会信道
			MonitorMode: true,
		},
		API: APIConfig{
			Host:             "0.0.0.0",
			Port:             "8080",
			CORSAllowOrigins: defaultCORSOrigins, // ✅ 默认注入安全列表
		},
		Logging: LoggingConfig{
			Level: "info",
			File:  "/tmp/remoteid-monitor.log",
		},
		Debug: false,
	}

	dataDir := filepath.Dir(defaultCfg.Database.Path)
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		os.MkdirAll(dataDir, 0755)
	}

	return defaultCfg
}

// Get 获取全局配置实例
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
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				t := a.Value.Time()
				a.Value = slog.StringValue(t.Format("2006-01-02T15:04:05.000"))
			}
			return a
		},
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, opts))
	slog.SetDefault(logger)
}
