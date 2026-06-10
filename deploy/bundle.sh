#!/usr/bin/env bash
# Build a single self-contained tarball for an AIR-GAPPED (offline) install:
# the api + runner + postgres images, the run script, an offline installer, the
# Helm chart, and a checksum/manifest. The resulting .tgz can be copied to a host
# with NO internet / registry access and installed with `bash install.sh`.
#
#   bash deploy/bundle.sh            → dist/ansible-ui-airgapped-<tag>-<date>.tgz
#   OUT=/var/tmp bash deploy/bundle.sh
#
# The ONLY step that needs network is pulling the postgres base image if it isn't
# already in the local Docker cache — do that once on a connected build host.
set -euo pipefail
cd "$(dirname "$0")/.."

DOCKER="${DOCKER:-docker}"
TAG="${TAG:-latest}"
REGISTRY="${REGISTRY:-ansible-ui}"
PG_IMAGE="${PG_IMAGE:-postgres:16-alpine}"
STAMP="$(date +%Y%m%d)"
OUT="${OUT:-dist}"
NAME="ansible-ui-airgapped-${TAG}-${STAMP}"
API_IMG="$REGISTRY/api:$TAG"
RUNNER_IMG="$REGISTRY/runner:$TAG"

echo "==> ensuring images are present"
if ! $DOCKER image inspect "$API_IMG" >/dev/null 2>&1 || ! $DOCKER image inspect "$RUNNER_IMG" >/dev/null 2>&1; then
  echo "    building app images via deploy/build.sh"
  TAG="$TAG" REGISTRY="$REGISTRY" DOCKER="$DOCKER" bash deploy/build.sh
fi
$DOCKER image inspect "$PG_IMAGE" >/dev/null 2>&1 || { echo "    pulling $PG_IMAGE"; $DOCKER pull "$PG_IMAGE"; }

STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT
B="$STAGE/$NAME"
mkdir -p "$B"

echo "==> saving images → images.tar.gz ($API_IMG + $RUNNER_IMG + $PG_IMAGE)"
$DOCKER save "$API_IMG" "$RUNNER_IMG" "$PG_IMAGE" | gzip > "$B/images.tar.gz"

echo "==> assembling bundle"
cp deploy/docker-run.sh "$B/docker-run.sh"
cp deploy/install.sh "$B/install.sh"
[ -d deploy/helm ] && cp -r deploy/helm "$B/helm"   # k8s air-gap: load images into the cluster's nodes/registry, then helm install

# manifest: exact image ids + sizes so an operator can confirm what they're loading
{
  echo "ansible-ui air-gapped bundle"
  echo "tag:   $TAG"
  echo "built: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "images:"
  $DOCKER image inspect "$API_IMG" "$RUNNER_IMG" "$PG_IMAGE" \
    --format '  - {{index .RepoTags 0}}  {{.Id}}  {{.Size}} bytes' 2>/dev/null || true
} > "$B/MANIFEST.txt"

cat > "$B/README.md" <<README
# ansible-ui — air-gapped install bundle

Tag \`${TAG}\`, built ${STAMP}. Everything needed to run ansible-ui on a host with
**no internet or container-registry access**.

## Contents
- \`images.tar.gz\` — the api, runner and postgres images (\`docker save\`d), + \`.sha256\`
- \`install.sh\` — loads the images **offline** and starts the stack
- \`docker-run.sh\` — the underlying 3-container launcher (api + runner + postgres)
- \`helm/\` — Helm chart for a Kubernetes air-gapped install
- \`MANIFEST.txt\` — exact image IDs + sizes

## Install (Docker)
\`\`\`sh
tar xzf ${NAME}.tgz && cd ${NAME}
APP_SECRET="\$(head -c32 /dev/urandom | base64)" WEB_PORT=8080 bash install.sh
# → ansible-ui on http://localhost:8080
\`\`\`
\`bash install.sh load\` only loads the images; \`bash install.sh down\` stops the stack.

## Install (Kubernetes)
Load \`images.tar.gz\` onto every node (or push into your private registry), then
\`helm install ansible-ui ./helm/ansible-ui\` pointing image refs at that registry.
README

echo "==> checksums"
( cd "$B" && sha256sum images.tar.gz > images.tar.gz.sha256 )

mkdir -p "$OUT"
OUT_ABS="$(cd "$OUT" && pwd)"
echo "==> packing → $OUT_ABS/$NAME.tgz"
tar -C "$STAGE" -czf "$OUT_ABS/$NAME.tgz" "$NAME"
echo "==> done"
ls -lh "$OUT_ABS/$NAME.tgz"
