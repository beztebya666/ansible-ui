#!/usr/bin/env bash
# Verify the multi-app execution backends end-to-end: log in, run the seeded
# Bash / Python / Terraform templates, and report status + tail of output.
set -uo pipefail
API="${API:-http://localhost:8080}"
JAR=/tmp/aui-ma.cookies
PY="$(command -v python3 || command -v python)"

echo "== auth status =="
curl -s "$API/api/auth/status"; echo

curl -s -c "$JAR" -X POST "$API/api/auth/login" -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}' >/tmp/login.json
echo "login: $(cat /tmp/login.json)"; echo

echo "== templates =="
curl -s -b "$JAR" "$API/api/templates" | "$PY" -c "import sys,json
for t in json.load(sys.stdin): print(' -', t['name'], '['+t.get('app','ansible')+(' '+t['action'] if t.get('action') else '')+']', '->', t['playbook'])"
echo

run_template() {
  app="$1"; action="${2:-}"
  name="$app${action:+ $action}"
  tid=$(curl -s -b "$JAR" "$API/api/templates" | "$PY" -c "import sys,json
ts=json.load(sys.stdin)
app=sys.argv[1]; action=sys.argv[2] if len(sys.argv)>2 else ''
print(next((t['id'] for t in ts if t.get('app')==app and (not action or t.get('action')==action)), ''))" "$app" "$action")
  if [ -z "$tid" ]; then echo "!! template app='$app' action='$action' not found"; return 1; fi
  echo "== run '$name' ($tid) =="
  rid=$(curl -s -b "$JAR" -X POST "$API/api/templates/$tid/run" -H 'Content-Type: application/json' -d '{}' | "$PY" -c "import sys,json;print(json.load(sys.stdin).get('id',''))")
  echo "run id: $rid"
  st=""; code=""
  for i in $(seq 1 40); do
    sleep 2
    line=$(curl -s -b "$JAR" "$API/api/runs/$rid" | "$PY" -c "import sys,json;r=json.load(sys.stdin)['run'];print(r['status'], r.get('exitCode'))")
    st=$(echo "$line" | cut -d' ' -f1)
    echo "  status: $line"
    case "$st" in success|failed|canceled) break;; esac
  done
  echo "--- output tail ---"
  curl -s -b "$JAR" "$API/api/runs/$rid/output" | tail -20
  echo "=== end '$name' ==="; echo
}

run_template bash
run_template python
run_template terraform plan
echo "DONE."
