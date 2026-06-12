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
	"remoteid-monitor/internal/config" // 💡 保持你原有的 config 包导入
	"remoteid-monitor/internal/db"
	"remoteid-monitor/internal/drone"
	"remoteid-monitor/pkg/ws"
)

func init() {
	// 保持原有的 JSON 结构化高性能日志配置
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
	// 1. 解析基础命令行参数（保留原有的配置文件注入方式）
	configPath := flag.String("config", "config.json", "Path to configuration file")
	flag.Parse()

	slog.Info("正在启动 Remote ID 监视后端系统...")

	// 2. 💡 对齐原版配置接口：调用你原有的初始化方法
	// 如果你原版的 config 包有特定的 Init 方法（例如 config.Init(*configPath)），请取消下行注释：
	// config.Init(*configPath)

	// 从你截图中的 `config.Get().API.Port` 可以看出，你的配置是通过 Get() 单例获取的
	// 这里我们直接通过 `config.Get()` 提取所需的全局变量
	globalCfg := config.Get()
	if globalCfg == nil {
		slog.Error("全局配置单例获取失败，请检查配置文件是否存在或格式是否正确")
		os.Exit(1)
	}

	// 3. 核心级安全链：接管全局退出信号 (Ctrl+C / kill / SIGTERM)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 4. 初始化持久化高速 SQLite WAL 连接池
	// 💡 这里的 DBPath 请根据你实际的 config 结构体字段进行替换（例如 globalCfg.DB.Path 或 globalCfg.DatabasePath）
	if err := db.Init(globalCfg.DBPath); err != nil {
		slog.Error("初始化数据库失败", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := db.Close(); err != nil {
			slog.Error("关闭数据库时发生异常", "error", err)
		}
	}()

	// 5. 构建中央高并发异步通信总线 (WebSocket Hub)
	wsHub := ws.NewHub()
	go wsHub.Run(ctx)

	// 6. 实例化归一化遥测状态机中央处理器 (Processor)
	broadcastCh := make(chan *drone.TrackedDrone, 1000)
	processor := drone.NewProcessor(broadcastCh)
	defer processor.Close()

	// 7. 启动桥接消费协程：将解包后的标准数据，同步灌入数据库与前端广播通道
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case data, ok := <-broadcastCh:
				if !ok {
					return
				}
				_ = db.SaveDrone(data.MACAddress, data.Telemetry.UASID, data.Telemetry.Protocol)
				_ = db.SavePosition(data.MACAddress, data.Telemetry.Latitude, data.Telemetry.Longitude, data.Telemetry.Altitude)
				wsHub.Broadcast(data)
			}
		}
	}()

	// 8. 挂载物理网卡驱动级核心嗅探器 (Sniffer)
	// 💡 这里的 Interface 请根据你实际的 config 字段替换（例如 globalCfg.Network.Interface）
	sniffer := drone.NewSniffer(globalCfg.Interface, processor)
	go func() {
		if err := sniffer.Start(ctx); err != nil {
			slog.Error("底层网卡嗅探引擎异常中断", "error", err)
			stop()
		}
	}()

	// 9. 启动 Web API 服务网关
	// 💡 恢复你原版的路由挂载：根据截图，你原版使用的是 api.NewServer
	// 如果你已经重构了 api 包，可以用这行：router := api.SetupRouter(wsHub)
	// 如果没有重构 api 包，则使用你原有的方式，把新的组件传入：
	server := api.NewServer(processor, wsHub, sniffer)

	// 💡 动态拼装服务器地址，完全对齐你原本的 `":" + config.Get().API.Port` 逻辑
	serverAddr := ":" + globalCfg.API.Port
	srv := &http.Server{
		Addr:    serverAddr,
		Handler: server, // 如果 NewServer 返回的是 http.Handler 或者符合 gin.Engine 的路由
	}

	go func() {
		slog.Info("Web 接口及 WebSocket 监听通道已就绪", "address", serverAddr)
		// 💡 改用标准库的 srv.ListenAndServe() 配合下方的优雅关闭
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP 网关异常崩溃", "error", err)
			stop()
		}
	}()

	// 10. ⚡ 优雅收尾哨兵：阻塞在这里，直到系统收到退出信号
	<-ctx.Done()
	slog.Info("接收到停机指令，启动优雅退出（Graceful Shutdown）机制...")

	// 给外围残存连接或慢 I/O 留出最多 5 秒的喘息时间
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("HTTP 服务器强制关闭", "error", err)
	}

	slog.Info("后端服务已安全离线，所有文件句柄与硬件网卡已正常归还系统。")
}
