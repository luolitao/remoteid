#!/bin/bash
set -e

echo "🚀 构建 Remote ID Monitor 后端 (Go 版本)..."

# 1. 设置交叉编译
export GOOS=linux
export GOARCH=arm64
export GOARM=7

# 2. 构建
go build -ldflags="-s -w" -o remoteid-monitor ./cmd/remoteid/

# 3. 优化二进制
upx --best --lzma remoteid-monitor 2>/dev/null || true

# 4. 安装
sudo mkdir -p /opt/remoteid-monitor
sudo cp remoteid-monitor /opt/remoteid-monitor/
sudo cp config.yaml /opt/remoteid-monitor/

# 5. 创建 systemd 服务
sudo tee /etc/systemd/system/remoteid-monitor.service > /dev/null <<EOF
[Unit]
Description=Remote ID Monitor Backend (Go)
After=network.target

[Service]
Type=simple
User=pi
Group=pi
WorkingDirectory=/opt/remoteid-monitor
ExecStart=/opt/remoteid-monitor/remoteid-monitor -config config.yaml
Restart=always
RestartSec=5
CapabilityBoundingSet=CAP_NET_RAW CAP_NET_ADMIN
AmbientCapabilities=CAP_NET_RAW CAP_NET_ADMIN
NoNewPrivileges=yes
StandardOutput=journal
StandardError=journal
SyslogIdentifier=remoteid-monitor

[Install]
WantedBy=multi-user.target
EOF

# 6. 重载 systemd
sudo systemctl daemon-reload
sudo systemctl enable remoteid-monitor

echo "✅ 构建完成! 使用 'sudo systemctl start remoteid-monitor' 启动服务"