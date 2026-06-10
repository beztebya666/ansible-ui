#!/usr/bin/env bash
# Run our box-art hello playbook in Semaphore via a file:// repo copied into the
# Semaphore container (avoids git-daemon networking). Prints the task URL.
set -uo pipefail
SEM="http://localhost:3000"
JAR=/tmp/sem.cookies
PY="$(command -v python3 || command -v python)"
C=ansible-semaphore
say() { echo "==> $*"; }

# ---------- repo ----------
rm -rf /tmp/semrepo && mkdir -p /tmp/semrepo/boxdemo
cat > /tmp/semrepo/boxdemo/hello.yml <<'YML'
---
- name: "Hello, Ansible"
  hosts: localhost
  gather_facts: false
  tasks:
    - name: Ping the control node
      ansible.builtin.ping:
    - name: Greet the operator
      ansible.builtin.debug:
        msg: >-
          Hello from {{ inventory_hostname }} - ansible-ui is wired up correctly.
    - name: Show a tiny banner
      ansible.builtin.debug:
        msg: "{{ item }}"
      loop:
        - "┌────────────────────────────────────────────┐"
        - "│   ansible-ui · the Ansible control plane     │"
        - "│   fast · minimal · native terminal           │"
        - "└────────────────────────────────────────────┘"
YML
cd /tmp/semrepo/boxdemo
git init -q && git config user.email a@a && git config user.name a && git add -A && git commit -qm box

# ---------- copy into the Semaphore container, clone via file:// ----------
sudo docker exec "$C" rm -rf /repo-boxdemo 2>/dev/null
sudo docker cp /tmp/semrepo/boxdemo "$C":/repo-boxdemo
sudo docker exec "$C" sh -c "chmod -R 777 /repo-boxdemo; git config --system --add safe.directory '*' 2>/dev/null; git config --global --add safe.directory '*' 2>/dev/null; true"
say "in-container clone test:"
sudo docker exec "$C" sh -c "rm -rf /tmp/cl && git clone -q file:///repo-boxdemo /tmp/cl && echo '  clone OK' || echo '  clone FAIL'"

# ---------- Semaphore API ----------
curl -s -c "$JAR" -X POST "$SEM/api/auth/login" -H 'Content-Type: application/json' \
  -d '{"auth":"admin","password":"changeme"}' >/dev/null
PID=$(curl -s -b "$JAR" "$SEM/api/projects" | "$PY" -c 'import sys,json;print(json.load(sys.stdin)[0]["id"])')
api() { curl -s -b "$JAR" -X "$1" "$SEM/api$2" -H 'Content-Type: application/json' -d "$3"; }

KID=$(api POST "/project/$PID/keys" "{\"name\":\"none-$RANDOM\",\"type\":\"none\",\"project_id\":$PID}" | "$PY" -c 'import sys,json;print(json.load(sys.stdin)["id"])')
RID=$(api POST "/project/$PID/repositories" "{\"name\":\"boxf-$RANDOM\",\"project_id\":$PID,\"git_url\":\"file:///repo-boxdemo\",\"git_branch\":\"master\",\"ssh_key_id\":$KID}" | "$PY" -c 'import sys,json;print(json.load(sys.stdin)["id"])')
IID=$(api POST "/project/$PID/inventory" "{\"name\":\"local-$RANDOM\",\"project_id\":$PID,\"type\":\"static\",\"inventory\":\"localhost ansible_connection=local\",\"ssh_key_id\":$KID,\"become_key_id\":$KID}" | "$PY" -c 'import sys,json;print(json.load(sys.stdin)["id"])')
EID=$(api POST "/project/$PID/environment" "{\"name\":\"empty-$RANDOM\",\"project_id\":$PID,\"json\":\"{}\",\"env\":\"{}\"}" | "$PY" -c 'import sys,json;print(json.load(sys.stdin)["id"])')
TID=$(api POST "/project/$PID/templates" "{\"project_id\":$PID,\"name\":\"boxf-$RANDOM\",\"app\":\"ansible\",\"playbook\":\"hello.yml\",\"inventory_id\":$IID,\"repository_id\":$RID,\"environment_id\":$EID,\"type\":\"\",\"arguments\":\"[]\",\"allow_override_args_in_task\":false}" | "$PY" -c 'import sys,json;print(json.load(sys.stdin)["id"])')
TKID=$(api POST "/project/$PID/tasks" "{\"project_id\":$PID,\"template_id\":$TID,\"debug\":false,\"dry_run\":false}" | "$PY" -c 'import sys,json;print(json.load(sys.stdin)["id"])')
say "project=$PID template=$TID task=$TKID"

for i in $(seq 1 40); do
  ST=$(curl -s -b "$JAR" "$SEM/api/project/$PID/tasks/$TKID" | "$PY" -c 'import sys,json;print(json.load(sys.stdin).get("status",""))')
  [ "$ST" = "success" ] || [ "$ST" = "error" ] || [ "$ST" = "stopped" ] && break
  sleep 1
done
say "status=$ST"
echo "SEM_URL=$SEM/project/$PID/templates/$TID/tasks?t=$TKID"
echo "SEM_VIEW=/project/$PID/templates/$TID/tasks?t=$TKID"
