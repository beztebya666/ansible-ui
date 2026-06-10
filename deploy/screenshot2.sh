#!/usr/bin/env bash
# Log in, seed a couple of demo credentials, and capture the new pages.
# Run with sudo (docker needs root on this host).
set -uo pipefail
API="${API:-http://localhost:8080}"
JAR=/tmp/aui.cookies
DIR=/mnt/hgfs/SharedLibrary/ansible-ui
OUT="$DIR/.shots"
mkdir -p "$OUT"; chmod 777 "$OUT"
PY="$(command -v python3 || command -v python)"

curl -s -c "$JAR" -X POST "$API/api/auth/login" -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}' >/dev/null

n=$(curl -s -b "$JAR" "$API/api/credentials" | "$PY" -c "import sys,json;print(len(json.load(sys.stdin)))" 2>/dev/null || echo 0)
if [ "$n" = 0 ]; then
  curl -s -b "$JAR" -X POST "$API/api/credentials" -H 'Content-Type: application/json' \
    -d '{"name":"github-deploy-key","type":"ssh","login":"git","sshPrivateKey":"-----BEGIN OPENSSH PRIVATE KEY-----\nDEMO\n-----END OPENSSH PRIVATE KEY-----"}' >/dev/null
  curl -s -b "$JAR" -X POST "$API/api/credentials" -H 'Content-Type: application/json' \
    -d '{"name":"prod-vault","type":"vault","vaultPassword":"s3cret-demo"}' >/dev/null
  curl -s -b "$JAR" -X POST "$API/api/credentials" -H 'Content-Type: application/json' \
    -d '{"name":"bitbucket-login","type":"login_password","login":"ci-bot","password":"demo"}' >/dev/null
fi

docker pull zenika/alpine-chrome:with-puppeteer >/dev/null 2>&1 || true
docker run --rm --network host -e NODE_PATH=/usr/src/app/node_modules \
  -v "$DIR/deploy:/app" -v "$OUT:/out" \
  --entrypoint node zenika/alpine-chrome:with-puppeteer /app/shoot.js
ls -la "$OUT"
