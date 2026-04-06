#!/bin/bash

# --- Kitty-Help Full Stack Deployment Script ---

# Ensure we are in the script's directory
cd "$(dirname "$0")"

echo "🚀 Starting Full Stack Update..."

# 0. Pre-flight Check: Ensure port 3000 is available (Aggressive)
echo "🔍 0/4: Force-clearing port 3000 connections..."
sudo fuser -k 3000/tcp 2>/dev/null || true
sleep 1

# 1. Update Backend
echo "📦 1/4: Rebuilding and restarting Backend Containers..."
docker compose down
docker compose up -d --build

# 2. Catch Cloudflare Tunnel URL
echo "🔍 2/4: Catching Tunnel URL (This might take a few seconds)..."
python catch_url.py

# 3. Build & Deploy Frontend
echo "🏗️ 3/4: Building Frontend for Production..."
npm run build

echo "☁️ 4/4: Deploying to Firebase Hosting..."
firebase deploy --only hosting

echo "✅ All done! Super Kitty is now LIVE on Firebase & NUC."
echo "🔗 Access it via: https://kitty-help.web.app"
