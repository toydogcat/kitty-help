# 🤖 AI Notice: Project Context & Architecture

This document tracks critical architectural decisions and "gotchas" for future AI agents or developers working on Kitty-Help.

## 🏗️ Architecture Overview
- **Frontend**: Single Page Application (SPA) hosted on **Firebase Hosting** (`kitty-help.web.app`).
- **Backend**: Containerized Node.js/Express server (`kitty-help-backend`) running on a local Linux machine via **Docker Compose**.
- **Bridge**: **Cloudflare Tunnel** (`cloudflared`) connects the local backend to the public internet securely, updating the tunnel URL dynamically.

## ⚙️ Environment Configuration
- `.env`: Contains sensitive database credentials.
- `.env.production`: Contains the dynamically updated `VITE_API_URL`. This file is generated/updated by `catch_url.py` and used during `npm run build`.
- **Note**: After any backend restart, the tunnel URL may change. The `run.sh` script automates the update of the frontend to point to the new URL.

## 💾 Database Schema (PostgreSQL)
- `users`: Core user table with `role` (admin, subadmin, user). **Toby** is forced as `admin` on startup.
- `devices`: Tracks browser sessions. Role-based access depends on the `user_id` linked to the device.
- `calendar_events`: Synchronized family notes. Unique constraint on `(user_id, event_date)`.
- `common_text_history`: FIFO history (10 items) for the shared clipboard.

## ⚠️ Known Gotchas
1. **Old URL Cache**: If the frontend shows `404` errors after a deployment, it's likely because the browser is still using the previous tunnel URL. **RELOAD the page** to fetch the new build.
2. **Network Mode**: The backend uses `network_mode: host` to avoid complex DNS resolution between containers and the local PostgreSQL/Cloudflare tunnel.
3. **Column Additions**: When adding columns to existing tables, use `ALTER TABLE ... ADD COLUMN IF NOT EXISTS` in `server/index.ts` to ensure users with existing databases aren't broken.
4. **CORS**: Aggressive CORS headers are implemented in `server/index.ts` to allow cross-origin requests from the dynamic tunnel URLs.

## 🛠️ Maintenance Commands
- `sh run.sh`: Fully automates the whole stack update.
- `docker logs kitty-help-backend`: Check backend health.
- `lsof -i :3000`: Check for port conflicts (though `run.sh` handles this).
