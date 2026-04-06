#!/usr/bin/env python3
import argparse
import subprocess
import os
import sys
import time
import re

# --- Kitty-Help Configuration ---
ENV_FILE = ".env"
ENV_PROD_FILE = ".env.production"
TUNNEL_CONTAINER = "kitty-tunnel"
DB_CONTAINER = "kitty-db"  # Defaults to standard name, can be changed
DB_NAME = "kitty_help"
DB_USER = "toby"
BACKUP_FILE = "backup_kitty_local.sql"

# --- Terminal Colors ---
class Colors:
    HEADER = '\033[95m'
    OKBLUE = '\033[94m'
    OKCYAN = '\033[96m'
    OKGREEN = '\033[92m'
    WARNING = '\033[93m'
    FAIL = '\033[91m'
    ENDC = '\033[0m'
    BOLD = '\033[1m'
    UNDERLINE = '\033[4m'

def print_step(msg):
    print(f"{Colors.BOLD}{Colors.OKBLUE}==>{Colors.ENDC} {Colors.BOLD}{msg}{Colors.ENDC}")

def run_command(cmd, msg=None):
    if msg: print_step(msg)
    try:
        subprocess.run(cmd, shell=True, check=True)
    except subprocess.CalledProcessError as e:
        print(f"{Colors.FAIL}[ERROR]{Colors.ENDC} Command failed: {cmd}")

def update_env_files(url):
    """Sync VITE_API_URL across all relevant env files."""
    files_to_update = [ENV_FILE, ENV_PROD_FILE]
    for target in files_to_update:
        if not os.path.exists(target): continue
        with open(target, "r") as f:
            lines = f.readlines()
        new_lines = []
        updated = False
        for line in lines:
            if line.startswith("VITE_API_URL="):
                new_lines.append(f"VITE_API_URL={url}\n")
                updated = True
            else:
                new_lines.append(line)
        if not updated:
            new_lines.append(f"VITE_API_URL={url}\n")
        with open(target, "w") as f:
            f.writelines(new_lines)
        print(f"{Colors.OKCYAN}📝 Updated {target}{Colors.ENDC}")

def catch_tunnel_url():
    """Monitor docker logs to catch the trycloudflare URL."""
    print_step(f"🔍 Monitoring [{TUNNEL_CONTAINER}] for Cloudflare Tunnel URL...")
    process = subprocess.Popen(
        ["docker", "logs", "--tail", "50", "-f", TUNNEL_CONTAINER],
        stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True
    )
    url_pattern = re.compile(r"https://[a-zA-Z0-9-]+\.trycloudflare\.com")
    try:
        for line in process.stdout:
            match = url_pattern.search(line)
            if match:
                new_url = match.group(0)
                print(f"{Colors.OKGREEN}🎉 Caught New URL: {new_url}{Colors.ENDC}")
                update_env_files(new_url)
                process.terminate()
                return True
    except KeyboardInterrupt:
        process.terminate()
        print("\nAborted.")
    return False

def export_db():
    """Backup the local database to a SQL file."""
    print_step(f"🗄️ Exporting database [{DB_NAME}] from [{DB_CONTAINER}]...")
    # Using docker exec to run pg_dump. 
    # Note: We try to find the container name dynamically if 'kitty-db' fails
    cmd = f"docker exec -t {DB_CONTAINER} pg_dump -U {DB_USER} {DB_NAME} > {BACKUP_FILE}"
    try:
        subprocess.run(cmd, shell=True, check=True)
        print(f"{Colors.OKGREEN}✅ Successfully backed up to: {BACKUP_FILE}{Colors.ENDC}")
    except Exception as e:
        print(f"{Colors.WARNING}⚠️ Failed with default container name, trying to find any postgres container...{Colors.ENDC}")
        # Fallback: try to find any container running postgres
        fb_cmd = f"docker ps --format '{{{{.Names}}}}' | grep db | head -n 1"
        container = subprocess.getoutput(fb_cmd)
        if container:
            print(f"🔍 Found candidate container: {container}")
            cmd = f"docker exec -t {container} pg_dump -U {DB_USER} {DB_NAME} > {BACKUP_FILE}"
            subprocess.run(cmd, shell=True)
            print(f"{Colors.OKGREEN}✅ Backup complete via {container}.{Colors.ENDC}")

def import_db():
    """Restore the database from a SQL file."""
    if not os.path.exists(BACKUP_FILE):
        print(f"{Colors.FAIL}[ERROR]{Colors.ENDC} Backup file {BACKUP_FILE} not found!")
        return

    print(f"{Colors.WARNING}{Colors.BOLD}🚨 WARNING: This will OVERWRITE your current database data!{Colors.ENDC}")
    confirm = input("Are you absolutely sure? (type 'yes' to proceed): ")
    if confirm.lower() != 'yes':
        print("Restoration cancelled.")
        return

    print_step(f"📥 Restoring database from {BACKUP_FILE}...")
    # 1. Clear existing data (optional but recommended for clean start)
    # 2. Run psql
    cmd = f"cat {BACKUP_FILE} | docker exec -i {DB_CONTAINER} psql -U {DB_USER} -d {DB_NAME}"
    try:
        subprocess.run(cmd, shell=True, check=True)
        print(f"{Colors.OKGREEN}🎉 Database restoration SUCCESSFUL!{Colors.ENDC}")
    except Exception as e:
        print(f"{Colors.FAIL}❌ Error during restoration: {e}{Colors.ENDC}")

def main():
    parser = argparse.ArgumentParser(description='🚀 Kitty-Help Full Stack Orchestrator')
    
    parser.add_argument('-d', '--docker', action='store_true', help='Restart Backend Docker containers')
    parser.add_argument('-c', '--catch', action='store_true', help='Catch Tunnel URL and update .env')
    parser.add_argument('-b', '--build', action='store_true', help='Build Frontend (npm run build)')
    parser.add_argument('-p', '--deploy', action='store_true', help='Deploy to Firebase Hosting')
    parser.add_argument('-e', '--export', action='store_true', help='Backup/Export Database to SQL')
    parser.add_argument('-i', '--import-db', action='store_true', help='Restore/Import Database from SQL')
    parser.add_argument('-all', '--full', action='store_true', help='Run everything (default if no flags)')
    parser.add_argument('--kill', action='store_true', help='Force kill port 3000')

    args = parser.parse_args()
    
    # If using specific DB flags, we don't run_all
    db_ops = args.export or args.import_db
    run_all = not db_ops and (args.full or not any([args.docker, args.catch, args.build, args.deploy]))

    os.chdir(os.path.dirname(os.path.abspath(__file__)))

    print(f"{Colors.HEADER}{Colors.BOLD}--- 🐱 Kitty-Help Unified Management System ---{Colors.ENDC}\n")

    if args.export:
        export_db()
        return

    if args.import_db:
        import_db()
        return

    start_time = time.time()

    if args.kill or run_all:
        print_step("0/4: Cleaning up port 3000...")
        subprocess.run("sudo fuser -k 3000/tcp 2>/dev/null || true", shell=True)
        time.sleep(1)

    if args.docker or run_all:
        run_command("docker compose down", "1/4: Stopping containers...")
        run_command("docker compose up -d --build", "1/4: Rebuilding and starting Backend...")

    if args.catch or run_all:
        catch_tunnel_url()

    if args.build or run_all:
        run_command("npm run build", "3/4: Building Frontend assets...")

    if args.deploy or run_all:
        run_command("firebase deploy --only hosting", "4/4: Deploying to Firebase...")

    end_time = time.time()
    print(f"\n{Colors.OKGREEN}{Colors.BOLD}✅ [SUCCESS]{Colors.ENDC} Completed in {int(end_time - start_time)}s\n")

if __name__ == "__main__":
    main()
