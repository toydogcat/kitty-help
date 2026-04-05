#!/bin/bash
# scripts/tunnel-manager.sh
# Replicated from my-claw for kitty-help

HOME_DIR=~
WORKSPACE_DIR="$HOME_DIR/.kitty-help/workspace"
LOG_FILE="$WORKSPACE_DIR/logs/cloudflared.log"
STATE_FILE="$WORKSPACE_DIR/state/webhook_line.url"
SIGNAL_FILE="$WORKSPACE_DIR/state/recreate.signal"

mkdir -p "$(dirname "$STATE_FILE")"
mkdir -p "$(dirname "$LOG_FILE")"

TARGET_URL=${TUNNEL_URL:-"http://localhost:3000"}

run_tunnel() {
    echo "🚀 Starting cloudflared tunnel to $TARGET_URL..."
    cloudflared tunnel --url "$TARGET_URL" --logfile "$LOG_FILE" --no-autoupdate &
    TUNNEL_PID=$!
}

# Initial start
run_tunnel

echo "👀 Monitoring for logs and recreate signals..."
while true; do
    if [ -f "$LOG_FILE" ]; then
        LATEST_URL=$(grep -o 'https://[a-zA-Z0-9-]*\.trycloudflare\.com' "$LOG_FILE" | tail -n 1)
        if [ -n "$LATEST_URL" ]; then
            CURRENT_FILE_URL=$(cat "$STATE_FILE" 2>/dev/null)
            if [ "$LATEST_URL" != "$CURRENT_FILE_URL" ]; then
                echo "$LATEST_URL" > "$STATE_FILE"
                echo "🔗 Updated Webhook URL: $LATEST_URL"
            fi
        fi
    fi

    if [ -f "$SIGNAL_FILE" ]; then
        echo "♻️ Recreate signal detected! Restarting tunnel..."
        rm "$SIGNAL_FILE"
        kill $TUNNEL_PID
        sleep 2
        cat /dev/null > "$LOG_FILE"
        run_tunnel
    fi
    sleep 5
done
