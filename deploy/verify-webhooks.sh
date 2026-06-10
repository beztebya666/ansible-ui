#!/usr/bin/env bash
# Verify inbound webhooks: an unauthenticated POST to the token URL triggers a run.
set -uo pipefail
API="${API:-http://localhost:8080}"
export PYTHONIOENCODING=utf-8
PY="$(command -v python3 || command -v python)"
JAR=/tmp/aui.cookies
g(){ "$PY" -c "import sys,json;print(json.load(sys.stdin)$1)" 2>/dev/null; }
c(){ curl -s -b "$JAR" "$@"; }
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
TID=$("$PY" -c "import json,sys;print(json.dumps({'projectId':sys.argv[1],'name':'webhook-hello','playbook':'playbooks/01-hello.yml','extraVars':{}}))" "$PID" \
  | c -X POST "$API/api/templates" -H 'Content-Type: application/json' --data-binary @- | g "['id']")
[ -n "$TID" ] && pass "template created ($TID)" || fail "template create failed"

TOKEN=$("$PY" -c "import json,sys;print(json.dumps({'templateId':sys.argv[1],'name':'ci'}))" "$TID" \
  | c -X POST "$API/api/integrations" -H 'Content-Type: application/json' --data-binary @- | g "['token']")
[ -n "$TOKEN" ] && pass "webhook created (${TOKEN:0:12}…)" || fail "webhook create failed"

# Unauthenticated POST (no cookie, no token header) to the webhook URL.
resp=$(curl -s -X POST "$API/api/webhooks/$TOKEN")
RID=$(echo "$resp" | g "['runId']")
[ -n "$RID" ] && pass "unauthenticated webhook triggered run ($RID)" || fail "webhook trigger failed: $resp"

code=$(curl -s -o /dev/null -w '%{http_code}' "$API/api/projects")
[ "$code" = 401 ] && pass "other API endpoints still require auth (401)" || fail "API not guarded ($code)"

code=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$API/api/webhooks/whk_bogus")
[ "$code" = 404 ] && pass "unknown webhook token -> 404" || fail "bad token got $code"

for _ in $(seq 1 30); do
  st=$(c "$API/api/runs/$RID" | g "['run']['status']")
  case "$st" in success|failed|canceled) break;; esac; sleep 1
done
[ "$st" = success ] && pass "webhook-triggered run executed ($st)" || fail "run status $st"
printf '\033[32mWEBHOOKS OK\033[0m\n'
