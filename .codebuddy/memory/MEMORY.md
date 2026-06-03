# 项目长期记忆

## 项目名称

- 项目名：`remoteid-monitor`
- Go module：`remoteid-monitor`

## 部署信息

- 后端部署在 2 台树莓派：
  - `rpi3.lan`
  - `rpi5.lan`
- 部署方式：rsync 源码 + 树莓派现场编译（`backend/deploy.sh`）
- 前置条件：树莓派需安装 `golang` 和 `libpcap-dev`
- 部署路径：`/opt/remoteid-monitor/`
- systemd 服务名：`remoteid-monitor`

## 项目结构

- `backend/` — Go 后端（主力）
- `frontend/` — Vue 前端
- `legacy/` — 旧版代码
- `deploy/` — Docker 部署配置
- `tools/` — 工具脚本

## 技术栈

- 后端：Go，使用 gopacket 抓包解析 WiFi Remote ID（ASTM F3411-22a / ASD-STAN + GB 42590-2023），涵盖 WiFi Beacon 和 NAN Service Discovery Frame
- 前端：Vue
- 部署：systemd 管理服务，`/opt/remoteid-monitor/`

## 协议关键知识点

### ASTM F3411-22a 与 GB 42590-2023 区分

两者使用相同的 OUI（`FA:0B:BC` + `0x0D`），通过消息 Header 字节的低 4 位区分：
- 低 4 位 = `0x1` → GB 42590-2023
- 低 4 位 = `0x2` → ASTM F3411-22a

### GB 与 ASTM 的关键差异

1. **方向编码**：GB 用 12 位（高 4 位在 flags 低 4 位 + 低 8 位单独字节）× 360/65535；ASTM 用 8 位（0-180）+ EW 方向标志位
2. **无效值**：GB 有明确的无效值定义（0xFFFF=无效方向, 0x7FFFFFFF=无效坐标, 0xFFFF=无效高度）
3. **字段偏移**：GB 和 ASTM 的 Basic ID / Location / System 消息的坐标/高度等字段偏移位置相同
4. **Packed 格式**：GB 支持消息类型 0xF 的 Packed 格式，ASTM 不支持

### 高度合理性检查

processor.go 中增加了高度合理性检查：如果 altitude > 5000m 或 < -500m，标记为异常并清零。因为无人机限高通常不超过 500 米，模拟器信号不超过 120 米。
