package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"remoteid-monitor/internal/api"
	"remoteid-monitor/internal/config"
	"remoteid-monitor/internal/db"
	"remoteid-monitor/internal/drone"
	"remoteid-monitor/pkg/ws"
)

func init() {
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
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

func main() {
	cfgFile := flag.String("config", "config.yaml", "配置文件路径")
	iface := flag.String("iface", "wlan1", "监控网络接口")
	flag.Parse()

	if err := config.Load(*cfgFile); err != nil {
		slog.Error("配置加载失败", "error", err)
		os.Exit(1)
	}
	config.SetLogLevel()
	slog.Info("加载配置", "file", *cfgFile)

	if err := db.Init(); err != nil {
		slog.Error("数据库初始化失败", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	slog.Info("数据库初始化成功")

	wsManager := ws.NewManager()

	// ✅ 1. 使用 Manager 统一编排 (替代直接创建 Sniffer 和 Processor)
	manager := drone.NewManager(wsManager, *iface)

	// 始终 defer Stop，确保即使 Start 部分失败也能安全清理资源
	defer manager.Stop()

	if err := manager.Start(); err != nil {
		slog.Warn("监控管理器启动失败，API 服务仍可运行（无实时数据）", "error", err)
	} else {
		slog.Info("2.4GHz 监控引擎已启动", "interface", *iface)
	}

	// ✅ 2. 创建 API 服务器，直接传入 manager (因为它实现了 StatusProvider 接口)
	// 在 main.go 中，找到创建 API 服务器的部分：

	// 原代码: server := api.NewServer(processor, wsManager, sniffer)
	// 修改为:
	server := api.NewServer(manager.GetProcessor(), wsManager, manager)
	// 3. 启动服务器
	serverAddr := ":" + config.Get().API.Port

	// 启动服务器部分也需微调，因为 Run() 不再需要传参：
	// 原代码: go func() { if err := server.Run(serverAddr); ... }
	// 修改为:
	go func() {
		if err := server.Run(serverAddr); err != nil && err != http.ErrServerClosed {
			slog.Error("服务器启动失败", "error", err)
			os.Exit(1)
		}
	}()
	slog.Info("API 服务启动", "addr", serverAddr)

	// 4. 优雅关闭
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	slog.Info("正在关闭服务...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Warn("服务器关闭错误", "error", err)
	}

	// manager.Stop() 会通过 defer 自动调用，安全停止所有 Capturer
	slog.Info("服务已安全关闭")
}
