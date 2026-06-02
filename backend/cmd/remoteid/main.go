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
	// 设置默认日志格式（JSON 格式便于结构化解析）
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				// 格式: 2006-01-02T15:04:05.000（毫秒精度，无时区）
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
	// 1. 解析命令行参数
	cfgFile := flag.String("config", "config.yaml", "配置文件路径")
	iface := flag.String("iface", "wlan1", "监控网络接口")
	flag.Parse()

	// 2. 加载配置
	if err := config.Load(*cfgFile); err != nil {
		slog.Error("配置加载失败", "error", err)
		os.Exit(1)
	}
	// 根据配置设置日志级别
	config.SetLogLevel()
	slog.Info("加载配置", "file", *cfgFile)

	// 3. 初始化数据库
	if err := db.Init(); err != nil {
		slog.Error("数据库初始化失败", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	slog.Info("数据库初始化成功")

	// 4. 创建 WebSocket 管理器
	wsManager := ws.NewManager()

	// 5. 创建处理器
	processor := drone.NewProcessor(wsManager)

	// 6. 启动抓包引擎（开发环境中可能没有物理网卡，允许失败）
	var sniffer *drone.Sniffer
	sniffer = drone.NewSniffer(*iface, processor)
	if err := sniffer.Start(); err != nil {
		slog.Warn("2.4GHz 抓包启动失败，API 服务仍可运行（无实时数据）", "error", err)
		sniffer = nil
	} else {
		defer sniffer.Stop()
		slog.Info("2.4GHz 信道 6 监控已启动")
		slog.Info("抓包引擎启动", "interface", *iface, "mode", "monitor")
	}

	// 7. 创建 API 服务器
	server := api.NewServer(processor, wsManager, sniffer)

	// 8. 启动服务器
	serverAddr := ":" + config.Get().API.Port
	go func() {
		if err := server.Run(serverAddr); err != nil && err != http.ErrServerClosed {
			slog.Error("服务器启动失败", "error", err)
			os.Exit(1)
		}
	}()
	slog.Info("API 服务启动", "addr", serverAddr)

	// 9. 优雅关闭
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	slog.Info("正在关闭服务...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Warn("服务器关闭错误", "error", err)
	}

	if sniffer != nil {
		sniffer.Stop()
	}
	db.Close()
	slog.Info("服务已安全关闭")
}
