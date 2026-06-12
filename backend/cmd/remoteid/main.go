package main

import (
	"context"
	"encoding/json"
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
				t := a.Value.Time()
				a.Value = slog.StringValue(t.Format("2006-01-02T15:04:05"))
			}
			return a
		},
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, opts))
	slog.SetDefault(logger)
}

func main() {
	// 1. 注册命令行参数
	// 💡 注意：因为 config 包可能不支持指定路径，保留 flag 仅用于命令行占位防止报错
	_ = flag.String("config", "config.yaml", "配置文件路径")
	ifaceFlag := flag.String("iface", "", "指定网络监听网卡接口 (例如 wlan0, wlan2)")
	flag.Parse()

	slog.Info("RemoteID 监控系统正在启动...")

	// 💡 2. 修正：移除错误的 config.Init 调用，直接获取全局配置
	cfg := config.Get()

	// 3. 初始化数据库
	dbPath := "remoteid.db"
	if cfg.Database.Path != "" {
		dbPath = cfg.Database.Path
	}
	if err := db.Init(dbPath); err != nil {
		slog.Error("数据库初始化失败", "error", err)
		os.Exit(1)
	}
	slog.Info("数据库初始化成功", "path", dbPath)

	// 4. 核心管道与组件装配
	broadcastCh := make(chan *drone.TrackedDrone, 2048)
	processor := drone.NewProcessor(broadcastCh)

	// 5. 网卡优先级抉择
	networkDevice := "wlan2"
	if *ifaceFlag != "" {
		networkDevice = *ifaceFlag
		slog.Info("使用命令行指定的网卡接口", "iface", networkDevice)
	} else if cfg.Network.Interface != "" {
		networkDevice = cfg.Network.Interface
		slog.Info("使用配置文件中的网卡接口", "iface", networkDevice)
	} else {
		slog.Warn("未指定网卡，将使用系统默认接口", "iface", networkDevice)
	}

	sniffer := drone.NewSniffer(networkDevice, processor)
	wsManager := ws.NewManager()

	// 6. 处理数据广播
	var totalReceived int64
	go func() {
		slog.Info("WebSocket 广播转发协程已启动")
		for data := range broadcastCh {
			totalReceived++

			msgBytes, err := json.Marshal(data)
			if err != nil {
				slog.Error("WebSocket 序列化失败", "error", err)
				continue
			}

			// 💡 修正：不再盲猜 data.ID 等字段，直接打印整段 JSON 字符串来安全观察无人机数据
			// 🔄 优化：避免日志刷屏，改为每 100 次输出一次（包含第 1 次以便确认连通）
			if totalReceived == 1 || totalReceived%200 == 0 {
				slog.Info("📡 [数据流转成功] 解析到无人机数据！",
					"totalReceived", totalReceived,
					"raw_json", string(msgBytes),
				)
			}

			wsManager.Broadcast(msgBytes)
		}
	}()

	// 7. 初始化并异步启动 API 服务
	serverHandler := api.NewServer(processor, wsManager, sniffer)
	go func() {
		port := ":" + cfg.API.Port
		slog.Info("API 服务尝试绑定端口", "port", cfg.API.Port)
		if err := serverHandler.Run(port); err != nil && err != http.ErrServerClosed {
			slog.Error("API 服务异常退出", "error", err)
			os.Exit(1)
		}
	}()

	// 8. 异步启动 Sniffer 硬件抓包
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		slog.Info("Sniffer 捕获协程正在启动...", "device", networkDevice)
		if err := sniffer.Start(ctx); err != nil {
			slog.Error("Sniffer 捕获异常中止", "error", err)
			os.Exit(1)
		}
	}()

	// 9. 优雅关闭响应机制
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("收到停机信号，正在释放资源...")

	// 设定 5 秒优雅关闭缓冲区
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	cancel()
	processor.Close()
	if err := serverHandler.Shutdown(shutdownCtx); err != nil {
		slog.Error("服务优雅关闭失败", "error", err)
		os.Exit(1)
	}

	slog.Info("RemoteID 监控系统已安全停止")
}
