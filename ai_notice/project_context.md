# 🐱 Kitty-Help 專案背景與架構紀錄 (Project Context)

本文件用於記錄 Kitty-Help 的核心架構演進與關鍵決策，供未來 AI 續寫代碼或維護時參考。

## 1. 專案演進里程碑
- **Phase 1-2**: 初始原型，使用 Firebase Firestore。
- **Phase 3**: 導入 **個人階層式剪貼簿 (Tree View)**，支援兩欄式佈局與無限層級資料夾。
- **Phase 3.5**: 新增 **生產力倒數計時器**，整合 Web Notification 與報警音效。
- **Phase 4**: **資料庫遷移至 Supabase (PostgreSQL)**。
- **Phase 5**: 部署至 **Firebase Hosting**，後端採 **ngrok 隧道** 模式。

## 2. 核心架構 (Current)
- **Frontend**: Vite + Vue 3。
- **Backend Service**: 
  - Node.js Express 伺服器處理業務邏輯、檔案上傳與 Socket.io 即時通訊。
  - 接介 Supabase PostgreSQL。
- **Database**:
  - 資料表：`users`, `devices`, `common_state`, `snippets`。
  - 使用者識別：透過 `kitty_device_id` (UUID) 進行裝置綁定。
- **Real-time**: 
  - 使用 Socket.io 確保多裝置間的「共同剪貼簿」即時同步。

## 3. 關鍵技術決策
- **IPv4 相容性**：由於部分網路環境不支援 IPv6，DB 連線應優先使用 Supabase 的 **Transaction Pooler (Port 6543)**。
- **圖片相容性修復**：瀏覽器 Clipboard API 僅支援 PNG。系統內建 **JPEG-to-PNG Canvas 轉換器**，確保所有複製行為皆能成功。
- **檔案存取**：後端上傳採相對路徑 `/uploads/`。前端透過 `VITE_API_URL` 變數動態組合成完成網址，相容 localhost 與 ngrok 環境。

## 5. Firebase + ngrok 聯動指南 (Critical Lessons)
當前端部署在 Firebase Hosting，而後端透過 ngrok 隧道連回本地開發機時，會遇到嚴重的連線挑戰。

### 核心難點與解決方案
- **ngrok 警告頁面 (Interstitial Page)**：
  - **現象**：API 請求回傳 200 OK 但內容是 HTML（ngrok 的警告頁），導致 JSON 解析失敗或 CORS 報錯。
  - **解決方案**：在前端 API 請求 Header 中加入 `ngrok-skip-browser-warning: "true"` (Vite / Axios 中設定 `defaults.headers.common`)。
- **CORS 跨域攔截**：
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
