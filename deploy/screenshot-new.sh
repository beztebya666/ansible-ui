#!/usr/bin/env bash
# Capture the multi-app / new-UI screenshots (desktop + mobile). Run with sudo.
set -uo pipefail
DIR=/mnt/hgfs/SharedLibrary/ansible-ui
OUT="$DIR/.shots"
mkdir -p "$OUT"; chmod 777 "$OUT"
docker pull zenika/alpine-chrome:with-puppeteer >/dev/null 2>&1 || true
docker run --rm --network host -e NODE_PATH=/usr/src/app/node_modules \
  -v "$DIR/deploy:/app" -v "$OUT:/out" \
  --entrypoint node zenika/alpine-chrome:with-puppeteer /app/shoot2.js
ls -la "$OUT"
