package config

import (
	"fmt"
	"net"
)

func ValidateConfig(cfg *Config) error {
	if cfg.Database.MaxConnections > 20 {
		return fmt.Errorf("数据库连接数过高: %d (最大 20)", cfg.Database.MaxConnections)
	}
	if cfg.Network.Channel < 1 || cfg.Network.Channel > 165 {
		return fmt.Errorf("无效的 WiFi 信道: %d (有效范围 1-165)", cfg.Network.Channel)
	}
	if _, err := net.ResolveTCPAddr("tcp", ":"+cfg.API.Port); err != nil {
		return fmt.Errorf("无效的 API 端口: %s", cfg.API.Port)
	}
	if cfg.API.Host == "" {
		cfg.API.Host = "0.0.0.0"
	}
	// ✅ 修正：使用 defaultCORSOrigins（在 config.go 中定义）
	if len(cfg.API.CORSAllowOrigins) == 0 {
		cfg.API.CORSAllowOrigins = defaultCORSOrigins
	}
	return nil
}
