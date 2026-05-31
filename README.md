# Remote ID — 无人机远程识别监控系统

基于 WiFi 抓包的无人机 Remote ID 监控系统，支持 **ASTM F3411-22a / ASD-STAN**（国际标准）和 **GB42590-2023**（中国国家标准 C-RID）双协议。部署于树莓派，通过 2.4GHz WiFi 监控模式实时捕获无人机 Beacon 帧中 Vendor Specific IE 的身份和位置信息。

## 功能特性

- **双协议支持**：同时解析 ASTM F3411-22a 和 GB42590-2023 标准
- **实时监控**：WebSocket 推送无人机位置、身份、高度、速度等数据
- **地图可视化**：Leaflet 地图实时展示无人机位置和飞行轨迹
- **轨迹回放**：支持历史轨迹查询与回放
- **告警管理**：自定义告警规则，支持创建/解决/统计
- **数据导出**：支持 JSON / CSV 格式导出

## 项目结构

```
remoteid/
├── backend/            # Go 后端（主力）
│   ├── cmd/remoteid/    # 入口
│   ├── internal/        # 内部包（api/config/db/drone）
│   ├── pkg/             # 公共库（types/ws）
│   └── build.sh         # 交叉编译脚本
├── frontend/            # Vue 3 前端
│   └── src/
├── tools/               # 辅助工具
│   └── spoofer/         # 无人机信号模拟器
├── deploy/              # 部署配置
├── legacy/              # 历史代码归档
├── .github/workflows/   # CI/CD
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
go run cmd/remoteid/main.go
```

或编译部署到树莓派：

```bash
cd backend
bash build.sh
```

### 前端启动

```bash
cd frontend
npm install
npm run dev
```

### Docker 部署（推荐）

```bash
docker-compose up -d
```

## 配置说明

配置文件位于 `backend/config.yaml`：

```yaml
database:
  path: /data/remoteid.db
network:
  interface: wlan1       # WiFi 监控接口
  channel: 6             # 监听信道（1-165）
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
- `REMOTEID_DB_PATH` — 数据库路径
- `REMOTEID_IFACE` — 网络接口
- `REMOTEID_PORT` — API 端口
- `REMOTEID_DEBUG` — 调试模式

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

> 注：ASTM Beacon 和 GB42590 使用相同的 Vendor IE OUI (`FA:0B:BC` + `0x0D`)，
> 两者经纬度编码相同（4 字节 int32 LE），通过协议版本号（ASTM=2, GB42590=0/1）和消息字段布局差异区分标准。

### GB42590-2023 (C-RID)
- **OUI**: `FA:0B:BC`，**Vendor Type**: `0x0D`
- 支持 Packed 格式（`0xF1`，高 4 位 = 0xF 表示消息包）
- Packed 头部：MessageCounter(1B) + MsgType|Proto(1B) + SingleMsgSize(1B) + MsgCount(1B)
- 编码特点：经纬度均为 4 字节 int32 little-endian
- Location 消息字段：Status(4bit) + Direction(12bit split) + SpeedH(1B) + SpeedV(1B) + Lat(4B LE) + Lon(4B LE) + Alt(2B LE)
- Basic ID 字段：ID Type(3bit high) + UA Type(4bit low)，与 ASTM 的 nibble 分配相反

## 许可证

MIT
