#!/usr/bin/env bash
set -euo pipefail
API="${API:-http://localhost:8090}"
export PYTHONIOENCODING=utf-8
PY="$(command -v python3 || command -v python)"
# Print the output of the most recent failed run whose playbook matches $1.
pat="${1:-}"
ids=$(curl -fsS "$API/api/runs?limit=40" | "$PY" -c "
import sys,json
for r in json.load(sys.stdin):
    if r['status']=='failed' and ('$pat' in r['playbook']):
        print(r['id']+'\t'+r['playbook']); break
")
echo "run: $ids"
rid=$(echo "$ids" | cut -f1)
[ -n "$rid" ] && curl -fsS "$API/api/runs/$rid/output" | sed -n '1,80p'
