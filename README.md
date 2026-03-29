# 🐱 Kitty-Help (PG Edition)

Kitty-Help 是一個跨裝置的輔助溝通工具，專門為家庭或小團隊設計，提供即時的文字/圖片同步、個人化的階層式剪貼簿，以及生產力倒數計時器。

## ✨ 特色功能

1.  **🚀 高效儀表板**：整合了歡迎詞、系統性能監測、倒數計時器及個人/共用剪貼簿。
2.  **📅 家庭日曆**：同步月份日曆，支援個人備忘錄紀錄，首頁可顯示全家人的彩色標記。
3.  **🧮 內建計算機**：整合於個人頁面的精美計算機。
4.  **⏲️ 倒數計時器**：支援時、分、秒設定，具備霓虹進度條與警報。
5.  **📚 階層式剪貼簿 (Personal Board)**：支援語音輸入、JSON 匯入匯出與無限層級資料夾。
6.  **🖼️ 共用剪貼簿**：即時同步的圖片與文字，支援 10 筆歷史紀錄。
7.  **🛡️ 管理員後台**：裝置審核、使用者角色分配 (Admin/SubAdmin)、即時 Socket.io 更新。
8.  **📊 性能監測**：端到端（瀏覽器 -> Firebase -> 隧道 -> 後端）的延遲分析。

---

## 🛠️ 技術棧

-   **Frontend**: Vue 3 (Vite), TypeScript, CSS Variables (Glassmorphism).
-   **Backend**: Node.js (Express), Socket.io, Multer, PostgreSQL.
-   **Containerization**: Docker, Docker Compose.
-   **Connectivity**: Cloudflare Tunnel (`cloudflared`).
-   **Hosting**: Firebase Hosting (Targeting: `kitty-help.web.app`).

---

## 🏗️ 穩定性開發三階段

### 第一階段：本地開發 (Dev)
- **環境**：根目錄執行 `npm run dev`，`/server` 目錄執行 `npm run dev`。
- **重點**：快速迭代業務邏輯，直接連接本地或 Supabase 資料庫。

### 第二階段：預行環境 (Staging)
- **環境**：透過 `ngrok` 或 `cloudflared` 手動轉發本地 3000 埠。
- **目標**：驗證 Firebase Hosting 上的前端能與動態後端正常通訊。

### 第三階段：生產隔離測試 (Prod-Isolated)
- **環境**：使用 **Docker Compose** 與 **Cloudflare Tunnel**。
- **目標**：驗證「容器化 + 外部存取」的完整性。

#### 1. 啟動 Docker 後端
```bash
docker compose up -d --build
```

#### 2. 自動更新前端網址
使用 `catch_url.py` 自動抓取隧道網址並寫入 `.env.production`：
```bash
python3 catch_url.py
```

#### 3. 部署前端至 Firebase
```bash
npm run build
firebase deploy --only hosting
```

---

## 🚀 一鍵快速部署 (One-Click Deploy)

為了簡化流程，專案提供了一個自動化腳本 `run.sh`：

```bash
sh run.sh
```

**這個腳本會自動完成：**
1.  **Backend**: 關閉並重新編譯/啟動 Docker 容器。
2.  **Tunnel**: 自動抓取最新的 Cloudflare 網址並寫入環境變數。
3.  **Frontend**: 使用最新網址進行 Vite 編譯。
4.  **Firebase**: 自動部署前端至 Firebase Hosting。

> [!TIP]
> 部署完畢後，請務必 **重新整理瀏覽器** 以確保載入最新的後端網址。

---

## 🔒 安全注意事項

以下檔案包含敏感資訊，已在 `.gitignore` 中排除：
-   `.env`：資料庫連線字串。
-   `.env.production`：暫時性的後端隧道網址。
-   `uploads/`：使用者上傳的私密圖檔。
-   `firebaseConfig.ts`：Firebase 專案金鑰。

---

## 🎥 參考文檔
- [Stage 3 Walkthrough & Demo](file:///home/toymsi/.gemini/antigravity/brain/2b51a0d3-068b-4fe5-9011-6db68c6acc9d/walkthrough.md)
- [Project Implementation Plan](file:///home/toymsi/.gemini/antigravity/brain/2b51a0d3-068b-4fe5-9011-6db68c6acc9d/implementation_plan.md)
