#!/usr/bin/env bash
# Ensure there's a fresh, finished run (with log_times) to screenshot the Log view.
set -euo pipefail
API="${API:-http://localhost:8080}"
J=/tmp/aui-cj.txt
rm -f "$J"

# Log in (admin/admin123 is the verify-script login).
curl -s -c "$J" -X POST "$API/api/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}' >/dev/null

# Pick the seeded demo project + its first ansible playbook template.
PID=$(curl -s -b "$J" "$API/api/projects" | python -c 'import sys,json;d=json.load(sys.stdin);print(d[0]["id"])')
TID=$(curl -s -b "$J" "$API/api/templates" | python -c 'import sys,json;d=json.load(sys.stdin);print([t for t in d if t.get("app","ansible")=="ansible"][0]["id"])')
echo "project=$PID template=$TID"

# Launch the template.
RID=$(curl -s -b "$J" -X POST "$API/api/templates/$TID/run" \
  -H 'Content-Type: application/json' -d '{}' \
  | python -c 'import sys,json;print(json.load(sys.stdin)["id"])')
echo "run=$RID"

# Wait for it to finish (max ~60s).
for i in $(seq 1 60); do
  ST=$(curl -s -b "$J" "$API/api/runs/$RID" | python -c 'import sys,json;print(json.load(sys.stdin)["run"]["status"])')
  [ "$ST" = "success" ] || [ "$ST" = "failed" ] || [ "$ST" = "canceled" ] && break
  sleep 1
done
echo "status=$ST"

# Show the log payload shape (output length + how many per-line timestamps).
curl -s -b "$J" "$API/api/runs/$RID/log" | python -c 'import sys,json;d=json.load(sys.stdin);print("output_bytes=%d times=%d"%(len(d["output"]),len(d["times"])))'
echo "RUN_ID=$RID"
