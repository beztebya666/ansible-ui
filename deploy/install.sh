#!/usr/bin/env bash
# Offline installer for an ansible-ui air-gapped bundle (see deploy/bundle.sh).
# Loads the bundled container images from a local tarball — NO network / registry
# access needed — then starts the stack. Run this from inside the unpacked bundle.
#
#   APP_SECRET=... WEB_PORT=8080 bash install.sh
#
# Sub-commands: `bash install.sh load` only loads the images; `down` stops the stack.
set -euo pipefail
cd "$(dirname "$0")"

DOCKER="${DOCKER:-docker}"
IMAGES="${IMAGES:-images.tar.gz}"

load_images() {
  [ -f "$IMAGES" ] || { echo "error: $IMAGES not found (run from inside the unpacked bundle)" >&2; exit 1; }
  if [ -f "$IMAGES.sha256" ]; then
    echo "==> verifying checksum"
    sha256sum -c "$IMAGES.sha256"
  fi
  echo "==> loading images from $IMAGES (offline)"
  $DOCKER load -i "$IMAGES"
}

case "${1:-up}" in
  load) load_images ;;
  down) bash docker-run.sh down ;;
  up)
    load_images
    echo "==> starting ansible-ui"
    APP_SECRET="${APP_SECRET:-please-change-this-secret}" \
      WEB_PORT="${WEB_PORT:-8080}" bash docker-run.sh up
    ;;
  *) echo "usage: $0 [up|load|down]"; exit 1 ;;
esac
