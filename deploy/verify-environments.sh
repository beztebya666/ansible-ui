#!/usr/bin/env bash
# Verify Environments (extra-vars + process env vars) and Survey variables by
# running a template whose playbook echoes all three, then asserting the output.
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

# auth
needs=$(curl -s "$API/api/auth/status" | g "['needsSetup']")
if [ "$needs" = "True" ]; then
  curl -s -c "$JAR" -X POST "$API/api/auth/setup" -H 'Content-Type: application/json' -d '{"username":"admin","password":"admin123"}' >/dev/null
else
  curl -s -c "$JAR" -X POST "$API/api/auth/login" -H 'Content-Type: application/json' -d '{"username":"admin","password":"admin123"}' >/dev/null
fi
PID=$(c "$API/api/projects" | g "[0]['id']")
echo "demo project = $PID"

echo "== write a test playbook into the demo project =="
PBCONTENT=$(cat <<'YAML'
---
- name: Environment + survey verification
  hosts: localhost
  gather_facts: false
  tasks:
    - debug: { msg: "EXTRAVAR env_marker={{ env_marker | default('UNSET') }}" }
    - debug: { msg: "EXTRAVAR survey_marker={{ survey_marker | default('UNSET') }}" }
    - debug: { msg: "PROCENV DEMO_ENV_VAR={{ lookup('env', 'DEMO_ENV_VAR') }}" }
YAML
)
"$PY" -c "import json,sys;print(json.dumps({'path':'playbooks/zz-envtest.yml','content':sys.argv[1]}))" "$PBCONTENT" \
  | c -X PUT "$API/api/projects/$PID/file" -H 'Content-Type: application/json' --data-binary @- >/dev/null
pass "wrote playbooks/zz-envtest.yml"

echo "== create environment (extra-vars + process env) =="
EID=$("$PY" -c "import json;print(json.dumps({'name':'verify-env','description':'','extraVars':{'env_marker':'EXTRA_FROM_ENV'},'envVars':{'DEMO_ENV_VAR':'PROC_FROM_ENV'}}))" \
  | c -X POST "$API/api/environments" -H 'Content-Type: application/json' --data-binary @- | g "['id']")
[ -n "$EID" ] && pass "environment created ($EID)" || fail "environment create failed"

echo "== create template with survey var + environment =="
TID=$("$PY" -c "import json,sys;print(json.dumps({'projectId':sys.argv[1],'name':'verify-tpl','playbook':'playbooks/zz-envtest.yml','environmentId':sys.argv[2],'surveyVars':[{'name':'survey_marker','title':'Marker','type':'text','required':False,'default':'SURVEY_DEFAULT'}],'extraVars':{},'diff':True}))" "$PID" "$EID" \
  | c -X POST "$API/api/templates" -H 'Content-Type: application/json' --data-binary @- | g "['id']")
[ -n "$TID" ] && pass "template created ($TID)" || fail "template create failed"

echo "== run template with survey answer =="
RID=$("$PY" -c "import json;print(json.dumps({'extraVars':{'survey_marker':'SURVEY_FROM_RUN'}}))" \
  | c -X POST "$API/api/templates/$TID/run" -H 'Content-Type: application/json' --data-binary @- | g "['id']")
for _ in $(seq 1 40); do
  st=$(c "$API/api/runs/$RID" | g "['run']['status']")
  case "$st" in success|failed|canceled) break;; esac; sleep 1
done
echo "run status: $st"
OUT=$(c "$API/api/runs/$RID/output")
echo "$OUT" | grep -q "EXTRAVAR env_marker=EXTRA_FROM_ENV"   && pass "environment extra-var applied"      || fail "env extra-var missing"
echo "$OUT" | grep -q "EXTRAVAR survey_marker=SURVEY_FROM_RUN" && pass "survey answer applied"             || fail "survey answer missing"
echo "$OUT" | grep -q "PROCENV DEMO_ENV_VAR=PROC_FROM_ENV"   && pass "environment process env var applied" || fail "env process var missing"
echo "run.extraVars (merged):"
c "$API/api/runs/$RID" | g "['run']['extraVars']"
[ "$st" = success ] && pass "run succeeded" || fail "run status $st"
printf '\033[32mENVIRONMENTS + SURVEYS OK\033[0m\n'
