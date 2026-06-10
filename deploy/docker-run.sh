#!/usr/bin/env bash
# Run ansible-ui without docker-compose: three independent containers on one
# network — postgres, the runner, and the api (which also serves the UI).
# Everything is configured via environment variables.
#
#   APP_SECRET=... WEB_PORT=8080 bash deploy/docker-run.sh up
#   bash deploy/docker-run.sh down
set -euo pipefail

DOCKER="${DOCKER:-docker}"
NET="${NET:-ansible-ui}"
TAG="${TAG:-latest}"
REGISTRY="${REGISTRY:-ansible-ui}"
WEB_PORT="${WEB_PORT:-8080}"
APP_SECRET="${APP_SECRET:-please-change-this-secret}"
POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-ansible}"
PG="ansible-ui-postgres"
RUNNER="ansible-ui-runner"
API="ansible-ui-api"

down() {
  $DOCKER rm -f "$API" "$RUNNER" "$PG" 2>/dev/null || true
  echo "stopped."
}

up() {
  $DOCKER network create "$NET" 2>/dev/null || true
  $DOCKER volume create ansible_data >/dev/null
  $DOCKER volume create ansible_pg >/dev/null
  down

  $DOCKER run -d --name "$PG" --network "$NET" --restart unless-stopped \
    -e POSTGRES_USER=ansible -e POSTGRES_PASSWORD="$POSTGRES_PASSWORD" -e POSTGRES_DB=ansible_ui \
    -v ansible_pg:/var/lib/postgresql/data \
    postgres:16-alpine

  $DOCKER run -d --name "$RUNNER" --network "$NET" --restart unless-stopped \
    -e RUNNER_ADDR=:8081 -e DATA_DIR=/data \
    -v ansible_data:/data \
    "$REGISTRY/runner:$TAG"

  # Pass through any LDAP_* / OIDC_* / COOKIE_SECURE auth settings present in the
  # environment, so SSO/directory auth is configurable without editing this file.
  local auth_envs=""
  for v in COOKIE_SECURE \
           LDAP_ENABLED LDAP_URL LDAP_START_TLS LDAP_BIND_DN LDAP_BIND_PASSWORD \
           LDAP_USER_BASE_DN LDAP_USER_FILTER LDAP_EMAIL_ATTR LDAP_DEFAULT_ROLE \
           OIDC_ENABLED OIDC_ISSUER OIDC_CLIENT_ID OIDC_CLIENT_SECRET \
           OIDC_REDIRECT_URL OIDC_USERNAME_CLAIM OIDC_DEFAULT_ROLE \
           RADIUS_ENABLED RADIUS_SERVER RADIUS_SECRET RADIUS_NAS_ID RADIUS_DEFAULT_ROLE \
           TACACS_ENABLED TACACS_SERVER TACACS_SECRET TACACS_DEFAULT_ROLE \
           SAML_ENABLED SAML_IDP_METADATA_URL SAML_ROOT_URL SAML_ENTITY_ID \
           SAML_SP_CERT SAML_SP_KEY SAML_USERNAME_ATTR SAML_DEFAULT_ROLE \
           GITHUB_CLIENT_ID GITHUB_CLIENT_SECRET GITHUB_REDIRECT_URL \
           BITBUCKET_CLIENT_ID BITBUCKET_CLIENT_SECRET BITBUCKET_REDIRECT_URL OAUTH_DEFAULT_ROLE \
           RUNNER_TOKEN METRICS_TOKEN DOCS_ENABLED; do
    if [ -n "${!v:-}" ]; then auth_envs="$auth_envs -e $v=${!v}"; fi
  done

  # shellcheck disable=SC2086
  $DOCKER run -d --name "$API" --network "$NET" --restart unless-stopped \
    -p "${WEB_PORT}:8080" \
    -e DATABASE_URL="postgres://ansible:${POSTGRES_PASSWORD}@${PG}:5432/ansible_ui?sslmode=disable" \
    -e RUNNER_URL="ws://${RUNNER}:8081" -e RUNNER_HTTP="http://${RUNNER}:8081" \
    -e DATA_DIR=/data -e APP_SECRET="$APP_SECRET" -e SEED_DEMO=true \
    -e DOCS_ENABLED="${DOCS_ENABLED:-true}" \
    $auth_envs \
    -v ansible_data:/data \
    "$REGISTRY/api:$TAG"

  echo "ansible-ui → http://localhost:${WEB_PORT}"
}

case "${1:-up}" in
  up) up ;;
  down) down ;;
  restart) down; up ;;
  *) echo "usage: $0 [up|down|restart]"; exit 1 ;;
esac
