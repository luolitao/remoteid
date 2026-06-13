package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
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
	_ = configPath // 预留：若 config 包支持动态路径可在此传入
	cfg := config.Get()

	// 3. 初始化数据库
	dbPath := "./remoteid.db"
	if cfg.Database.Path != "" {
		dbPath = cfg.Database.Path
	}
	if err := db.Init(dbPath); err != nil {
		slog.Error("数据库初始化失败", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	slog.Info("数据库初始化成功", "path", dbPath)

	// 4. 核心管道与组件装配
	broadcastCh := make(chan *drone.TrackedDrone, 2048)
	processor := drone.NewProcessor(broadcastCh)

	// 5. 网卡优先级抉择
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

	// 6. 数据广播协程（优化：5秒统计 + 扁平化 JSON + 航向归一化）
	go func() {
		slog.Info("WebSocket 广播转发协程已启动")
		ticker := time.NewTicker(5 * time.Second) // ✅ 改为 5 秒实时监控
		defer ticker.Stop()

		var totalReceived atomic.Int64
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

				// 🎯 构建完全扁平化的数据对象，兼容前端各种字段名
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
					"heading":            t.Heading % 360, // ✅ 航向角归一化 (0~359)
					"operator_latitude":  t.OperatorLat,
					"operator_longitude": t.OperatorLng,
					"protocol":           t.Protocol,
					"rssi":               data.RSSI,
				}
				if !data.LastSeen.IsZero() {
					flatData["last_seen"] = data.LastSeen.Format(time.RFC3339)
				}

				// 🎯 构建最终消息：提供 3 层访问路径防护
				wrappedMsg := map[string]interface{}{
					"type":  "drone_update",
					"data":  flatData, // 兼容: const { latitude } = msg.data
					"drone": flatData, // 兼容: const { latitude } = msg.drone
				}
				for k, v := range flatData {
					wrappedMsg[k] = v // 兼容: const { latitude } = msg
				}

				msgBytes, err := json.Marshal(wrappedMsg)
				if err != nil {
					slog.Error("WebSocket 序列化失败", "error", err)
					continue
				}
				latestSample = string(msgBytes)
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
	serverHandler := api.NewServer(processor, wsManager, sniffer)
	errCh := make(chan error, 2)

	go func() {
		port := ":" + cfg.API.Port
		slog.Info("API 服务尝试绑定端口", "port", cfg.API.Port)
		if err := serverHandler.Run(port); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("API 服务异常退出: %w", err)
		}
	}()

	// 8. 异步启动 Sniffer 硬件抓包
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		slog.Info("Sniffer 捕获协程正在启动...", "device", networkDevice)
		slog.Info("💡 提示：如果长期无数据，请确认网卡已切换至 Monitor 模式并锁定正确信道。")
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

	// ✅ 严格按顺序关闭，防止协程死锁或数据丢失
	cancel()           // 1. 取消 Sniffer 上下文
	processor.Close()  // 2. 停止 Processor 内部清理 ticker
	close(broadcastCh) // 3. 关闭广播管道，释放 WS 协程
	if err := serverHandler.Shutdown(shutdownCtx); err != nil {
		slog.Error("服务优雅关闭失败", "error", err)
	}

	slog.Info("RemoteID 监控系统已安全停止")
}
