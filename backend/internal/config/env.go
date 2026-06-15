package config

import (
	"os"
	"strings"
)

// ApplyEnvironmentVars 允许通过环境变量覆盖配置文件中的值
func ApplyEnvironmentVars() {
	c := Get()

	if path := os.Getenv("REMOTEID_MONITOR_DB_PATH"); path != "" {
		c.Database.Path = path
	}
	if iface := os.Getenv("REMOTEID_MONITOR_IFACE"); iface != "" {
		c.Network.Interface = iface
	}
	if host := os.Getenv("REMOTEID_MONITOR_API_HOST"); host != "" {
		c.API.Host = host
	}
	if port := os.Getenv("REMOTEID_MONITOR_PORT"); port != "" {
		c.API.Port = port
	}
	// ✅ 新增：支持通过逗号分隔的字符串覆盖 CORS 列表
	if cors := os.Getenv("REMOTEID_MONITOR_CORS_ORIGINS"); cors != "" {
		origins := strings.Split(cors, ",")
		for i := range origins {
			origins[i] = strings.TrimSpace(origins[i])
		}
		c.API.CORSAllowOrigins = origins
	}
	if debug := os.Getenv("REMOTEID_MONITOR_DEBUG"); debug != "" {
		c.Debug = debug == "true" || debug == "1"
	}
}
