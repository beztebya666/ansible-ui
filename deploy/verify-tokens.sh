#!/usr/bin/env bash
# Verify API token auth: create a token, then call the guarded API with
# Authorization: Bearer (no session cookie), including launching a run.
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

TOK=$(c -X POST "$API/api/tokens" -H 'Content-Type: application/json' -d '{"name":"ci-test"}' | g "['token']")
[ -n "$TOK" ] && pass "token created (plaintext returned once): ${TOK:0:12}…" || fail "token create failed"

code=$(curl -s -o /dev/null -w '%{http_code}' -H "Authorization: Bearer $TOK" "$API/api/projects")
[ "$code" = 200 ] && pass "Bearer token authenticates /api/projects (200, no cookie)" || fail "bearer auth got $code"

code=$(curl -s -o /dev/null -w '%{http_code}' -H "Authorization: Bearer aui_bogus" "$API/api/projects")
[ "$code" = 401 ] && pass "invalid token rejected (401)" || fail "bad token got $code"

PID=$(curl -s -H "Authorization: Bearer $TOK" "$API/api/projects" | g "[0]['id']")
RID=$("$PY" -c "import json,sys;print(json.dumps({'projectId':sys.argv[1],'playbook':'playbooks/01-hello.yml'}))" "$PID" \
  | curl -s -H "Authorization: Bearer $TOK" -H 'Content-Type: application/json' -X POST "$API/api/runs" --data-binary @- | g "['id']")
[ -n "$RID" ] && pass "launched a run via Bearer token ($RID)" || fail "run via token failed"

n=$(c "$API/api/tokens" | "$PY" -c "import sys,json;print(len(json.load(sys.stdin)))")
[ "$n" -ge 1 ] && pass "token listed for the user" || fail "token not listed"
printf '\033[32mAPI TOKENS OK\033[0m\n'
