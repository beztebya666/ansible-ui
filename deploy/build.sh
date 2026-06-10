#!/usr/bin/env bash
# Build the two ansible-ui images (api — serves the UI too — and runner).
set -euo pipefail
cd "$(dirname "$0")/.."

DOCKER="${DOCKER:-docker}"
TAG="${TAG:-latest}"
REGISTRY="${REGISTRY:-ansible-ui}"

echo "==> building $REGISTRY/api:$TAG"
$DOCKER build -f deploy/api.Dockerfile -t "$REGISTRY/api:$TAG" .

echo "==> building $REGISTRY/runner:$TAG"
$DOCKER build -f deploy/runner.Dockerfile -t "$REGISTRY/runner:$TAG" .

echo "done."
