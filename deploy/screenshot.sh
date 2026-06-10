#!/usr/bin/env bash
# Capture screenshots of the running UI with headless Chromium (run with sudo:
# docker needs root on this host). Writes PNGs to .shots/ on the mount.
set -euo pipefail
API="${API:-http://localhost:8090}"
WEB="${WEB:-http://localhost:8080}"
OUT=/mnt/hgfs/SharedLibrary/ansible-ui/.shots
mkdir -p "$OUT"; chmod 777 "$OUT"
export PYTHONIOENCODING=utf-8
PY=$(command -v python3 || command -v python)

RID=$(curl -fsS "$API/api/runs?limit=30" | "$PY" -c "
import sys,json
rs=[r for r in json.load(sys.stdin) if r['playbook'].endswith('10-full-stack.yml') and r['status']=='success']
print(rs[0]['id'] if rs else '')")

shoot() { # file url
  docker run --rm --network host -v "$OUT:/out" zenika/alpine-chrome \
    --no-sandbox --disable-gpu --hide-scrollbars --window-size=1600,1000 \
    --virtual-time-budget=9000 --screenshot=/out/"$1" "$2" >/dev/null 2>&1 || true
}

echo "dashboard…"; shoot dashboard.png "$WEB/"
echo "runs…";      shoot runs.png      "$WEB/runs"
echo "templates…"; shoot templates.png "$WEB/templates"
if [ -n "$RID" ]; then echo "run terminal ($RID)…"; shoot run-terminal.png "$WEB/runs/$RID"; fi
ls -la "$OUT"
