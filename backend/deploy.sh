#!/bin/bash
# 一键部署到树莓派（rsync 源码 + 现场编译）
# 用法: ./deploy.sh [target_host]
# 默认目标: rpi5.lan
set -e

TARGET="${1:-rpi5.lan}"
USER="${2:-pi}"
APP_DIR="/opt/remoteid"
BUILD_DIR="/tmp/remoteid-backend"
SVC="remoteid"

echo "📦 同步源码到 $TARGET ..."
rsync -avz --delete \
  --exclude '.DS_Store' \
  --exclude '*.db' \
  --exclude '*.log' \
  --exclude 'ridparse' \
  --exclude 'remoteid' \
  ./ "$USER@$TARGET:$BUILD_DIR/"

echo "🔨 在 $TARGET 上编译 ..."
ssh "$USER@$TARGET" bash -s << 'ENDSSH'
set -e
cd /tmp/remoteid-backend

echo "  → go build ..."
go build -ldflags="-s -w" -o remoteid ./cmd/remoteid/

echo "  → 停服 & 部署 ..."
sudo systemctl stop remoteid 2>/dev/null || true
sudo mkdir -p /opt/remoteid
sudo cp remoteid /opt/remoteid/
sudo cp config.yaml /opt/remoteid/
sudo chmod +x /opt/remoteid/remoteid

# 确保 systemd 服务存在
if ! sudo test -f /etc/systemd/system/remoteid.service; then
  echo "  → 创建 systemd 服务 ..."
  sudo tee /etc/systemd/system/remoteid.service > /dev/null <<SVC
[Unit]
Description=Remote ID Backend
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
SyslogIdentifier=remoteid

[Install]
WantedBy=multi-user.target
SVC
  sudo systemctl daemon-reload
  sudo systemctl enable remoteid
fi

echo "  → 启动服务 ..."
sudo systemctl start remoteid
ENDSSH

echo ""
echo "✅ 部署完成"
echo ""
echo "查看状态: ssh $USER@$TARGET 'sudo systemctl status $SVC'"
echo "查看日志: ssh $USER@$TARGET 'sudo journalctl -u $SVC -f'"
