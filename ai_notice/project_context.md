# 🐱 Kitty-Help 專案背景與架構紀錄 (Project Context)

本文件用於記錄 Kitty-Help 的核心架構演進與關鍵決策，供未來 AI 續寫代碼或維護時參考。

## 1. 專案演進里程碑
- **Phase 1-2**: 初始原型，使用 Firebase Firestore。
- **Phase 3**: 導入 **個人階層式剪貼簿 (Tree View)**，支援兩欄式佈局與無限層級資料夾。
- **Phase 3.5**: 新增 **生產力倒數計時器**，整合 Web Notification 與報警音效。
- **Phase 4**: **資料庫遷移至 Supabase (PostgreSQL)**。
- **Phase 5**: 部署至 **Firebase Hosting**，後端採 **ngrok 隧道** 模式。
- **Phase 6 (Stage 3 - Current)**: 全面容器化。使用 **Docker Compose** 啟動本地 PostgreSQL 與 Node.js 後端，並透過 **Cloudflare Tunnel** 提供公網存取。

## 2. 核心架構 (Current)
- **Frontend**: Vite + Vue 3 (部署於 Firebase Hosting)。
- **Backend Service**: 
  - Node.js Express 伺服器 (容器化)。
  - 處理業務邏輯、檔案上傳 (本地 Volume) 與 Socket.io 即時通訊。
- **Database**:
  - **Local PostgreSQL (Docker Container)**：取代了先前的 Supabase。
  - 資料表：`users`, `devices`, `common_state`, `snippets`, `calendar_events`, `common_text_history`。
- **Connectivity**: 
  - 使用 **Cloudflare Tunnel** (`cloudflared`) 代替 ngrok，解決了 interstitial 警告頁面問題。
  - 自動網址抓取：透過 `catch_url.py` 監頻 Log 並自動更新 `.env.production`。

## 3. 關鍵技術決策
- **One-Click Deploy**: 實作 `run.sh` 腳本整合「後端重啟 -> 網址更新 -> 前端編譯 -> Firebase 部署」的完整自動化流程。
- **Role-Based Access (RBAC)**: 新增 `subadmin` 角色。管理員可在 Admin Dashboard 分配權限。
- **即時性能監測**：實作端到端延遲追蹤（Pill 顯示），監控 瀏覽器 -> 隧道 -> 後端 的回應速度。

## 5. Firebase + ngrok 聯動指南 (Critical Lessons)
當前端部署在 Firebase Hosting，而後端透過 ngrok 隧道連回本地開發機時，會遇到嚴重的連線挑戰。

### 核心難點與解決方案
- **ngrok 警告頁面 (Interstitial Page)**：
  - **現象**：API 請求回傳 200 OK 但內容是 HTML（ngrok 的警告頁），導致 JSON 解析失敗或 CORS 報錯。
  - **解決方案**：
    - 前端 **Axios**：設定 `defaults.headers.common['ngrok-skip-browser-warning'] = 'true'`。
    - 前端 **Fetch (圖片複製)**：在 `fetch(url, { headers: { 'ngrok-skip-browser-warning': 'any' } })` 中加入標頭。
- **CORS 跨域攔截**：
  ... (No change) ...

## 8. 共同文字歷史紀錄 (Text History)
為了解決多人共同作業時文字被覆蓋的問題，實作了歷史追蹤功能：
- **存儲策略**：獨立資料表 `common_text_history`，紀錄 `content`, `user_id`, `created_at`。
- **維護邏輯**：後端採取 **FIFO (先進先出)**，每次更新 `common_state` 時寫入歷史，並主動刪除超過 10 筆的舊資料，僅保留最新 10 筆。
- **即時廣播**：透過 `commonHistoryUpdate` Socket 事件，讓所有連結裝置的歷史清單同步更新。
- **快速複製**：UI 實作「點擊項目即複製」，方便使用者找回被洗掉的資訊。
  - **現象**：`Access-Control-Allow-Origin` 缺失或與 `credentials` 衝突。
  - **解決方案**：後端實作「暴力 CORS Middleware」，動態反射 Request Origin，並顯式處理並結束 `OPTIONS` 預檢請求，避免 matched route 衝突。
- **Socket.io 輪詢失敗**：
  - **現象**：Socket.io 預設的 Polling 模式極易觸發跨域錯誤且受 ngrok 阻擋。
  - **解決方案**：前端強制設定 `transports: ['websocket']`，直接建立穩固的二進位協議連線。

## 6. 排查除錯 (Troubleshooting Quick-Start)
若未來連線中斷（Connection Error），請依序執行：
1. **ngrok URL**：檢查 `.env.production` 的 `VITE_API_URL` 是否對齊。若網址變動，必須 `npm run build` 並 `firebase deploy`。
2. **手動測試**：瀏覽器直入 ngrok 網址。若見「Cannot GET /」表示後端通暢。
3. **後端 Log**：若 Node.js 終端機無 Request 進來，表示請求被 ngrok 擋掉或網址錯誤。
4. **Header 驗證**：在 Network 分頁確認請求是否帶有 `ngrok-skip-browser-warning` 標頭。

## 7. 環境變數與機密資訊 (Secrets)
機密資訊應存在 `.env` (本地) 或 `.env.production` (部署用)，**禁止**寫入本文件或提交至 Git：
- `DATABASE_URL`: Supabase 連線字串 (含密碼)。
- `VITE_API_URL`: 後端 ngrok 或雲端 Server 網址。
