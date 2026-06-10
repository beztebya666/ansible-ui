#!/usr/bin/env bash
# Run every demo playbook through the API and assert each ends successfully.
set -uo pipefail
API="${API:-http://localhost:8090}"
export PYTHONIOENCODING=utf-8
PY="$(command -v python3 || command -v python)"
JAR=/tmp/aui.cookies
g(){ "$PY" -c "import sys,json;print(json.load(sys.stdin)$1)" 2>/dev/null; }
c(){ curl -s -b "$JAR" "$@"; }

# Authenticate (first-run setup, else login).
needs=$(curl -s "$API/api/auth/status" | g "['needsSetup']")
if [ "$needs" = "True" ]; then
  curl -s -c "$JAR" -X POST "$API/api/auth/setup" -H 'Content-Type: application/json' \
    -d '{"username":"admin","password":"admin123","email":"admin@local"}' >/dev/null
else
  curl -s -c "$JAR" -X POST "$API/api/auth/login" -H 'Content-Type: application/json' \
    -d '{"username":"admin","password":"admin123"}' >/dev/null
fi

PID=$(c "$API/api/projects" | g "[0]['id']")
fail=0

for pb in 01-hello 02-facts 03-variables-loops 04-files-and-templates 05-handlers \
          06-roles 07-blocks-rescue 08-async-parallel 09-vault 10-full-stack; do
  rid=$(c -X POST "$API/api/runs" -H 'Content-Type: application/json' \
        -d "{\"projectId\":\"$PID\",\"playbook\":\"playbooks/$pb.yml\"}" | g "['id']")
  for _ in $(seq 1 60); do
    st=$(c "$API/api/runs/$rid" | g "['run']['status']")
    case "$st" in success|failed|canceled) break;; esac
    sleep 1
  done
  ec=$(c "$API/api/runs/$rid" | g "['run']['exitCode']")
  if [ "$st" = success ]; then
    printf '  \033[32m✓\033[0m %-26s success (exit %s)\n' "$pb" "$ec"
  else
    printf '  \033[31m✗ %-26s %s (exit %s)\033[0m\n' "$pb" "$st" "$ec"
    fail=1
  fi
done
[ "$fail" = 0 ] && printf '\033[32mALL 10 PLAYBOOKS GREEN\033[0m\n' || { printf '\033[31mSOME FAILED\033[0m\n'; exit 1; }
