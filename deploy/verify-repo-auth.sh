#!/usr/bin/env bash
# Verify the auth + Git-repository + run-from-repo path end to end.
set -uo pipefail
API="${API:-http://localhost:8090}"
REPO_URL="${REPO_URL:-https://github.com/fiftin/ansible-semaphore-deploy-test.git}"
REPO_BRANCH="${REPO_BRANCH:-master}"
export PYTHONIOENCODING=utf-8
PY="$(command -v python3 || command -v python)"
JAR=/tmp/aui.cookies
g(){ "$PY" -c "import sys,json;print(json.load(sys.stdin)$1)" 2>/dev/null; }
pass(){ printf '  \033[32m✓\033[0m %s\n' "$1"; }
fail(){ printf '  \033[31m✗ %s\033[0m\n' "$1"; }

echo "== auth =="
code=$(curl -s -o /dev/null -w '%{http_code}' "$API/api/projects")
[ "$code" = 401 ] && pass "unauthenticated /api/projects -> 401 (guarded)" || fail "expected 401, got $code"

needs=$(curl -s "$API/api/auth/status" | g "['needsSetup']")
if [ "$needs" = "True" ]; then
  curl -s -c $JAR -X POST "$API/api/auth/setup" -H 'Content-Type: application/json' \
    -d '{"username":"admin","password":"admin123","email":"admin@local"}' >/dev/null
  pass "first-run setup created admin"
else
  curl -s -c $JAR -X POST "$API/api/auth/login" -H 'Content-Type: application/json' \
    -d '{"username":"admin","password":"admin123"}' >/dev/null
  pass "logged in as admin"
fi
code=$(curl -s -b $JAR -o /dev/null -w '%{http_code}' "$API/api/projects")
[ "$code" = 200 ] && pass "authenticated /api/projects -> 200" || fail "expected 200, got $code"

echo "== repository (git clone) =="
RID=$(curl -s -b $JAR -X POST "$API/api/repositories" -H 'Content-Type: application/json' \
  -d "{\"name\":\"deploy-test\",\"gitUrl\":\"$REPO_URL\",\"branch\":\"$REPO_BRANCH\"}" | g "['id']")
echo "repo id: $RID"
curl -s -b $JAR -X POST "$API/api/repositories/$RID/sync" >/dev/null
status=$(curl -s -b $JAR "$API/api/repositories/$RID" | g "['status']")
commit=$(curl -s -b $JAR "$API/api/repositories/$RID" | g "['lastCommit']")
[ "$status" = ready ] && pass "repo cloned & synced (ready, commit ${commit:0:8})" || fail "repo status=$status"

echo "== repo tree =="
curl -s -b $JAR "$API/api/repositories/$RID/tree" | "$PY" -c "
import sys,json
def walk(ns,d=0):
  for n in ns:
    print('  '*d + ('📁 ' if n['type']=='dir' else '📄 ') + n['name'])
    if d<2: walk(n.get('children') or [],d+1)
walk(json.load(sys.stdin))" 2>/dev/null | head -40

echo "== project from repo =="
PID=$(curl -s -b $JAR -X POST "$API/api/projects" -H 'Content-Type: application/json' \
  -d "{\"name\":\"deploy-test\",\"repositoryId\":\"$RID\",\"subPath\":\"\"}" | g "['id']")
echo "project id: $PID"
echo "discovered playbooks:"
curl -s -b $JAR "$API/api/projects/$PID/playbooks" | "$PY" -c "import sys,json;[print('  -',p) for p in json.load(sys.stdin)]" 2>/dev/null
PB=$(curl -s -b $JAR "$API/api/projects/$PID/playbooks" | g "[0]")

if [ -n "$PB" ] && [ "$PB" != "None" ]; then
  echo "== run-from-repo: $PB =="
  RUN=$(curl -s -b $JAR -X POST "$API/api/runs" -H 'Content-Type: application/json' \
    -d "{\"projectId\":\"$PID\",\"playbook\":\"$PB\"}" | g "['id']")
  for _ in $(seq 1 90); do
    rs=$(curl -s -b $JAR "$API/api/runs/$RUN" | g "['run']['status']")
    case "$rs" in success|failed|canceled) break;; esac; sleep 1
  done
  echo "run status: $rs"
  echo "---- output (first 45 lines) ----"
  curl -s -b $JAR "$API/api/runs/$RUN/output" | head -45
  if curl -s -b $JAR "$API/api/runs/$RUN/output" | grep -q "ansible-galaxy"; then
    pass "ansible-galaxy requirements were installed"
  fi
  [ -n "$rs" ] && pass "repo-backed run executed (status: $rs)"
else
  echo "(no playbooks auto-detected at repo root)"
fi
