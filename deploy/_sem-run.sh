#!/usr/bin/env bash
# Fix repo perms/ownership inside the Semaphore container (as root), then trigger
# a fresh task on the existing box template and wait for it.
set -uo pipefail
SEM="http://localhost:3000"; JAR=/tmp/sem.cookies; C=ansible-semaphore
PY="$(command -v python3 || command -v python)"
PID="${1:-1}"; TID="${2:-11}"

sudo docker exec -u 0 "$C" sh -c "chmod -R 777 /repo-boxdemo; git config --system --add safe.directory '*'; git config --system --add safe.directory /repo-boxdemo; echo perms-set"

curl -s -c "$JAR" -X POST "$SEM/api/auth/login" -H 'Content-Type: application/json' -d '{"auth":"admin","password":"changeme"}' >/dev/null
TKID=$(curl -s -b "$JAR" -X POST "$SEM/api/project/$PID/tasks" -H 'Content-Type: application/json' \
  -d "{\"project_id\":$PID,\"template_id\":$TID,\"debug\":false,\"dry_run\":false}" \
  | "$PY" -c 'import sys,json;print(json.load(sys.stdin)["id"])')
echo "task=$TKID"
for i in $(seq 1 40); do
  ST=$(curl -s -b "$JAR" "$SEM/api/project/$PID/tasks/$TKID" | "$PY" -c 'import sys,json;print(json.load(sys.stdin).get("status",""))')
  [ "$ST" = "success" ] || [ "$ST" = "error" ] || [ "$ST" = "stopped" ] && break
  sleep 1
done
echo "status=$ST"
echo "SEM_VIEW=/project/$PID/templates/$TID/tasks?t=$TKID"
echo "---- output ----"
curl -s -b "$JAR" "$SEM/api/project/$PID/tasks/$TKID/output" | "$PY" -c 'import sys,json;d=json.load(sys.stdin);print(chr(10).join(o["output"] for o in d))' | head -50
