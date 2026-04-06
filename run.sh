#!/bin/bash

# --- Kitty-Help Full Stack Deployment Script (Turbo) ---

# Ensure we are in the script's directory
cd "$(dirname "$0")"

echo "🚀 Starting Full Stack Update..."

# 0. 清除舊的連線
echo "🔍 0/4: Force-clearing port 3000 connections..."
sudo fuser -k 3000/tcp 2>/dev/null || true
sleep 1

# 1. 重啟後端容器
echo "📦 1/4: Rebuilding and restarting Backend Containers..."
docker compose down
docker compose up -d --build

# 2. 捕捉最新的 Tunnel 網址
echo "🔍 2/4: Catching Tunnel URL via run.py..."
./run.py -c

# 3. 強制刷新環境變數並打包
echo "🏗️ 3/4: Building Frontend with NEW URL..."
# 我們在這裡再 confirm 一次環境變數
npm run build

# 4. 部署到 Firebase
echo "☁️ 4/4: Deploying to Firebase Hosting..."
firebase deploy --only hosting

echo "✅ [SUCCESS] 全系統已同步更新！"
echo "🔗 請使用最新的網址開啟網頁測試。"
