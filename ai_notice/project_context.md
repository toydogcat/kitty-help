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

## 4. 環境變數規範 (Secrets)
機密資訊應存在 `.env` (本地) 或 `.env.production` (部署用)，**禁止**寫入本文件或提交至 Git：
- `DATABASE_URL`: Supabase 連線字串 (含密碼)。
- `VITE_API_URL`: 後端 ngrok 或雲端 Server 網址。

## 5. 給 AI 的維護提醒
- **ID 識別**：UI 顯示應統一取 **Device ID 的前 8 碼**，避免使用 User ID 造成混淆。
- **兩欄式佈局**：`SnippetExplorer.vue` 採用 Split Pane 設計，左側為樹狀結構，右側為編輯區。
- **Socket 事件**：注意 `commonUpdate`, `snippetUpdate`, `deviceStatusUpdate` 等事件的廣播邏輯。
