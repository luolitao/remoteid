# Remote ID Monitor — 无人机远程识别监控系统

基于 WiFi 抓包的无人机 Remote ID 监控系统，支持 **ASTM F3411-22a / ASD-STAN**（国际标准）、**GB 42590-2023**（中国国标）和 **GB 46750-2023**（中国国标变长格式）三种协议。部署于树莓派，通过 2.4GHz WiFi 监控模式实时捕获无人机 Beacon 帧和 NAN Service Discovery Frame 中 Vendor Specific IE 的身份和位置信息。

## 功能特性

- **多协议支持**：ASTM F3411-22a / ASD-STAN prEN 4709-002 + GB 42590-2023 + GB 46750-2023
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

三种协议均使用相同的 **Vendor Specific IE** (IEEE 802.11 Element ID `0xDD`)，通过 **OUI** `FA:0B:BC` + **OUI_Type** `0x0D` 承载 Remote ID 数据，自动区分：

| 协议 | 识别方式 | 消息格式 |
|------|---------|---------|
| **ASTM F3411-22a** | Header 低4位=`0x2` | 25 字节固定消息（5 种类型） |
| **GB 42590-2023** | Header 低4位=`0x1` | 25 字节固定消息（方向 12 位编码） |
| **GB 46750-2023** | `data[1]==0xFF` | 变长自定义格式（21 项数据标识位表） |

### ASTM F3411-22a / ASD-STAN prEN 4709-002

- 协议版本：`2`（Header 字节低 4 位）
- 消息类型：Basic ID / Location / Authentication / Self ID / System / Operator ID（类型 0-5）
- 经纬度：4 字节 int32 LE × 1e-7
- Location 消息：Status(4bit) + Direction(1B, 0-180° + EW 标志) + SpeedH(1B) + SpeedV(1B) + Lat(4B LE) + Lon(4B LE) + AltBaro(2B LE) + AltGeo(2B LE) + Height(2B LE) + Accuracy(3B) + Timestamp(2B LE)

### GB 42590-2023

- 协议版本：`1`（Header 字节低 4 位），与 ASTM 共用 OUI
- 方向编码：12 位（高 4 位在 flags 低 4 位 + 低 8 位单独字节）× 360/65535
- 无效值定义：`0xFFFF`=无效方向，`0x7FFFFFFF`=无效坐标，`0xFFFF`=无效高度
- 支持 Packed 格式（消息类型 `0xF`）

### GB 46750-2023

- 识别魔数：`data[1] == 0xFF`，版本号高 3 位=`0x1`
- Wire 格式：`[MsgCounter:1B] [0xFF:1B] [版本:1B] [数据长度:1B] [数据标识:3B] [数据内容:变长]`
- 数据标识位表：3 字节 × 7 位 = 21 个数据项（001-021），按需组合
- 高度编码：`(raw/2.0) - 1000.0`（大地/气压/遥控站）或 `(raw/2.0) - 9000.0`（相对高度）
- 方向编码：2 字节 uint16 LE × 0.1°
- 时间戳：6 字节 LE Unix 毫秒

### NAN Service Discovery

通过 Action Frame (subtype 13) 承载，Wi-Fi Alliance OUI `50:6F:9A` 内嵌 Remote ID 数据。

## 许可证

MIT
