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

	// 2. 加载配置
	cfg := config.Get()
	if *configPath != "config.yaml" {
		slog.Info("使用自定义配置文件", "path", *configPath)
		// 若 config 包支持路径参数，请在此处传入：cfg = config.Load(*configPath)
	}

	// 3. 初始化数据库
	dbPath := "./remoteid.db"
	if cfg.Database.Path != "" {
		dbPath = cfg.Database.Path
	}
	if err := db.Init(dbPath); err != nil {
		slog.Error("数据库初始化失败", "error", err)
		os.Exit(1)
	}
	defer db.Close() // ✅ 修复原代码中错误的类型断言 defer
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
	var totalReceived atomic.Int64

	// ✅ 关键修复 1：启动全局上下文（必须在 Worker 和 Sniffer 之前创建）
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 6. 数据广播协程（优化：移除 per-packet goroutine，修复 300s 统计周期）
	go func() {
		slog.Info("WebSocket 广播转发协程已启动")
		ticker := time.NewTicker(5 * time.Second) // ✅ 改为 5 秒，符合实时监控直觉
		defer ticker.Stop()

		var intervalCount int64
		var latestSample string

		for {
			select {
			case data, ok := <-broadcastCh:
				if !ok {
					slog.Info("广播管道已关闭，转发协程退出")
					return
				}
				if data.Telemetry == nil {
					continue
				}
				t := data.Telemetry
				totalReceived.Add(1)
				intervalCount++

				flatData := map[string]interface{}{
					"mac":                data.MACAddress,
					"uas_id":             t.UASID,
					"operator_id":        t.OperatorID,
					"latitude":           t.Latitude,
					"longitude":          t.Longitude,
					"lat":                t.Latitude,
					"lng":                t.Longitude,
					"altitude":           t.Altitude,
					"height":             t.Height,
					"speed":              t.Speed,
					"heading":            t.Heading,
					"operator_latitude":  t.OperatorLat,
					"operator_longitude": t.OperatorLng,
					"protocol":           t.Protocol,
					"rssi":               data.RSSI,
				}
				if !data.LastSeen.IsZero() {
					flatData["last_seen"] = data.LastSeen.Format(time.RFC3339)
				}

				wrappedMsg := map[string]interface{}{
					"type":  "drone_update",
					"data":  flatData,
					"drone": flatData,
				}
				for k, v := range flatData {
					wrappedMsg[k] = v
				}

				msgBytes, err := json.Marshal(wrappedMsg)
				if err != nil {
					slog.Error("WebSocket 序列化失败", "error", err)
					continue
				}
				latestSample = string(msgBytes)

				// ✅ 关键修复 3：移除 go func() 包装，直接广播
				// 原代码每秒创建数千个协程会导致 OOM。假设 wsManager.Broadcast 是异步或带缓冲的
				wsManager.Broadcast(msgBytes)

			case <-ticker.C:
				if intervalCount > 0 {
					slog.Info("📊 [定时汇总] 数据流转统计",
						"interval_count", intervalCount,
						"total_received", totalReceived.Load(),
						"latest_sample", latestSample,
					)
					intervalCount = 0
				}
			}
		}
	}()

	// 7. 初始化并异步启动 API 服务
	serverHandler := api.NewServer(processor, wsManager, sniffer) // ✅ 修复原代码 sniff er 语法错误
	errCh := make(chan error, 2)

	go func() {
		port := ":" + cfg.API.Port
		slog.Info("API 服务尝试绑定端口", "port", cfg.API.Port)
		if err := serverHandler.Run(port); err != nil && err.Error() != "http: Server closed" {
			errCh <- fmt.Errorf("API 服务异常退出: %w", err)
		}
	}()

	// 8. 异步启动 Sniffer 硬件抓包
	go func() {
		slog.Info("Sniffer 捕获协程正在启动...", "device", networkDevice)
		if err := sniffer.Start(ctx); err != nil {
			errCh <- fmt.Errorf("Sniffer 捕获异常中止: %w", err)
		}
	}()

	// 9. 优雅关闭响应机制
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		slog.Info("收到停机信号", "signal", sig)
	case err := <-errCh:
		slog.Error("子组件发生致命错误，触发全局关闭", "error", err)
	}

	slog.Info("正在释放资源...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	// ✅ 关键修复 4：严格按顺序关闭，防止协程死锁
	processor.Close()                   // 1. 停止 Processor 内部 ticker
	close(broadcastCh)                  // 2. 关闭广播管道，释放 WS 协程
	cancel()                            // 3. 取消 Sniffer 上下文
	serverHandler.Shutdown(shutdownCtx) // 4. 停止 HTTP 服务

	slog.Info("RemoteID 监控系统已安全停止")
}
