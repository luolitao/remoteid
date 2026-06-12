package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"remoteid-monitor/internal/api"
	"remoteid-monitor/internal/config"
	"remoteid-monitor/internal/db"
	"remoteid-monitor/internal/drone"
	"remoteid-monitor/pkg/ws"
)

func init() {
	// 💡 优化 1：保留毫秒级时间戳，方便高频数据排查
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
	// 1. 注册命令行参数
	configPath := flag.String("config", "config.yaml", "配置文件路径")
	ifaceFlag := flag.String("iface", "", "指定网络监听网卡接口 (例如 wlan0, wlan2)")
	flag.Parse()

	slog.Info("RemoteID 监控系统正在启动...")

	// 💡 优化 2：如果 config 包支持，这里应该把 configPath 传进去
	// 假设 config 包目前只能读默认路径，这里仅作占位
	_ = configPath
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
	// 💡 优化 3：确保退出时关闭数据库 (假设 db 包有 Close 方法)
	defer func() {
		if closer, ok := db.Close().(interface{ Close() error }); ok {
			closer.Close()
		}
	}()
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

	// 💡 优化 4：使用 atomic 保证并发安全，且方便外部读取
	var totalReceived atomic.Int64

	// 6. 处理数据广播
	go func() {
		slog.Info("WebSocket 广播转发协程已启动")

		// ⏱️ 设定汇总周期（例如每 5 秒汇总一次，可根据需要调整）
		ticker := time.NewTicker(300 * time.Second)
		defer ticker.Stop()

		var intervalCount int64 // 当前时间窗口内的计数
		var latestSample string // 记录最新的一条 JSON，用于抽样展示

		for {
			select {
			case data, ok := <-broadcastCh:
				if !ok {
					slog.Info("广播管道已关闭，转发协程退出")
					return
				}

				totalReceived.Add(1)
				intervalCount++ // 窗口内计数 +1

				msgBytes, err := json.Marshal(data)
				if err != nil {
					slog.Error("WebSocket 序列化失败", "error", err)
					continue
				}

				latestSample = string(msgBytes) // 更新最新样本
				wsManager.Broadcast(msgBytes)   // 100% 广播给前端

			case <-ticker.C:
				// ⏱️ 定时触发汇总
				if intervalCount > 0 { // 只有当这段时间内有数据时才打印，避免空闲刷屏
					slog.Info("📊 [定时汇总] 数据流转统计",
						"interval_count", intervalCount, // 本周期（如5秒内）新增数量
						"totalReceived", totalReceived.Load(), // 全局累计总数
						"latest_sample", latestSample, // 抽样展示最新的一条完整 JSON
					)
					intervalCount = 0 // 重置周期计数，开始下一个窗口
				}
			}
		}
	}()

	// 7. 初始化并异步启动 API 服务
	serverHandler := api.NewServer(processor, wsManager, sniffer)

	// 💡 优化 5：创建一个全局的 errorCh，用于子协程向主协程报告致命错误
	errCh := make(chan error, 2)

	go func() {
		port := ":" + cfg.API.Port
		slog.Info("API 服务尝试绑定端口", "port", cfg.API.Port)
		if err := serverHandler.Run(port); err != nil && err.Error() != "http: Server closed" {
			errCh <- fmt.Errorf("API 服务异常退出: %w", err)
		}
	}()

	// 8. 异步启动 Sniffer 硬件抓包
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		slog.Info("Sniffer 捕获协程正在启动...", "device", networkDevice)
		if err := sniffer.Start(ctx); err != nil {
			errCh <- fmt.Errorf("Sniffer 捕获异常中止: %w", err)
		}
	}()

	// 9. 优雅关闭响应机制
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// 💡 优化 6：使用 select 同时监听系统信号和子协程的致命错误
	select {
	case sig := <-quit:
		slog.Info("收到停机信号", "signal", sig)
	case err := <-errCh:
		slog.Error("子组件发生致命错误，触发全局关闭", "error", err)
	}

	slog.Info("正在释放资源...")

	// 设定 5 秒优雅关闭缓冲区
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	// 停止 Sniffer
	cancel()

	// 停止 Processor (关闭 broadcastCh)
	processor.Close()

	// 停止 API 服务
	if err := serverHandler.Shutdown(shutdownCtx); err != nil {
		slog.Error("API 服务优雅关闭失败", "error", err)
	}

	slog.Info("RemoteID 监控系统已安全停止")
}
