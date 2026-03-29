# 🛠️ Troubleshooting & Battle Lessons (除錯與戰鬥經驗紀錄)

本文件紀錄了 Kitty-Help 在開發與部署過程中遇到的重大錯誤及其解決方案，供未來維護參考。

## 1. 埠口衝突與殭屍程序 (Port 3000 Conflict)
- **錯誤現象**：容器啟動後不斷重啟，Log 顯示 `EADDRINUSE: address already in use :::3000`。
- **原因**：主機上有手動啟動的 `ts-node-dev` 或是舊的 Docker 容器殘留，佔用了 3000 埠口。
- **解決方案**：
  - 使用 `lsof -t -i:3000 | xargs kill -9` 強制結束佔用程序。
  - **腳本自動化**：在 `run.sh` 中加入 Pre-flight check，啟動前自動檢測並殺死 conflict PIDs。

## 2. Docker 環境下的路徑解析 (Path Resolution)
- **錯誤現象**：後端找不到 `.env` 或 `uploads/` 資料夾，報錯 `No such file or directory`。
- **原因**：在 Docker 容器內部，`__dirname` 的指向可能與本地開發環境不同。
- **解決方案**：
  - 統一使用 `path.resolve(process.cwd(), '.env')`。
  - `process.cwd()` 在 Docker 中通常指向 `/app`，這與 `docker-compose.yml` 的 `WORKDIR` 一致。

## 3. Cloudflare Tunnel (TryCloudflare) 的坑
### A. 網址動態性 (URL Dynamism)
- **問題**：每次重啟 `cloudflared` 容器，都會產生一個新的臨時網址（如 `envelope-navy...` -> `compete-strategies...`）。
- **影響**：前端編譯時如果寫死網址，重啟後會報 **530 (Origin Error)**。
- **解決方案**：實作 `catch_url.py` 自動攔截 Log 中的網址並寫入 `.env.production`，再進行前端編譯。

### B. 警告頁面攔截 (Interstitial Page)
- **問題**：新裝置訪問時，Cloudflare 會顯示警告頁，導致 API 請求拿到 HTML 而非 JSON，報 CORS 或 530 錯誤。
- **解決方案**：
  - 前端必須發送 `cf-skip-browser-warning: any` 標頭 (Axios / Socket.io / Fetch)。
  - 後端 CORS Middleware 必須允許此標頭。

## 4. API 路由路徑不一致 (404 Routing)
- **錯誤現象**：手機版報 `POST /api/devices/register 404 Not Found`，但電腦版正常（可能是電腦版緩存了舊邏輯）。
- **原因**：前端 `api.ts` 使用 `/api/devices/register`，但後端 `index.ts` 誤寫為 `/api/register`。
- **解決方案**：將後端路徑統一位於 `/api/devices/` 前綴下。

## 5. 全域字體縮放 (Font Scaling)
- **問題**：字體大小設定只對部分組件有效，Notice Board 等組件文字太小且不隨設定縮放。
- **規律**：開發時若使用了 `px` 或固定的 `rem` 且 `html` 根元素沒有動態設定，縮放會失效。
- **解決方案**：
  - 將 `html { font-size: var(--base-font-size) }` 與設定連動。
  - 組件內部改用 `em` 或不含單位的比例，確保繼承根元素的縮放力。
