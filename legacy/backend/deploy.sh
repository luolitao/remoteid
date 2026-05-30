#!/bin/bash
# deploy.sh - 部署前后端分离应用到树莓派

echo "🚀 Starting deployment to Raspberry Pi"

# 1. 安装系统依赖
sudo apt update
sudo apt install -y nginx python3-pip sqlite3

# 2. 创建数据目录
sudo mkdir -p /data/remoteid
sudo mkdir -p /data/pcaps
sudo chown -R $USER:$USER /data

# 3. 部署后端
echo "📦 Deploying backend..."
cd backend
pip3 install -r requirements.txt --user
# 创建 systemd 服务
sudo tee /etc/systemd/system/remoteid-backend.service > /dev/null <<EOF
[Unit]
Description=Remote ID Backend API
After=network.target

[Service]
Type=simple
User=$USER
WorkingDirectory=$(pwd)
ExecStart=/usr/bin/python3 main.py
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable remoteid-backend
sudo systemctl restart remoteid-backend

# 4. 部署前端
echo "📦 Deploying frontend..."
cd ../frontend
npm install
npm run build

# 配置 Nginx
sudo tee /etc/nginx/sites-available/remoteid > /dev/null <<EOF
server {
    listen 80;
    server_name _;
    
    root /home/$USER/projects/remoteid/frontend/dist;
    index index.html;
    
    location / {
        try_files \$uri \$uri/ /index.html;
    }
    
    location /api/ {
        proxy_pass http://localhost:8000/;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
    }
    
    location /ws/ {
        proxy_pass http://localhost:8000/ws;
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
EOF

sudo ln -sf /etc/nginx/sites-available/remoteid /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl restart nginx

# 5. 设置监控
echo "✅ Deployment completed!"
echo ""
echo "🌐 Access the application at: http://raspberrypi.local"
echo "🔧 Backend API: http://raspberrypi.local/api/drones"
echo "📊 Nginx logs: sudo tail -f /var/log/nginx/error.log"
echo "🐍 Backend logs: journalctl -u remoteid-backend -f"
