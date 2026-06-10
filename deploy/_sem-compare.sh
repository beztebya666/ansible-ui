#!/usr/bin/env bash
# Publish our box-art hello playbook over a local git daemon and drive Semaphore's
# API to clone + run it, so we can screenshot Semaphore rendering the SAME play.
set -uo pipefail
SEM="http://localhost:3000"
JAR=/tmp/sem.cookies
HOSTIP=192.168.1.25
PORT=9418
PY="$(command -v python3 || command -v python)"

say() { echo "==> $*"; }

# ---------- 1. build the repo ----------
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
          Hello from {{ inventory_hostname }} - ansible-ui is wired up
          correctly. Try the other playbooks for loops, roles, vault and more.

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
git init -q
git config user.email a@a && git config user.name a
git add -A && git commit -qm box
touch /tmp/semrepo/boxdemo/.git/git-daemon-export-ok

# ---------- 2. git daemon ----------
pkill -f 'git daemon' 2>/dev/null
git daemon --reuseaddr --base-path=/tmp/semrepo --export-all --port=$PORT --detach 2>/dev/null
sleep 1
say "git daemon up; test clone:"
rm -rf /tmp/cltest && git clone -q git://$HOSTIP:$PORT/boxdemo /tmp/cltest && echo "  clone OK" || echo "  clone FAILED"

# ---------- 3. Semaphore API ----------
curl -s -c "$JAR" -X POST "$SEM/api/auth/login" -H 'Content-Type: application/json' \
  -d '{"auth":"admin","password":"changeme"}' >/dev/null
PID=$(curl -s -b "$JAR" "$SEM/api/projects" | "$PY" -c 'import sys,json;d=json.load(sys.stdin);print(d[0]["id"])')
say "project=$PID"

api() { # method path json
  curl -s -b "$JAR" -X "$1" "$SEM/api$2" -H 'Content-Type: application/json' -d "$3"
}

KEY=$(api POST "/project/$PID/keys" "{\"name\":\"none-$RANDOM\",\"type\":\"none\",\"project_id\":$PID}")
echo "key resp: $KEY"
KID=$(echo "$KEY" | "$PY" -c 'import sys,json;print(json.load(sys.stdin)["id"])')

REPO=$(api POST "/project/$PID/repositories" "{\"name\":\"boxdemo-$RANDOM\",\"project_id\":$PID,\"git_url\":\"git://$HOSTIP:$PORT/boxdemo\",\"git_branch\":\"master\",\"ssh_key_id\":$KID}")
echo "repo resp: $REPO"
RID=$(echo "$REPO" | "$PY" -c 'import sys,json;print(json.load(sys.stdin)["id"])')

INV=$(api POST "/project/$PID/inventory" "{\"name\":\"local-$RANDOM\",\"project_id\":$PID,\"type\":\"static\",\"inventory\":\"localhost ansible_connection=local\",\"ssh_key_id\":$KID,\"become_key_id\":$KID}")
echo "inv resp: $INV"
IID=$(echo "$INV" | "$PY" -c 'import sys,json;print(json.load(sys.stdin)["id"])')

ENV=$(api POST "/project/$PID/environment" "{\"name\":\"empty-$RANDOM\",\"project_id\":$PID,\"json\":\"{}\",\"env\":\"{}\"}")
echo "env resp: $ENV"
EID=$(echo "$ENV" | "$PY" -c 'import sys,json;print(json.load(sys.stdin)["id"])')

TPL=$(api POST "/project/$PID/templates" "{\"project_id\":$PID,\"name\":\"box-$RANDOM\",\"app\":\"ansible\",\"playbook\":\"hello.yml\",\"inventory_id\":$IID,\"repository_id\":$RID,\"environment_id\":$EID,\"type\":\"\",\"arguments\":\"[]\",\"allow_override_args_in_task\":false}")
echo "tpl resp: $TPL"
TID=$(echo "$TPL" | "$PY" -c 'import sys,json;print(json.load(sys.stdin)["id"])')

TASK=$(api POST "/project/$PID/tasks" "{\"project_id\":$PID,\"template_id\":$TID,\"debug\":false,\"dry_run\":false}")
echo "task resp: $TASK"
TKID=$(echo "$TASK" | "$PY" -c 'import sys,json;print(json.load(sys.stdin)["id"])')

say "waiting for task $TKID..."
for i in $(seq 1 40); do
  ST=$(curl -s -b "$JAR" "$SEM/api/project/$PID/tasks/$TKID" | "$PY" -c 'import sys,json;print(json.load(sys.stdin).get("status",""))')
  [ "$ST" = "success" ] || [ "$ST" = "error" ] || [ "$ST" = "stopped" ] && break
  sleep 1
done
say "task status=$ST"
echo "SEM_URL=$SEM/project/$PID/templates/$TID/tasks?t=$TKID"
echo "SEM_TASK=$TKID SEM_PROJECT=$PID SEM_TEMPLATE=$TID"
