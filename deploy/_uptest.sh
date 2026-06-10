#!/usr/bin/env bash
set -uo pipefail
API=http://localhost:8080; J=/tmp/up.jar; rm -f "$J"
curl -s -c "$J" -X POST "$API/api/auth/login" -H 'Content-Type: application/json' -d '{"username":"admin","password":"admin123"}' >/dev/null
# build a tar.gz and a zip on the VM
rm -rf /tmp/upsrc; mkdir -p /tmp/upsrc/playbooks
printf -- "---\n- hosts: localhost\n  tasks: [{debug: {msg: tgz-ok}}]\n" > /tmp/upsrc/playbooks/a.yml
printf "[local]\nlocalhost ansible_connection=local\n" > /tmp/upsrc/inventory.ini
tar czf /tmp/up.tgz -C /tmp/upsrc .
python -c "import zipfile,os;z=zipfile.ZipFile('/tmp/up.zip','w',zipfile.ZIP_DEFLATED);[z.write(os.path.join(r,f),os.path.relpath(os.path.join(r,f),'/tmp/upsrc')) for r,_,fs in os.walk('/tmp/upsrc') for f in fs];z.close()"
for kind in tgz zip; do
  PID=$(curl -s -b "$J" -X POST "$API/api/projects" -H 'Content-Type: application/json' -d "{\"name\":\"up-$kind-$RANDOM\"}" | python -c "import sys,json;print(json.load(sys.stdin)['id'])")
  R=$(curl -s -b "$J" -X POST "$API/api/projects/$PID/upload" -F "file=@/tmp/up.$kind" -w ' [http %{http_code}]')
  PB=$(curl -s -b "$J" "$API/api/projects/$PID/playbooks")
  echo "$kind -> $R | playbooks=$PB"
  curl -s -b "$J" -X DELETE "$API/api/projects/$PID" -o /dev/null
done
