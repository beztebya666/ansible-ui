#!/usr/bin/env bash
# Verify the WebSocket upgrade succeeds both directly against the api and
# through the nginx reverse proxy (proves the Upgrade/Connection headers are
# forwarded correctly for the live terminal + events streams).
set -euo pipefail
API="${API:-http://localhost:8090}"
WEB="${WEB:-http://localhost:8080}"
KEY="dGhlIHNhbXBsZSBub25jZQ=="

check() {
  local label="$1" url="$2"
  # The socket stays open after a 101, so bound curl with -m and read only the
  # status line (head -1 closes the pipe, ending curl).
  local line
  line=$(curl -s -m 3 -i \
    -H "Connection: Upgrade" -H "Upgrade: websocket" \
    -H "Sec-WebSocket-Key: $KEY" -H "Sec-WebSocket-Version: 13" \
    "$url" 2>/dev/null | head -1 || true)
  if echo "$line" | grep -q "101"; then
    printf '  \033[32m✓\033[0m %s -> %s\n' "$label" "$(echo "$line" | tr -d '\r')"
  else
    printf '  \033[31m✗ %s -> "%s" (expected 101)\033[0m\n' "$label" "$(echo "$line" | tr -d '\r')"
    exit 1
  fi
}

echo "== websocket upgrade =="
check "api direct   /ws/events" "$API/ws/events"
check "nginx proxy  /ws/events" "$WEB/ws/events"
printf '\033[32mWEBSOCKET OK\033[0m\n'
