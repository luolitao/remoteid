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

- 后端：Go，使用 gopacket 抓包解析 WiFi Remote ID（ASTM F3411-22a / ASD-STAN），涵盖 WiFi Beacon 和 NAN Service Discovery Frame
- 前端：Vue
- 部署：systemd 管理服务，`/opt/remoteid-monitor/`
