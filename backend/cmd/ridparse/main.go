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
	ifaceFlag := flag.String("iface", "", "指定网络监听网卡接口 (例如 wlan0, wlan2)")
	flag.Parse()

	slog.Info("RemoteID 监控系统正在启动...")

	// 智能探测配置文件
	targetConfig := *cfgFile
	if targetConfig == "config.yaml" {
		if _, err := os.Stat("config.yaml"); os.IsNotExist(err) {
			if _, errYml := os.Stat("config.yml"); errYml == nil {
				targetConfig = "config.yml"
				slog.Info("未检测到 config.yaml，自动适配并加载同路径下的 config.yml")
			}
		}
	}

	if err := config.Init(targetConfig); err != nil {
		slog.Error("加载配置文件失败", "path", targetConfig, "error", err)
		os.Exit(1)
	}

	cfg := config.Get()

	// 初始化数据库
	dbPath := "remoteid.db"
	if cfg.Database.Path != "" {
		dbPath = cfg.Database.Path
	}
	if err := db.Init(dbPath); err != nil {
		slog.Error("数据库初始化失败", "error", err)
		os.Exit(1)
	}
	slog.Info("数据库初始化成功", "path", dbPath)

	// 核心管道与组件装配
	broadcastCh := make(chan *drone.TrackedDrone, 2048)
	processor := drone.NewProcessor(broadcastCh)

	// 网卡优先级抉择
	networkDevice := "wlan1"
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

	// 💡 优化点：参考 ridparse 的在线监控思想，对数据流向进行结构化计数和监控
	var totalReceived int64
	go func() {
		slog.Info("WebSocket 广播转发协程已启动")
		for data := range broadcastCh {
			totalReceived++
			// 每收到 1 个或多个无人机数据时，在后台打出高亮调试日志，证明核心链路通了
			slog.Info("📡 [数据流转成功] 成功接收并解析到无人机 Remote ID 数据！",
				"ID", data.ID,
				"MAC", data.MAC,
				"Lat", data.Latitude,
				"Lng", data.Longitude,
				"累计接收总数", totalReceived,
			)

			msgBytes, err := json.Marshal(data)
			if err != nil {
				slog.Error("WebSocket 序列化失败", "error", err)
				continue
			}
			wsManager.Broadcast(msgBytes)
		}
	}()

	// 初始化并异步启动 API 服务
	serverHandler := api.NewServer(processor, wsManager, sniffer)
	go func() {
		port := ":" + cfg.API.Port
		slog.Info("API 服务尝试绑定端口", "port", cfg.API.Port)
		if err := serverHandler.Run(port); err != nil && err != http.ErrServerClosed {
			slog.Error("API 服务异常退出", "error", err)
			os.Exit(1)
		}
	}()

	// 异步启动 Sniffer 硬件抓包
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		slog.Info("Sniffer 捕获协程正在启动...", "device", networkDevice)
		// 提示用户检查网卡状态
		slog.Info("💡 提示：如果长时期无数据，请确认网卡已通过 'iwconfig' 强转为 Monitor 模式并锁定了正确信道。")
		if err := sniffer.Start(ctx); err != nil {
			slog.Error("Sniffer 捕获异常中止", "error", err)
			os.Exit(1)
		}
	}()

	// 优雅关闭响应机制
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
