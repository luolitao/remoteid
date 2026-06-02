# Remote ID Monitor — 无人机远程识别监控系统

基于 WiFi 抓包的无人机 Remote ID 监控系统，支持 **ASTM F3411-22a / ASD-STAN**（国际标准）协议。部署于树莓派，通过 2.4GHz WiFi 监控模式实时捕获无人机 Beacon 帧和 NAN Service Discovery Frame 中 Vendor Specific IE 的身份和位置信息。

## 功能特性

- **ASTM/ASD-STAN 协议支持**：解析 ASTM F3411-22a / ASD-STAN prEN 4709-002 标准
- **实时监控**：WebSocket 推送无人机位置、身份、高度、速度等数据
- **TUI 命令行工具**：终端内卡片式展示无人机实时状态，支持本地开发机远程连接
- **离线分析**：支持 pcap/pcapng 文件离线解析
- **地图可视化**：Leaflet 地图实时展示无人机位置和飞行轨迹
- **轨迹回放**：支持历史轨迹查询与回放
- **告警管理**：自定义告警规则，支持创建/解决/统计
- **数据导出**：支持 JSON / CSV 格式导出

## 项目结构

```
remoteid-monitor/
├── backend/                # Go 后端
│   ├── cmd/
│   │   ├── remoteid/       # 后端服务入口（API + 抓包）
│   │   └── ridparse/       # TUI / 离线分析命令行工具
│   ├── internal/           # 内部包（api / config / db / drone）
│   ├── pkg/                # 公共库（types / ws）
│   ├── build.sh            # 树莓派本地编译脚本
│   ├── deploy.sh           # 一键部署脚本（rsync + 远程编译）
│   └── config.yaml         # 后端配置文件
├── frontend/               # Vue 3 前端
├── tools/
│   └── spoofer/            # 无人机信号模拟器
├── deploy/                 # Docker 部署配置
├── legacy/                 # 历史代码归档
├── .github/workflows/      # CI/CD
└── README.md
```

## 技术栈

| 层级 | 技术 |
|------|------|
| **后端** | Go 1.25 + Gin + gopacket/pcap |
| **前端** | Vue 3 + Pinia + Vue Router + Leaflet + Tailwind CSS |
| **数据库** | SQLite（WAL 模式） |
| **通信** | REST API + WebSocket |
| **部署** | 树莓派 + systemd / Docker |

## 快速开始

### 环境要求

- Go 1.25+
- Node.js 20+
- libpcap（用于 WiFi 抓包）
- 支持监控模式的 WiFi 网卡

### 后端启动

```bash
cd backend
cp config.yaml.example config.yaml  # 按需修改配置
go run ./cmd/remoteid/
```

### 前端启动

```bash
cd frontend
npm install
npm run dev
```

### TUI 命令行工具

```bash
# 编译
cd backend && go build -o ridparse ./cmd/ridparse/

# 实时抓包
./ridparse -iface wlan1

# 离线分析 pcap 文件
./ridparse -file capture.pcap

# TUI 监控（连接后端服务）
./ridparse -tui                            # 连接本地后端
./ridparse -tui -url http://rpi3.lan:8000  # 连接树莓派 rpi3
./ridparse -tui -url http://rpi5.lan:8000  # 连接树莓派 rpi5
```

TUI 模式通过 WebSocket 实时接收推送（毫秒级更新），HTTP API 5 秒全量拉取兜底，自动重连。

### 部署到树莓派

项目后端部署在 2 台树莓派（`rpi3.lan` / `rpi5.lan`），使用 rsync 源码 + 现场编译方案：

```bash
cd backend
./deploy.sh rpi3.lan    # 部署到 rpi3
./deploy.sh rpi5.lan    # 部署到 rpi5
```

脚本自动完成：rsync 源码 → SSH 远程 `go build` → 停服 → 部署到 `/opt/remoteid-monitor/` → 启服。首次部署自动创建 systemd 服务。

**前置条件**：树莓派需安装 `golang` 和 `libpcap-dev`。

### Docker 部署

```bash
docker-compose up -d
```

## 配置说明

配置文件位于 `backend/config.yaml`：

```yaml
database:
  path: /data/remoteid-monitor.db
network:
  interface: wlan2         # WiFi 监控接口
  channel: 6               # 监听信道（1-165）
api:
  port: 8000
  cors_origins:
    - http://localhost:5173
logging:
  level: info
debug:
  enabled: false
```

支持环境变量覆盖：
- `REMOTEID_MONITOR_DB_PATH` — 数据库路径
- `REMOTEID_MONITOR_IFACE` — 网络接口
- `REMOTEID_MONITOR_PORT` — API 端口
- `REMOTEID_MONITOR_DEBUG` — 调试模式

## API 概览

| 端点 | 说明 |
|------|------|
| `GET /health` | 健康检查 |
| `GET /api/drones` | 活跃无人机列表 |
| `GET /api/drones/:mac` | 无人机详情 |
| `GET /api/drones/:mac/trajectory` | 飞行轨迹 |
| `GET /api/alerts` | 告警列表 |
| `POST /api/alerts` | 创建告警 |
| `PUT /api/alerts/:id/resolve` | 解决告警 |
| `GET /api/system` | 系统信息 |
| `WS /ws` | WebSocket 实时推送 |

## 协议说明

### ASTM F3411-22a / ASD-STAN prEN 4709-002

ASTM/ASD-STAN 通过 **Vendor Specific IE** (IEEE 802.11 Element ID `0xDD`) 承载 Remote ID 数据：

- **OUI**: `FA:0B:BC` (ASD-STAN)，**OUI_Type**: `0x0D`
- 协议版本：`2`（字节 0 高 4 位）
- 消息类型：Basic ID / Location / Authentication / Self ID / System / Operator ID（类型 0-5）
- 编码特点：经纬度均为 4 字节 int32 little-endian，缩放因子 1e-7
- 每条消息固定 25 字节（1 字节 header + 24 字节 payload）
- Location 消息字段：Status(4bit) + Direction(1B) + SpeedH(1B) + SpeedV(1B) + Lat(4B LE) + Lon(4B LE) + AltBaro(2B LE) + AltGeo(2B LE) + Height(2B LE) + Accuracy(3B) + Timestamp(2B LE)

### NAN Service Discovery

## 许可证

MIT
