#!/usr/bin/env bash
# Verify the compose-free deployment: the api serves the embedded SPA and the
# API on the same port, then run the full auth + repo flow.
set -uo pipefail
API="${API:-http://localhost:8080}"
export API

echo "waiting for api…"
for _ in $(seq 1 40); do
  curl -fsS "$API/api/health" >/dev/null 2>&1 && { echo "api ready"; break; }
  sleep 2
done

echo "== single-container UI + API =="
code=$(curl -s -o /dev/null -w '%{http_code}' "$API/")
body=$(curl -s "$API/")
[ "$code" = 200 ] && printf '  \033[32m✓\033[0m GET / -> 200\n' || printf '  \033[31m✗ GET / -> %s\033[0m\n' "$code"
echo "$body" | grep -q 'id="root"' && printf '  \033[32m✓\033[0m SPA index served by the api (no nginx)\n' || printf '  \033[31m✗ SPA index not served\033[0m\n'
asset=$(echo "$body" | grep -o '/assets/[^"]*\.js' | head -1)
if [ -n "$asset" ]; then
  acode=$(curl -s -o /dev/null -w '%{http_code}' "$API$asset")
  [ "$acode" = 200 ] && printf '  \033[32m✓\033[0m hashed asset served (%s -> 200)\n' "$asset" || printf '  \033[31m✗ asset %s -> %s\033[0m\n' "$asset" "$acode"
fi
hcode=$(curl -s -o /dev/null -w '%{http_code}' "$API/api/health")
[ "$hcode" = 200 ] && printf '  \033[32m✓\033[0m /api/health on the same port -> 200\n' || printf '  \033[31m✗ /api/health -> %s\033[0m\n' "$hcode"

echo
echo "== full auth + repository flow =="
bash deploy/verify-repo-auth.sh
