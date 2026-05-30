package config

import (
	"log/slog"

	"github.com/fsnotify/fsnotify"
)

// internal/config/reloader.go
func StartConfigReloader(configFile string, reloadFunc func()) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Warn("创建文件监视器失败", "error", err)
		return
	}

	go func() {
		defer watcher.Close()
		watcher.Add(configFile)

		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Write) {
					slog.Info("检测到配置文件变更，重新加载...")
					if err := Load(configFile); err == nil {
						reloadFunc()
						slog.Info("配置重新加载成功")
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				slog.Warn("配置监视器错误", "error", err)
			}
		}
	}()
}
