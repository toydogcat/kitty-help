# 🐱 Kitty-Help (PG Edition)

Kitty-Help 是一個跨裝置的輔助溝通工具，專門為家庭或小團隊設計，提供即時的文字/圖片同步、個人化的階層式剪貼簿，以及生產力倒數計時器。

## ✨ 特色功能

1.  **🚀 高效儀表板**：整合了歡迎詞、倒數計時器、個人剪貼簿與共用剪貼簿。
2.  **⏲️ 倒數計時器**：支援時、分、秒設定，具備霓虹進度條、桌面通知與聲音警報。
3.  **📚 階層式剪貼簿 (Tree View)**：左側樹狀導航，右側內容編輯，支援無限層級資料夾。
4.  **🖼️ 共用剪貼簿**：即時同步的圖片與文字，支援一鍵複製圖片 (自動將 JPEG 轉為 PNG 以相容剪貼簿)。
5.  **🛡️ 管理員後台**：裝置審核機制、使用者角色分配、即時 Socket.io 更新。

---

## 🛠️ 技術棧

-   **Frontend**: Vue 3 (Vite), TypeScript, CSS Variables (Glassmorphism).
-   **Backend**: Node.js (Express), Socket.io, Multer.
-   **Database**: PostgreSQL (Managed on **Supabase**).
-   **Hosting**: Firebase Hosting.

---

## ⚙️ 本地開發設定

### 1. 安裝環境
在根目錄與 `/server` 目錄下分別執行：
```bash
npm install
```

### 2. 資料庫設定 (Supabase)
本專案已遷移至 Supabase。若要自行建立新資料庫：
1.  在 Supabase 建立新專案。
2.  前往 **SQL Editor** 並執行專案中的 `migration.sql` 腳本。
3.  在根目錄建立 `.env` 檔案，填入你的 Supabase 連線資訊：
    ```bash
    DATABASE_URL=postgresql://postgres:[PASSWORD]@db.[PROJECT-ID].supabase.co:6543/postgres
    ```

### 3. 啟動服務
-   **啟動後端** (Port 3000):
    ```bash
    cd server
    npm run dev
    ```
-   **啟動前端** (Port 5173):
    ```bash
    npm run dev
    ```

---

## 🚀 部署指南 (Firebase Hosting)

由於後端伺服器 (Node.js) 目前跑在本地，我們需要透過 **ngrok** 來讓 Firebase 上的前端能連到你的電腦。

### 1. 啟動 ngrok 隧道
在你的伺服器電腦上執行：
```bash
ngrok http 3000
```
複製得到的 `https://xxxx.ngrok-free.app` 網址。

### 2. 設定生產環境變數
在根目錄建立 `.env.production`：
```bash
VITE_API_URL=https://您的-ngrok-網址.ngrok-free.app
```

### 3. 建置與部署
```bash
npm run build
firebase deploy --only hosting
```

---

## 🔒 安全注意事項 (機密檔案)

以下檔案包含敏感資訊（如資料庫密碼、API Key），**切勿上傳至 Git 公開倉庫**：
-   `.env`：包含 Supabase 資料庫主密碼與連線字串。
-   `.env.production`：包含暫時性的 ngrok 存取網址。
-   `uploads/`：包含使用者上傳的私密圖檔。
-   `firebaseConfig.ts`：包含 Firebase 專案憑證。

本專案已在 `.gitignore` 中設定忽略上述檔案。

---

## 🎥 演示影片與文檔
-   [Walkthrough & Demo](file:///home/toymsi/.gemini/antigravity/brain/660367df-c9ca-4ded-accb-304c19165ac2/walkthrough.md)
-   [Initial Implementation Plan](file:///home/toymsi/.gemini/antigravity/brain/660367df-c9ca-4ded-accb-304c19165ac2/implementation_plan.md)
