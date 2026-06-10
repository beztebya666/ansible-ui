#!/usr/bin/env bash
# ansible-ui CLI — export / import a project's structure over the HTTP API, so a
# project bundle (inventories, templates, workflows, schedules — refs by name,
# secrets excluded) can be version-controlled, promoted between instances, or
# wired into CI. Thin wrapper over GET /api/projects/{id}/export + POST
# /api/projects/import.
#
#   export AUI_URL=https://ansible.example.com
#   export AUI_TOKEN=aui_xxx          # an API token (Settings → API tokens)
#
#   aui-cli.sh list                          # list projects (id + name)
#   aui-cli.sh export <projectId> > prod.json
#   aui-cli.sh import < prod.json            # creates "<name> (imported)"
#   aui-cli.sh export <projectId> | aui-cli.sh --url https://staging … import
set -euo pipefail

URL="${AUI_URL:-http://localhost:8080}"
TOKEN="${AUI_TOKEN:-}"

# Allow --url / --token to override the env per invocation.
args=()
while [ $# -gt 0 ]; do
  case "$1" in
    --url) URL="$2"; shift 2 ;;
    --token) TOKEN="$2"; shift 2 ;;
    *) args+=("$1"); shift ;;
  esac
done
set -- "${args[@]+"${args[@]}"}"

hdr=()
[ -n "$TOKEN" ] && hdr=(-H "Authorization: Bearer $TOKEN")

api() { curl -fsS "${hdr[@]+"${hdr[@]}"}" "$@"; }

case "${1:-}" in
  list)
    api "$URL/api/projects" ;;
  export)
    pid="${2:?usage: aui-cli.sh export <projectId>}"
    api "$URL/api/projects/$pid/export" ;;
  import)
    api -H "Content-Type: application/json" -X POST "$URL/api/projects/import" --data-binary @- ;;
  *)
    echo "usage: AUI_URL=… AUI_TOKEN=… $0 {list | export <projectId> | import < bundle.json}" >&2
    exit 1 ;;
esac
