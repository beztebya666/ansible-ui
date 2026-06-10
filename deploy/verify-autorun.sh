#!/usr/bin/env bash
# E2E check for auto-run-on-commit: a repo-backed template with autorunOnCommit
# fires automatically when the backing repo gets a new commit.
set -uo pipefail
API="${API:-http://localhost:8080}"
J=/tmp/ar.jar; rm -f "$J"
curl -s -c "$J" -X POST "$API/api/auth/login" -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}' >/dev/null

# 1) bare repo + an initial commit, on the shared /data volume so the runner
#    (which performs git sync) can clone it via file://.
sudo docker exec ansible-ui-runner sh -lc '
  set -e
  git config --global init.defaultBranch master
  git config --global user.email a@b.c; git config --global user.name autorun
  git config --global protocol.file.allow always
  rm -rf /data/autorun-test.git /tmp/arw
  git init --bare -q -b master /data/autorun-test.git
  git clone -q /data/autorun-test.git /tmp/arw
  cd /tmp/arw
  printf "[local]\nlocalhost ansible_connection=local\n" > inventory.ini
  printf -- "---\n- hosts: localhost\n  gather_facts: false\n  tasks:\n    - debug: msg=autorun v1\n" > play.yml
  git add -A; git commit -qm v1; git push -q origin master
' || { echo "FAIL: bare repo setup"; exit 1; }
echo "bare repo + v1 ready"

RID=$(curl -s -b "$J" -X POST "$API/api/repositories" -H 'Content-Type: application/json' \
  -d '{"name":"autorun-test","gitUrl":"file:///data/autorun-test.git","branch":"master"}' \
  | python -c 'import sys,json;print(json.load(sys.stdin)["id"])')
echo "repo=$RID"
echo -n "baseline sync: "; curl -s -b "$J" -X POST "$API/api/repositories/$RID/sync" \
  | python -c 'import sys,json;d=json.load(sys.stdin);print("commit",d.get("commit","")[:8],"err",d.get("repository",{}).get("error",""))'

PID=$(curl -s -b "$J" -X POST "$API/api/projects" -H 'Content-Type: application/json' \
  -d "{\"name\":\"Autorun Demo\",\"repositoryId\":\"$RID\"}" \
  | python -c 'import sys,json;print(json.load(sys.stdin)["id"])')
echo "project=$PID"
TID=$(curl -s -b "$J" -X POST "$API/api/templates" -H 'Content-Type: application/json' \
  -d "{\"projectId\":\"$PID\",\"name\":\"autorun-play\",\"app\":\"ansible\",\"playbook\":\"play.yml\",\"autorunOnCommit\":true}" \
  | python -c 'import sys,json;print(json.load(sys.stdin)["id"])')
echo "template=$TID (autorunOnCommit=true)"

count() { curl -s -b "$J" "$API/api/runs" | python -c "import sys,json;print(sum(1 for r in json.load(sys.stdin) if r.get('templateId')=='$TID'))"; }
before=$(count); echo "runs before push=$before"

# 2) push a NEW commit
sudo docker exec ansible-ui-runner sh -lc '
  cd /tmp/arw
  printf -- "---\n- hosts: localhost\n  gather_facts: false\n  tasks:\n    - debug: msg=autorun v2\n" > play.yml
  git add -A; git commit -qm v2; git push -q origin master
' || { echo "FAIL: push v2"; exit 1; }
echo "pushed v2 — waiting for the autorun poll (<=~110s)…"

after=$before
for i in $(seq 1 55); do
  after=$(count)
  if [ "$after" -gt "$before" ]; then echo "AUTORUN FIRED after ~$((i*2))s: runs $before -> $after"; break; fi
  sleep 2
done
echo "=== template runs ==="
curl -s -b "$J" "$API/api/runs" | python -c "import sys,json;[print(' ',r['id'],'by',r.get('triggeredBy'),r.get('status')) for r in json.load(sys.stdin) if r.get('templateId')=='$TID']"
[ "$after" -gt "$before" ] && echo "RESULT: PASS" || echo "RESULT: FAIL (no autorun within timeout)"
