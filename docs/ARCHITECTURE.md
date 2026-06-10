# Architecture

ansible·ui is four decoupled services orchestrated by Docker Compose. The design goal is a
**native terminal experience**: the bytes you see in the browser are the exact bytes
`ansible-playbook` wrote to its TTY.

## Deployment shape

Two app containers — **api** and **runner** — plus a database. There is **no docker-compose
and no nginx**: the compiled React/Vite SPA is embedded into the api binary (`go:embed`,
`internal/webui`), so the api serves the UI, REST and WebSocket on a single port.

- **Docker**: three independent `docker run` containers (`deploy/docker-run.sh`) — postgres,
  runner, api — on one network, sharing a named volume for `/data`.
- **Kubernetes**: a Helm chart (`deploy/helm/ansible-ui`) runs api + runner as **one pod**,
  the runner as a **sidecar**, sharing an emptyDir/PVC at `/data`; api reaches the runner at
  `localhost:8081`. Postgres is bundled (toggle) or external.

## Services

### web (embedded in the api)
The single-page app is built by the api image's node stage and embedded into the Go binary.
The api serves static assets (hashed, cached immutably) and falls back to `index.html` for
client-side routes. Auth never guards static paths — only `/api` and `/ws`.

### api (Go)
The control plane and the only service the browser talks to.

- **REST** for projects, inventories, templates and runs (Go 1.22 `net/http.ServeMux`).
- **Persistence** in Postgres via `pgx` (schema applied idempotently at boot).
- **Seeding**: the demo Ansible project is embedded in the binary (`go:embed`) and
  materialised into the shared data volume on first boot, along with a default inventory
  and starter templates.
- **Run manager**: on launch it resolves the working directory, inventory and environment,
  persists a `pending` run, then dials the runner. It relays the runner's byte stream to:
  - every subscribed browser (live, via `/ws/runs/{id}`), and
  - an in-memory buffer that is persisted to Postgres when the run finishes (for replay).
- **Event bus**: a fan-out hub publishes run lifecycle events to `/ws/events`, which the
  SPA uses to update lists and the dashboard live.
- **Recap parsing**: the stored output's `PLAY RECAP` is parsed (ANSI-stripped) into
  ok/changed/failed/… counts.

### runner (Go + ansible-core)
The execution sandbox — the only component with Ansible installed.

- Exposes `GET /v1/exec` (WebSocket). The api sends an `ExecSpec`; the runner builds the
  `ansible-playbook` argument list (as a slice — never a shell string) and starts it inside
  a **pseudo-terminal** (`creack/pty`) with `TERM=xterm-256color` and forced colour.
- Streams raw PTY bytes back as base64 frames; relays `resize` and `cancel` control frames.
- Cancellation signals the process group (`SIGINT → SIGTERM → SIGKILL`). Because the PTY
  layer calls `setsid`, the child is its own session/process-group leader, so the whole
  tree of Ansible workers is signalled via the negative PID.

### postgres
Stores projects, inventories, templates, and runs (including the full captured output as
`bytea` for replay).

## The live-terminal data path

```
ansible-playbook (PTY)
   │ raw bytes (ANSI, CR/LF, cursor moves)
   ▼
runner  ── base64 "stdout" frames over WS ──▶  api
                                                │  ├─▶ append to in-memory run buffer ─▶ Postgres (on finish)
                                                │  └─▶ fan-out to each browser subscriber
                                                ▼
                                      /ws/runs/{id} (binary frames)
                                                │
                                                ▼
                                       xterm.js  term.write(bytes)
```

- **Late join**: opening a run that is already in progress sends the buffered-so-far bytes
  first (a snapshot), then live frames — so you never miss output.
- **Finished run**: opening a completed run replays the stored output instantly, then ends.
- **Resize**: xterm's `FitAddon` + a `ResizeObserver` send `{type:"resize",cols,rows}` up
  the same socket; the api forwards it to the runner, which resizes the PTY.

## Why split api and runner?

- **Isolation**: only the runner mounts Ansible and executes playbooks. The api stays a
  small, stateless-ish gateway.
- **Version control**: the Ansible version is pinned in the runner image, independent of
  the host — guaranteeing compatibility on any host OS (the demo runs on a RHEL 7 host
  with no Python 3, Node or Go installed; everything lives in containers).
- **Scalability**: the runner is the natural unit to scale or sandbox per-tenant.

## Data & volumes

- `ansible_data` (shared by api + runner at `/data`): `projects/<slug>/…` working trees and
  per-run materialised inventories under `runs/<id>/`.
- `pgdata`: Postgres data directory.

Both api and runner run as root so they can manage the shared volume without cross-container
UID coordination. Hardening (non-root with an init container to own the volume, read-only
root FS, seccomp profiles) is a natural follow-up.

## Security notes

- `ansible-playbook` arguments are passed as an `exec` arg slice, never via a shell, so
  user-supplied values (limit, tags, extra-vars) cannot inject commands.
- File-browser paths are validated to stay within the project root.
- The bundled demo ships a Vault password and plaintext "secrets" **for demoing only** —
  never commit real credentials.
- There is no authentication yet; run it behind your own auth/VPN. Auth + RBAC is the next
  milestone.

## Backup, retention & export

**Backup (Postgres + the data volume):** Postgres holds all state except the materialised
working trees + run artifacts, which live in the `ansible_data` volume (re-derivable from git
for git-backed projects; only captured **artifacts** under `/data/artifacts/<runID>/` are not).

```sh
# Database — logical dump (restore with: psql … < ansible_ui.sql, or pg_restore for -Fc)
docker exec ansible-ui-postgres pg_dump -U ansible -d ansible_ui -Fc > ansible_ui.dump
docker exec -i ansible-ui-postgres pg_restore -U ansible -d ansible_ui --clean < ansible_ui.dump

# Captured artifacts (optional — the rest of /data is reproducible)
docker run --rm -v ansible_data:/data -v "$PWD":/out alpine \
  tar czf /out/artifacts.tgz -C /data artifacts
```

Keep the `APP_SECRET` safe alongside the dump — secrets at rest (credentials, vault tokens,
external-backend tokens, run `secret_vars`) are AES-GCM-encrypted with it, so a restore without
the same `APP_SECRET` can't decrypt them.

**Retention (Settings → Retention & export, admin):** `runsDays` / `auditDays` auto-delete
finished runs / audit events older than N days (0 = keep forever). A sweep runs on save, on
boot, and hourly; deleting a run also drops its on-disk artifacts. Active/queued runs are never
swept. Stored as `app_settings` (`retention.runs_days` / `retention.audit_days`).

**Export (compliance):** `GET /api/audit/export` and `GET /api/runs/export` (admin) stream the
full audit log / run history as CSV.
