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

## 🚀 部署細節 (Firebase Hosting)

本專案已設定專屬 Hosting Site：`https://kitty-help.web.app`。
執行部署時將自動鎖定該站台，不會影響同 Project 下的其他應用程式。

```bash
firebase deploy --only hosting
```

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
