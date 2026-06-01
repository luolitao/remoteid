package config

import "os"

// internal/config/env.go
func ApplyEnvironmentVars() {
	if path := os.Getenv("REMOTEID_MONITOR_DB_PATH"); path != "" {
		Get().Database.Path = path
	}

	if iface := os.Getenv("REMOTEID_MONITOR_IFACE"); iface != "" {
		Get().Network.Interface = iface
	}

	if port := os.Getenv("REMOTEID_MONITOR_PORT"); port != "" {
		Get().API.Port = port
	}

	if debug := os.Getenv("REMOTEID_MONITOR_DEBUG"); debug != "" {
		Get().Debug = debug == "true" || debug == "1"
	}
}
