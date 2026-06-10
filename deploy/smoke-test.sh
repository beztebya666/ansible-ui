#!/usr/bin/env bash
# End-to-end smoke test for the ansible-ui stack.
# Authenticates, then launches demo playbooks through the API and verifies they
# execute, stream, persist and parse their recap.
#
#   bash deploy/smoke-test.sh            # uses http://localhost:8090 (api) + :8080 (web)
set -uo pipefail

API="${API:-http://localhost:8090}"
WEB="${WEB:-http://localhost:8080}"
PY="$(command -v python3 || command -v python)"
export PYTHONIOENCODING=utf-8
JAR=/tmp/aui.cookies

jqget() { "$PY" -c "import sys,json;print(json.load(sys.stdin)$1)" 2>/dev/null; }
c() { curl -s -b "$JAR" "$@"; }
pass() { printf '  \033[32m✓\033[0m %s\n' "$1"; }
fail() { printf '  \033[31m✗ %s\033[0m\n' "$1"; exit 1; }

echo "== infrastructure =="
curl -fsS "$API/api/health" >/dev/null && pass "api healthy" || fail "api unhealthy"
code=$(curl -s -o /dev/null -w '%{http_code}' "$WEB/") ; [ "$code" = 200 ] && pass "web serving SPA (200)" || fail "web returned $code"

echo "== auth =="
code=$(curl -s -o /dev/null -w '%{http_code}' "$API/api/projects")
[ "$code" = 401 ] && pass "API is guarded (401 without session)" || fail "API not guarded (got $code)"
needs=$(curl -s "$API/api/auth/status" | jqget "['needsSetup']")
if [ "$needs" = "True" ]; then
  curl -s -c "$JAR" -X POST "$API/api/auth/setup" -H 'Content-Type: application/json' \
    -d '{"username":"admin","password":"admin123","email":"admin@local"}' >/dev/null
  pass "first-run setup created admin"
else
  curl -s -c "$JAR" -X POST "$API/api/auth/login" -H 'Content-Type: application/json' \
    -d '{"username":"admin","password":"admin123"}' >/dev/null
  pass "logged in"
fi
code=$(c -o /dev/null -w '%{http_code}' "$API/api/projects") ; [ "$code" = 200 ] && pass "authenticated access OK" || fail "auth failed ($code)"

echo "== seed =="
PID=$(c "$API/api/projects" | jqget "[0]['id']")
[ -n "$PID" ] && pass "demo project present ($PID)" || fail "no seeded project"
ntpl=$(c "$API/api/templates" | "$PY" -c "import sys,json;print(len(json.load(sys.stdin)))")
[ "$ntpl" -ge 1 ] && pass "seeded $ntpl templates" || fail "no templates seeded"

run_playbook() {
  local pb="$1" expect="$2" rid status out
  rid=$(c -X POST "$API/api/runs" -H 'Content-Type: application/json' \
        -d "{\"projectId\":\"$PID\",\"playbook\":\"$pb\"}" | jqget "['id']")
  for _ in $(seq 1 40); do
    status=$(c "$API/api/runs/$rid" | jqget "['run']['status']")
    case "$status" in success|failed|canceled) break;; esac
    sleep 1
  done
  if [ "$status" = "$expect" ]; then
    pass "$pb -> $status"
  else
    fail "$pb -> $status (expected $expect)"
  fi
  out=$(c "$API/api/runs/$rid/output")
  echo "$out" | grep -q "PLAY RECAP" && pass "    output persisted with PLAY RECAP" || fail "    no recap in stored output"
}

echo "== executing playbooks =="
run_playbook "playbooks/01-hello.yml"           success
run_playbook "playbooks/06-roles.yml"           success
run_playbook "playbooks/07-blocks-rescue.yml"   success
run_playbook "playbooks/10-full-stack.yml"      success

echo
printf '\033[32mALL SMOKE TESTS PASSED\033[0m\n'
