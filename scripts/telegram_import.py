import json
import psycopg2
import uuid
from datetime import datetime

# Database Connection Details
DB_CONFIG = {
    "dbname": "kitty_local",
    "user": "postgres",
    "password": "password",
    "host": "localhost",
    "port": 5432
}

def import_history(json_file):
    print(f"Reading {json_file}...")
    with open(json_file, 'r', encoding='utf-8') as f:
        data = json.load(f)

    conn = psycopg2.connect(**DB_CONFIG)
    cur = conn.cursor()

    count = 0
    # Telegram Export JSON structure: data['messages']
    for msg in data.get('messages', []):
        # We only care about media messages or certain types
        # This is a simplified filter
        if 'photo' in msg or 'file' in msg or 'video' in msg:
            msg_id = str(msg.get('id'))
            media_type = 'photo' if 'photo' in msg else ('video' if 'video' in msg else 'file')
            caption = msg.get('text', '')
            if isinstance(caption, list):
                caption = "".join([t if isinstance(t, str) else t.get('text', '') for t in caption])
            
            sender = msg.get('from', 'Unknown')
            timestamp = datetime.fromisoformat(msg.get('date'))
            
            # Use file_id placeholder if not present in export
            file_id = msg.get('file', 'exported_static_file')
            
            cur.execute("""
                INSERT INTO media_archives 
                (id, file_id, message_id, media_type, caption, source_platform, sender_name, created_at)
                VALUES (%s, %s, %s, %s, %s, %s, %s, %s)
                ON CONFLICT (file_id) DO NOTHING
            """, (
                str(uuid.uuid4()), 
                f"export_{msg_id}", 
                msg_id, 
                media_type, 
                caption, 
                'telegram_export', 
                sender, 
                timestamp
            ))
            count += 1

    conn.commit()
    cur.close()
    conn.close()
    print(f"Successfully imported {count} items into media_archives!")

if __name__ == "__main__":
    import sys
    if len(sys.argv) < 2:
        print("Usage: python telegram_import.py <result.json>")
        print("Note: You can get result.json by using 'Export Chat History' -> 'JSON' in Telegram Desktop.")
    else:
        try:
            import_history(sys.argv[1])
        except Exception as e:
            print(f"Error: {e}")
            print("Make sure you have psycopg2 installed: pip install psycopg2-binary")
