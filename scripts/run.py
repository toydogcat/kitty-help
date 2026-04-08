#!/usr/bin/env python3
import argparse
import subprocess
import os
import sys
import time
import re
import zipfile
import shutil

# --- Kitty-Help Configuration ---
ENV_FILE = ".env"
ENV_PROD_FILE = ".env.production"
TUNNEL_CONTAINER = "kitty-tunnel"
DB_CONTAINER = "kitty-db"
DB_NAME = "kitty_help"
DB_USER = "toby"
BACKUP_DIR = "/home/toymsi/文件/等待整理"
BACKUP_FILE = os.path.join(BACKUP_DIR, "kitty_help_backup.sql")

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

def fix_epub_file(src_path):
    """Sanitize EPUB: Remove scripts, inline events, and broken res:/// font links."""
    if not os.path.exists(src_path):
        print(f"{Colors.FAIL}[ERROR]{Colors.ENDC} File not found: {src_path}")
        return
    
    filename = os.path.basename(src_path)
    if not os.path.exists(BACKUP_DIR):
        os.makedirs(BACKUP_DIR, exist_ok=True)
    
    dest_path = os.path.join(BACKUP_DIR, filename.replace(".epub", "_Cleaned.epub"))
    print_step(f"🛡️  Purifying Intel: {filename} ...")
    
    # Cleaning Logic
    font_pattern = re.compile(r'@font-face\s*\{[^}]*res:\/\/[^}]*\}', re.IGNORECASE | re.DOTALL)
    url_res_pattern = re.compile(r'url\(["\']?res:\/\/[^)]+\)', re.IGNORECASE)
    script_pattern = re.compile(r'<script\b[^>]*>([\s\S]*?)<\/script>', re.IGNORECASE)
    on_attr_pattern = re.compile(r'\son\w+="[^"]*"', re.IGNORECASE)

    try:
        with zipfile.ZipFile(src_path, 'r') as src_zip:
            with zipfile.ZipFile(dest_path, 'w', compression=zipfile.ZIP_DEFLATED) as dest_zip:
                for item in src_zip.infolist():
                    content = src_zip.read(item.filename)
                    if item.filename.lower().endswith(('.html', '.xhtml', '.css', '.htm')):
                        text = content.decode('utf-8', errors='ignore')
                        text = script_pattern.sub('', text)
                        text = on_attr_pattern.sub('', text)
                        text = font_pattern.sub('', text)
                        text = url_res_pattern.sub('none', text)
                        dest_zip.writestr(item, text.encode('utf-8'))
                    else:
                        dest_zip.writestr(item, content)
        
        print(f"{Colors.OKGREEN}✅ Success! Intel sanitized at: {dest_path}{Colors.ENDC}")
    except Exception as e:
        print(f"{Colors.FAIL}❌ Purification Error: {e}{Colors.ENDC}")

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
    """Backup the remote database (150) using Docker tool container."""
    if not os.path.exists(BACKUP_DIR):
        print_step(f"📂 Creating backup directory: {BACKUP_DIR}")
        os.makedirs(BACKUP_DIR, exist_ok=True)

    print_step(f"🗄️ Exporting database [kitty_help] from 192.168.0.150...")
    cmd = f'docker run --rm -e PGPASSWORD=andy1984 postgres:latest pg_dump -h 192.168.0.150 -U toby kitty_help > "{BACKUP_FILE}"'
    try:
        subprocess.run(cmd, shell=True, check=True)
        print(f"{Colors.OKGREEN}✅ Successfully backed up to: {BACKUP_FILE}{Colors.ENDC}")
        print(f"{Colors.OKCYAN}📍 File Size: {os.path.getsize(BACKUP_FILE) / 1024:.2f} KB{Colors.ENDC}")
    except Exception as e:
        print(f"{Colors.FAIL}❌ Backup failed: {e}{Colors.ENDC}")

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

    print_step(f"📥 Restoring database from {BACKUP_FILE} to 192.168.0.150...")
    cmd = f'cat "{BACKUP_FILE}" | docker run -i --rm -e PGPASSWORD=andy1984 postgres:latest psql -h 192.168.0.150 -U toby -d kitty_help'
    try:
        subprocess.run(cmd, shell=True, check=True)
        print(f"{Colors.OKGREEN}🎉 Database restoration SUCCESSFUL!{Colors.ENDC}")
    except Exception as e:
        print(f"{Colors.FAIL}❌ Error during restoration: {e}{Colors.ENDC}")

def get_obsidian_path():
    """Extract LOCAL_OBSIDIAN_PATH from .env."""
    if not os.path.exists(ENV_FILE): return None
    with open(ENV_FILE, "r") as f:
        for line in f:
            if line.startswith("LOCAL_OBSIDIAN_PATH="):
                return line.split("=", 1)[1].strip()
    return None

def sync_obsidian():
    """Run git pull in the local obsidian vault directory."""
    path = get_obsidian_path()
    if not path:
        print(f"{Colors.FAIL}[ERROR]{Colors.ENDC} LOCAL_OBSIDIAN_PATH not found in .env")
        return
    
    print_step(f"🔄 Syncing Obsidian Vault: {path}")
    if not os.path.exists(path):
        print(f"{Colors.FAIL}[ERROR]{Colors.ENDC} Directory does not exist: {path}")
        return
    
    try:
        subprocess.run(f"cd {path} && git pull", shell=True, check=True)
        print(f"{Colors.OKGREEN}✅ Obsidian Vault synced successfully!{Colors.ENDC}")
    except Exception as e:
        print(f"{Colors.FAIL}❌ Git Sync failed: {e}{Colors.ENDC}")

def main():
    parser = argparse.ArgumentParser(description='🚀 Kitty-Help Full Stack Orchestrator')
    
    parser.add_argument('-d', '--docker', action='store_true', help='Restart Backend Docker containers')
    parser.add_argument('-c', '--catch', action='store_true', help='Catch Tunnel URL and update .env')
    parser.add_argument('-b', '--build', action='store_true', help='Build Frontend (npm run build)')
    parser.add_argument('-p', '--deploy', action='store_true', help='Deploy to Firebase Hosting')
    parser.add_argument('-e', '--export', action='store_true', help='Backup/Export Database to SQL')
    parser.add_argument('-i', '--import-db', action='store_true', help='Restore/Import Database from SQL')
    parser.add_argument('-f', '--fix', type=str, help='Purify EPUB file (remove res:/// and scripts)')
    parser.add_argument('-obs', '--obsidian', action='store_true', help='Sync Obsidian Vault (git pull)')
    parser.add_argument('-all', '--full', action='store_true', help='Run everything (default if no flags)')
    parser.add_argument('--kill', action='store_true', help='Force kill port 3000')

    args = parser.parse_args()
    
    db_ops = args.export or args.import_db or args.fix or args.obsidian
    run_all = not db_ops and (args.full or not any([args.docker, args.catch, args.build, args.deploy]))

    # Set working directory to project root (parent of scripts/)
    os.chdir(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

    print(f"{Colors.HEADER}{Colors.BOLD}--- 🐱 Kitty-Help Unified Management System ---{Colors.ENDC}\n")

    if args.obsidian:
        sync_obsidian()
        if not args.full: return

    if args.fix:
        fix_epub_file(args.fix)
        return

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
        run_command("docker compose --env-file .env -f infra/docker-compose.yml down", "1/4: Stopping containers...")
        run_command("docker compose --env-file .env -f infra/docker-compose.yml up -d --build", "1/4: Rebuilding and starting Backend...")

    if args.catch or run_all:
        catch_tunnel_url()

    if args.build or run_all:
        run_command("cd frontend && npm run build", "3/4: Building Frontend assets...")

    if args.deploy or run_all:
        run_command("cd frontend && firebase deploy --only hosting", "4/4: Deploying to Firebase...")

    end_time = time.time()
    print(f"\n{Colors.OKGREEN}{Colors.BOLD}✅ [SUCCESS]{Colors.ENDC} Completed in {int(end_time - start_time)}s\n")

if __name__ == "__main__":
    main()
