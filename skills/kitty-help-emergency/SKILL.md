---
name: kitty-help-emergency
description: 專用於 Kitty Help 專案的急救工作流。當專案需要重新部署、修復或更新時，使用此 skill 以 conda 環境 toby 執行 run.sh。
---

# Kitty Help 急救工作流

此 skill 提供了一個標準化的程序，用於在 Kitty Help 專案遇到問題或需要快速部署時執行「急救」腳本。

## 主要工作流

此工作流會切換到專案目錄，並使用 `conda` 環境 `toby` 執行專案根目錄下的 `run.sh`。

### 執行方式

您可以手動執行：

```bash
conda run -n toby /home/toby/documents/projects/kitty-help/run.sh
```

或使用本 skill 附帶的腳本：

```bash
bash scripts/emergency_fix.sh
```

## 當遇到以下情況時使用：

-   前端或後端服務異常需要重啟。
-   需要重新捕捉 Cloudflare Tunnel URL。
-   需要手動重新部署到 Firebase。
-   專案代碼更新後需要完整構建與部署。

## 注意事項

-   `run.sh` 會先清理埠號 3000 的佔用，然後重啟 Docker 容器。
-   腳本包含 Python 腳本 `catch_url.py` 的執行，確保相關依賴已安裝於 `toby` 環境中。
