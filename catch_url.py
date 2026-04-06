import subprocess
import re
import os
import sys

# --- 戰旗配置 (NUC Flagship Edition) ---
# .env 檔案位置，建議統一套用 .env (Vite 也支援)
ENV_FILE = os.path.join(os.path.dirname(__file__), ".env")
# 必須對應 docker-compose.yml 中的 container_name
CONTAINER_NAME = "kitty-tunnel" 

def catch_and_update():
    print(f"🔍 偵測戰訊：正在監控 [{CONTAINER_NAME}] 以捕捉臨時傳送門...")
    
    # 執行 docker logs -f
    process = subprocess.Popen(
        ["docker", "logs", "-f", CONTAINER_NAME],
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True
    )
    
    # 鎖定 trycloudflare 的隨機網域頻率
    url_pattern = re.compile(r"https://[a-zA-Z0-9-]+\.trycloudflare\.com")
    
    try:
        for line in process.stdout:
            match = url_pattern.search(line)
            if match:
                new_url = match.group(0)
                print(f"🎉 攔截成功！Cloudflare 專屬傳送門: {new_url}")
                
                # 同步更新 .env 檔案
                update_env(new_url)
                
                print("\n✅ 全軍對位完成！您現在可以：")
                print(f"   1. 直接開啟上述網址 (部屬雞全方位版)")
                print(f"   2. 或執行 npm run build 以鑄造靜態資源")
                
                # 任務達成，停止監聽
                process.terminate()
                return
    except KeyboardInterrupt:
        process.terminate()
        print("\n中斷監控。")

def update_env(url):
    # 如果同時有 .env.production，也一併更新
    files_to_update = [ENV_FILE, os.path.join(os.path.dirname(__file__), ".env.production")]
    
    for target_file in files_to_update:
        if not os.path.exists(target_file):
            continue

        with open(target_file, "r") as f:
            lines = f.readlines()

        updated = False
        new_lines = []
        for line in lines:
            if line.startswith("VITE_API_URL="):
                new_lines.append(f"VITE_API_URL={url}\n")
                updated = True
            else:
                new_lines.append(line)
        
        if not updated:
            if new_lines and not new_lines[-1].endswith('\n'):
                new_lines[-1] += '\n'
            new_lines.append(f"VITE_API_URL={url}\n")

        with open(target_file, "w") as f:
            f.writelines(new_lines)
        print(f"📝 已同步環境變數：{os.path.basename(target_file)}")

if __name__ == "__main__":
    catch_and_update()