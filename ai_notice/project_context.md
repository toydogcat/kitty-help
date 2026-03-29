# 🐱 Kitty-Help 專案背景與架構紀錄 (Project Context)

本文件用於記錄 Kitty-Help 的核心架構演進與關鍵決策，供未來 AI 續寫代碼或維護時參考。

## 1. 專案演進里程碑
- **Phase 1-2**: 初始原型，使用 Firebase Firestore。
- **Phase 3**: 導入 **個人階層式剪貼簿 (Tree View)**，支援兩欄式佈局與無限層級資料夾。
- **Phase 3.5**: 新增 **生產力倒數計時器**，整合 Web Notification 與報警音效。
- **Phase 4**: **資料庫遷移至 Supabase (PostgreSQL)**。
- **Phase 5**: 部署至 **Firebase Hosting**，後端採 **ngrok 隧道** 模式。
- **Phase 6 (Current)**: 全面容器化。使用 **Docker Compose** 啟動本地 PostgreSQL 與 Node.js 後端，並透過 **Cloudflare Tunnel** 提供公網存取。

## 2. 核心架構 (Current)
- **Frontend**: Vite + Vue 3 (部署於 Firebase Hosting)。
- **Backend Service**: 
  - Node.js Express 伺服器 (容器化)。
  - 處理業務邏輯、檔案上傳 (本地 Volume) 與 Socket.io 即時通訊。
- **Database**:
  - **Local PostgreSQL (Docker Container)**：取代了先前的 Supabase。
  - 資料表：`users`, `devices`, `common_state`, `snippets`, `calendar_events`, `common_text_history`。
- **Connectivity**: 
  - 使用 **Cloudflare Tunnel** (`cloudflared`) 代替 ngrok，解決了穩定性與自動化網址更新問題。
  - 自動網址抓取：透過 `catch_url.py` 監測 Log 並自動更新 `.env.production`。

## 3. 關鍵技術決策
- **One-Click Deploy**: 實作 `run.sh` 腳本整合「後端重啟 -> 網址更新 -> 前端編譯 -> Firebase 部署」的完整自動化流程。
- **Role-Based Access (RBAC)**: 新增 `subadmin` 角色。管理員可在 Admin Dashboard 分配權限。
- **全域字體縮放**：實作 `html { font-size: var(--base-font-size) }` 噴灑系統，確保 UI 在不同裝置上的易讀性。
- **即時性能監測**：實作端到端延遲追蹤（Pill 顯示），監控 瀏覽器 -> 隧道 -> 後端 的回應速度。

## 4. 共同文字歷史紀錄 (Text History)
為了解決多人共同作業時文字被覆蓋的問題，實作了歷史追蹤功能：
- **存儲策略**：獨立資料表 `common_text_history`，採取 **FIFO (先進先出)** 邏輯保留最新 10 筆。
- **即時廣播**：透過 `commonHistoryUpdate` Socket 事件，讓所有連結裝置的歷史清單同步更新。

## 5. Cloudflare Tunnel 聯動指南 (Critical Lessons)
當前端部署在 Firebase Hosting，而後端透過 Cloudflare 隧道連回本地開發機時，會遇到連線挑戰：
- **Cloudflare 警告頁面 (Interstitial Page)**： API 請求會被警告頁攔截導致 CORS 錯誤。
- **解決方案**：前端必須發送 `cf-skip-browser-warning: any` 標頭 (Axios / Socket.io)。
- **WebSocket 傳輸**：強制設定 `transports: ['websocket']` 確保穩定性。

## 6. 排查除錯 (Troubleshooting Quick-Start)
詳細除錯經驗與歷史錯誤紀錄，請參閱 [troubleshooting_logs.md](./troubleshooting_logs.md)。

## 7. 環境變數與機密資訊 (Secrets)
機密資訊應存在 `.env` (本地) 或 `.env.production` (部署用)，**禁止**寫入本文件或提交至 Git：
- `DATABASE_URL`: PostgreSQL 連線字串 (含密碼)。
- `VITE_API_URL`: 後端 Cloudflare 隧道網址。
