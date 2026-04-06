#!/home/toymsi/miniconda3/envs/toby/bin/python
import os
import subprocess
import argparse
import sys
from urllib.parse import urlparse

# 指向根目錄的 .env 檔案
ENV_PATH = "/home/toymsi/documents/projects/Github/kitty-help/.env"

def load_db_url():
    if not os.path.exists(ENV_PATH):
        print(f"❌ Error: {ENV_PATH} not found.")
        sys.exit(1)
    
    with open(ENV_PATH, 'r') as f:
        for line in f:
            if line.startswith('DATABASE_URL='):
                return line.split('=')[1].strip()
    
    print("❌ Error: DATABASE_URL not found in .env")
    sys.exit(1)

def run_psql(db_name, sql, db_url):
    parsed = urlparse(db_url)
    cmd = [
        "psql", "-h", parsed.hostname, "-p", str(parsed.port or 5432),
        "-U", parsed.username, "-d", db_name, "-c", sql
    ]
    env = os.environ.copy()
    env["PGPASSWORD"] = parsed.password
    # 使用 capture_output 避免干擾主流程
    subprocess.run(cmd, env=env, check=True, capture_output=True)

def transfer_data(source_db, target_db, db_url):
    parsed = urlparse(db_url)
    env = os.environ.copy()
    env["PGPASSWORD"] = parsed.password

    # pg_dump 導出 -> psql 導入 (不包含權限 owner 資訊避免報錯)
    dump_cmd = [
        "pg_dump", "-h", parsed.hostname, "-p", str(parsed.port or 5432),
        "-U", parsed.username, "-d", source_db, "--no-owner"
    ]
    restore_cmd = [
        "psql", "-h", parsed.hostname, "-p", str(parsed.port or 5432),
        "-U", parsed.username, "-d", target_db
    ]

    print(f"⏳ 正在搬移數據: {source_db} -> {target_db}...")
    p1 = subprocess.Popen(dump_cmd, env=env, stdout=subprocess.PIPE)
    p2 = subprocess.Popen(restore_cmd, env=env, stdin=p1.stdout)
    p1.stdout.close()
    p2.communicate()

    if p2.returncode == 0:
        print(f"✅ 數據同步成功: {source_db} 已鏡像至 {target_db}")
    else:
        print(f"❌ 數據搬移失敗。")
        sys.exit(1)

def backup(db_url):
    parsed = urlparse(db_url)
    source_db = parsed.path[1:]
    backup_db = "kitty_help_backup"

    print(f"📦 [備份啟動] 主資料庫 {source_db} -> 備份庫 {backup_db}")

    # 1. 無情清空並重建備份資料庫
    try:
        run_psql("postgres", f"DROP DATABASE IF EXISTS {backup_db} WITH (FORCE);", db_url)
        run_psql("postgres", f"CREATE DATABASE {backup_db};", db_url)
        print(f"✅ 已重置備份資料庫: {backup_db}")
    except Exception as e:
        print(f"❌ 無法重置備份資料庫: {e}")
        sys.exit(1)

    transfer_data(source_db, backup_db, db_url)

def restore(db_url):
    parsed = urlparse(db_url)
    target_db = parsed.path[1:]
    source_db = "kitty_help_backup"

    print(f"⚠️ [還原啟動] 正在使用 {source_db} 「無情取代」主資料庫 {target_db}")

    # 1. 使用 WITH (FORCE) 強制斷連並重建目標資料庫
    try:
        run_psql("postgres", f"DROP DATABASE IF EXISTS {target_db} WITH (FORCE);", db_url)
        run_psql("postgres", f"CREATE DATABASE {target_db};", db_url)
        print(f"🛑 已終止所有連線並重置主資料庫: {target_db}")
    except Exception as e:
        print(f"❌ 無法重置主資料庫進行還原: {e}")
        sys.exit(1)

    transfer_data(source_db, target_db, db_url)

if __name__ == "__main__":
    # python db_tool.py backup
    # python db_tool.py restore
    parser = argparse.ArgumentParser(description="Kitty-Help 無情資料庫管理工具")
    parser.add_argument("action", choices=["backup", "restore"], help="backup: 備份, restore: 還原")
    args = parser.parse_args()

    db_url = load_db_url()

    if args.action == "backup":
        backup(db_url)
    elif args.action == "restore":
        restore(db_url)
