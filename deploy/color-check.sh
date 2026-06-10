#!/usr/bin/env bash
# Confirm runs capture real ANSI colour (proves the PTY + force-color path).
set -euo pipefail
API="${API:-http://localhost:8090}"
export PYTHONIOENCODING=utf-8
PY="$(command -v python3 || command -v python)"
RID=$(curl -fsS "$API/api/runs?limit=1" | "$PY" -c "import sys,json;print(json.load(sys.stdin)[0]['id'])")
OUT=$(curl -fsS "$API/api/runs/$RID/output")
ESC=$(printf '%s' "$OUT" | grep -c $'\033' || true)
echo "latest run: $RID"
echo "lines with ANSI escapes: $ESC"
if [ "$ESC" -gt 0 ]; then
  printf '\033[32m✓ real ANSI colour captured (native terminal output)\033[0m\n'
else
  printf '\033[31m✗ no ANSI colour found\033[0m\n'; exit 1
fi
echo "--- sample (raw, with escapes shown) ---"
printf '%s' "$OUT" | grep $'\033' | head -6 | cat -v
