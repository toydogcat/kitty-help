#!/bin/bash

# --- Kitty-Help Full Stack Deployment Script ---

# Ensure we are in the script's directory
cd "$(dirname "$0")"

echo "🚀 Starting Full Stack Update..."

# 0. Pre-flight Check: Ensure port 3000 is available
echo "🔍 0/4: Checking for port conflicts on 3000..."
CONFLICT_PID=$(lsof -t -i:3000)
if [ ! -z "$CONFLICT_PID" ]; then
  echo "⚠️  Port 3000 is occupied by PID $CONFLICT_PID. Killing it..."
  kill -9 $CONFLICT_PID
fi

# 1. Update Backend
echo "📦 1/4: Rebuilding and restarting Backend Containers..."
docker compose down
docker compose up -d --build

# 2. Catch Cloudflare Tunnel URL
echo "🔍 2/4: Catching Tunnel URL (This might take a few seconds)..."
python catch_url.py

# 3. Build Frontend
echo "🏗️ 3/4: Building Frontend for Production..."
npm run build

# 4. Deploy to Firebase
echo "🚀 4/4: Deploying to Firebase Hosting..."
firebase deploy --only hosting

echo "✅ All done! Your site is live and the backend is synchronized."
