package config

import (
	"fmt"
	"net"
)

// internal/config/validator.go
func ValidateConfig(cfg *Config) error {
	if cfg.Database.MaxConnections > 20 {
		return fmt.Errorf("数据库连接数过高: %d (最大 20)", cfg.Database.MaxConnections)
	}

	if cfg.Network.Channel < 1 || cfg.Network.Channel > 165 {
		return fmt.Errorf("无效的 WiFi 信道: %d", cfg.Network.Channel)
	}

	if _, err := net.ResolveTCPAddr("tcp", ":"+cfg.API.Port); err != nil {
		return fmt.Errorf("无效的 API 端口: %s", cfg.API.Port)
	}

	return nil
}
