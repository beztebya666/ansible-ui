#!/usr/bin/env bash
# Inspect a run's metadata + output. Usage: bash deploy/inspect-run.sh <runId>
set -uo pipefail
API="${API:-http://localhost:8080}"
export PYTHONIOENCODING=utf-8
PY="$(command -v python3 || command -v python)"
JAR=/tmp/aui.cookies
curl -s -c "$JAR" -X POST "$API/api/auth/login" -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}' >/dev/null
RID="${1:?run id required}"
echo "=== run meta ==="
curl -s -b "$JAR" "$API/api/runs/$RID" | "$PY" -c "import sys,json;d=json.load(sys.stdin)['run'];print('status=%s exit=%s\nplaybook=%s\nstats=%s'%(d['status'],d['exitCode'],d['playbook'],d['stats']))"
echo "=== output (last 45 lines) ==="
curl -s -b "$JAR" "$API/api/runs/$RID/output" | tail -45
