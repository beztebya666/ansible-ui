#!/usr/bin/env bash
# All-in-one entrypoint: start PostgreSQL, the runner, then the api - all in one
# container. For quick trials/demos only; for anything real use docker-compose or Helm.
set -e

PGDATA=/var/lib/postgresql/data
PGBIN="$(ls -d /usr/lib/postgresql/*/bin | head -1)"

mkdir -p "$PGDATA" /data
chown -R postgres:postgres "$PGDATA"

# First run: initialise the database cluster.
if [ ! -s "$PGDATA/PG_VERSION" ]; then
  su postgres -c "$PGBIN/initdb -D '$PGDATA' --auth-local=trust --auth-host=trust -E UTF8 >/dev/null"
fi

# Start Postgres (local only) and wait for it.
su postgres -c "$PGBIN/pg_ctl -D '$PGDATA' -o '-c listen_addresses=127.0.0.1' -w -t 60 start"

# Create role + database (idempotent).
su postgres -c "psql -tAc \"SELECT 1 FROM pg_roles WHERE rolname='ansible'\" | grep -q 1 || psql -c \"CREATE ROLE ansible LOGIN PASSWORD 'ansible' SUPERUSER\""
su postgres -c "psql -tAc \"SELECT 1 FROM pg_database WHERE datname='ansible_ui'\" | grep -q 1 || createdb -O ansible ansible_ui"

# Runner (background) + api (foreground). They share /data.
DATA_DIR=/data RUNNER_ADDR=:8081 /usr/local/bin/runner &

export DATABASE_URL="postgres://ansible:ansible@127.0.0.1:5432/ansible_ui?sslmode=disable"
export RUNNER_URL="ws://127.0.0.1:8081"
export RUNNER_HTTP="http://127.0.0.1:8081"
export DATA_DIR=/data
export APP_SECRET="${APP_SECRET:-change-me-all-in-one-trial}"
export SEED_DEMO="${SEED_DEMO:-true}"
exec /usr/local/bin/api
