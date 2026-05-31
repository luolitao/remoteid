#!/bin/bash
set -e

echo "🚀 构建 Remote ID 后端 (Go 版本)..."

# 1. 设置交叉编译
export GOOS=linux
export GOARCH=arm64
export GOARM=7

# 2. 构建
go build -ldflags="-s -w" -o remoteid cmd/remoteid/main.go

# 3. 优化二进制
upx --best --lzma remoteid 2>/dev/null || true

# 4. 安装
sudo mkdir -p /opt/remoteid
sudo cp remoteid /opt/remoteid/
sudo cp config.yaml /opt/remoteid/

# 5. 创建 systemd 服务
sudo tee /etc/systemd/system/remoteid.service > /dev/null <<EOF
[Unit]
Description=Remote ID Backend (Go)
After=network.target

[Service]
Type=simple
User=pi
Group=pi
WorkingDirectory=/opt/remoteid
ExecStart=/opt/remoteid/remoteid -config config.yaml
Restart=always
RestartSec=5
CapabilityBoundingSet=CAP_NET_RAW CAP_NET_ADMIN
AmbientCapabilities=CAP_NET_RAW CAP_NET_ADMIN
NoNewPrivileges=yes
StandardOutput=journal
StandardError=journal
SyslogIdentifier=remoteid-go

[Install]
WantedBy=multi-user.target
EOF

# 6. 重载 systemd
sudo systemctl daemon-reload
sudo systemctl enable remoteid

echo "✅ 构建完成! 使用 'sudo systemctl start remoteid' 启动服务"