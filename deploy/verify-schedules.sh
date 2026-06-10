#!/usr/bin/env bash
# Verify the scheduler fires a template run on a cron expression.
set -uo pipefail
API="${API:-http://localhost:8080}"
export PYTHONIOENCODING=utf-8
PY="$(command -v python3 || command -v python)"
JAR=/tmp/aui.cookies
g(){ "$PY" -c "import sys,json;print(json.load(sys.stdin)$1)" 2>/dev/null; }
c(){ curl -s -b "$JAR" "$@"; }
count_sched(){ c "$API/api/runs?limit=300" | "$PY" -c "import sys,json;print(len([r for r in json.load(sys.stdin) if r['name'].endswith('(scheduled)')]))"; }
pass(){ printf '  \033[32m✓\033[0m %s\n' "$1"; }
fail(){ printf '  \033[31m✗ %s\033[0m\n' "$1"; exit 1; }

for _ in $(seq 1 40); do curl -fsS "$API/api/health" >/dev/null 2>&1 && break; sleep 2; done
needs=$(curl -s "$API/api/auth/status" | g "['needsSetup']")
if [ "$needs" = "True" ]; then
  curl -s -c "$JAR" -X POST "$API/api/auth/setup" -H 'Content-Type: application/json' -d '{"username":"admin","password":"admin123"}' >/dev/null
else
  curl -s -c "$JAR" -X POST "$API/api/auth/login" -H 'Content-Type: application/json' -d '{"username":"admin","password":"admin123"}' >/dev/null
fi

PID=$(c "$API/api/projects" | g "[0]['id']")
TID=$("$PY" -c "import json,sys;print(json.dumps({'projectId':sys.argv[1],'name':'sched-hello','playbook':'playbooks/01-hello.yml','extraVars':{}}))" "$PID" \
  | c -X POST "$API/api/templates" -H 'Content-Type: application/json' --data-binary @- | g "['id']")
[ -n "$TID" ] && pass "template created ($TID)" || fail "template create failed"

before=$(count_sched)
echo "scheduled runs before: $before"

SID=$("$PY" -c "import json,sys;print(json.dumps({'templateId':sys.argv[1],'name':'every-min','cron':'* * * * *'}))" "$TID" \
  | c -X POST "$API/api/schedules" -H 'Content-Type: application/json' --data-binary @- | g "['id']")
NEXT=$(c "$API/api/schedules/$SID" | g "['nextRunAt']")
[ -n "$SID" ] && pass "schedule created (nextRunAt=$NEXT)" || fail "schedule create failed"

echo "waiting up to 100s for the scheduler to fire…"
fired=0
for i in $(seq 1 20); do
  sleep 5
  now=$(count_sched)
  if [ "$now" -gt "$before" ]; then fired=1; echo "  scheduled run appeared after ~$((i*5))s"; break; fi
done
c -X DELETE "$API/api/schedules/$SID" >/dev/null   # stop it firing
[ "$fired" = 1 ] && pass "scheduler fired a run on cron" || fail "no scheduled run appeared within 100s"
pass "schedule deleted (cleanup)"
printf '\033[32mSCHEDULES OK\033[0m\n'
