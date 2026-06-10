<div align="center">

# ansible·ui

**A fast, minimal control plane for Ansible — with a genuinely native live terminal.**

Run playbooks from a clean web UI and watch them stream in real time, byte-for-byte,
exactly as they look in a terminal: real ANSI colour, real TTY behaviour, real `ansible-playbook`.

`Go` · `React + Vite` · `xterm.js` · `PostgreSQL` · `Docker` · `Kubernetes / Helm`

</div>

---

## ansible·ui vs. Ansible Semaphore

**Everything in ansible·ui is free and open-source — no tiers, no seats, no paywalls.**
The table shows where Ansible Semaphore charges for the same capability behind its paid **Pro**
(`$490/yr`) or **Enterprise** tier, ships only a partial version, or has nothing at all. Rows are
ordered by Semaphore status: **parity → partial → Pro → Enterprise → absent**.

> **ansible·ui** is `✅ Free` for every row. **Semaphore:** `✅ Free` (open-source core) · `◑ Partial` · `💲 Pro` · `💲 Enterprise` (paid) · `❌ —` (absent).
> _Semaphore tiers verified against [semaphoreui.com/pro](https://semaphoreui.com/pro) and its docs (June 2026)._

| Feature | ansible·ui | Ansible Semaphore |
| :------ | :--------: | :---------------: |
| Apps: Ansible · Terraform · OpenTofu · Terragrunt · Bash · PowerShell · Python | ✅ Free | ✅ Free |
| Single-binary self-host (one port) | ✅ Free | ✅ Free |
| Scheduled runs — cron + one-time | ✅ Free | ✅ Free |
| Project export / import (incl. CLI) | ✅ Free | ✅ Free |
| **Native PTY live terminal** — real ANSI/TTY, byte-for-byte | ✅ Free | ◑ Parsed log |
| Live lists & run updates over WebSocket (no refresh) | ✅ Free | ◑ |
| **Workflows** — DAG editor · conditional steps · parallel · per-step overrides · cross-project · versioning | ✅ Free | ◑ Build→deploy only |
| Notification channels — Slack · Telegram · Teams · Discord · PagerDuty · Opsgenie · +10 | ✅ Free | ◑ A few |
| Dynamic cloud inventory — AWS · GCP · Azure | ✅ Free | ◑ |
| Run artifacts · retention + CSV export | ✅ Free | ◑ |
| **Full bilingual UI** — EN + RU, every string | ✅ Free | ◑ Partial i18n |
| **Distributed runners** (tags / pools) + concurrency limits | ✅ Free | 💲 Pro |
| 2FA — TOTP authenticator + email OTP | ✅ Free | 💲 Pro |
| LDAP / AD + OIDC login | ✅ Free | 💲 Pro |
| External **HashiCorp Vault** secret storage | ✅ Free | 💲 Pro |
| Log export to external systems (syslog · Splunk · Elasticsearch) | ✅ Free | 💲 Pro |
| Analytics / insights / Prometheus metrics | ✅ Free | 💲 Pro |
| Terraform / OpenTofu **HTTP state backend** | ✅ Free | 💲 Pro |
| **HA active-active** (leader election + run-stream routing) | ✅ Free | 💲 Enterprise |
| Capability-based **custom roles** | ✅ Free | 💲 Enterprise |
| LDAP / OIDC **group → role mapping** | ✅ Free | 💲 Enterprise |
| **Air-gapped / offline install** + multi-instance | ✅ Free | 💲 Enterprise |
| **AI-agent control plane** (MCP server) | ✅ Free | ❌ — |
| **Pulumi** app | ✅ Free | ❌ — |
| Auth: **SAML SSO** | ✅ Free | ❌ — |
| Auth: **RADIUS · TACACS+** | ✅ Free | ❌ — |
| Secret managers: **AWS · Azure · GCP · CyberArk** | ✅ Free | ❌ — |
| **SSH certificates** (CA-signed) + per-inventory SSH keys | ✅ Free | ❌ — |
| **Secret-access audit trail** + notification delivery log | ✅ Free | ❌ — |
| Host **fact storage & visualisation** + reachability monitoring | ✅ Free | ❌ — |
| **Compliance report** (access matrix · secret-access · activity) | ✅ Free | ❌ — |
| Notification **egress proxy** (alert-proxy) | ✅ Free | ❌ — |
| **Licensing / seats** | ✅ **Unlimited — free forever** | 💲 Paid Pro / Enterprise |

<sub>Semaphore status verified against semaphoreui.com/pro + docs (Jun 2026): runners, 2FA, LDAP/OIDC, log export, insights, Terraform HTTP backend & HashiCorp Vault are **Pro**; HA, custom roles, group→role mapping & air-gapped are **Enterprise**; SAML, Pulumi, RADIUS/TACACS+ are absent. `✅ Free` = in Semaphore's open-source core; `◑` = partial.</sub>

---

## Screenshots

**Live PTY terminal — watch `ansible-playbook` stream byte-for-byte, real ANSI colour:**

![Live terminal](docs/img/terminal.gif)

<details><summary>Full run detail (static)</summary>

![Run detail](docs/img/terminal.png)

</details>

|  |  |
| :---: | :---: |
| **Dashboard** — live overview, recent runs & health | **Workflows** — visual DAG, conditional + parallel steps |
| ![Dashboard](docs/img/dashboard.png) | ![Workflows DAG](docs/img/workflows.png) |
| **HA cluster** — leader, runner capacity, live queue | **Insights** — success rate, duration & dispatch trends |
| ![Cluster](docs/img/cluster.png) | ![Insights](docs/img/insights.png) |

---

## Why

Ansible Semaphore works, but the UX is dated and the output viewer doesn't feel like a
terminal. **ansible·ui** is built around three ideas:

1. **The terminal is the product.** Playbooks run inside a real **PTY** on a dedicated
   runner; raw bytes are streamed over WebSocket into **xterm.js**. What you see is what
   `ansible-playbook` actually printed — colour, progress, alignment and all.
2. **Fast and minimal.** A snappy single-page app, a tiny Go API, no bloat. Lists update
   live over a WebSocket event bus — no spinners, no manual refresh.
3. **Production-shaped.** Decoupled microservices, Postgres persistence, run history with
   full replay, health checks, and a one-command Docker Compose deploy.

## Architecture

The app is **two containers** (one pod in Kubernetes — runner as a sidecar) plus a database:

```
                       ┌───────────────────── pod ─────────────────────┐
            REST+WS+UI │  ┌──────────┐   ws://localhost  ┌──────────┐   │
 browser ──────────────┼─▶│   api    │──────────────────▶│  runner  │   │
 xterm.js  (one port)  │  │  (Go,    │     (PTY bytes)   │  (Go +   │   │
                       │  │  serves  │                   │ ansible) │   │
                       │  │  the SPA)│◀── shared /data ─▶│          │   │
                       │  └────┬─────┘                   └────┬─────┘   │
                       └───────┼─────────────────────────────┼─────────┘
                               ▼                              │ exec in PTY
                         ┌──────────┐                         ▼
                         │ postgres │                  ansible-playbook
                         └──────────┘                  (real TTY, colour)
```

| Container | Stack | Responsibility |
|-----------|-------|----------------|
| **api** | Go (stdlib mux, pgx, gorilla/ws) + **embedded** React/Vite SPA | Serves the UI **and** REST + WebSocket on one port; persistence, auth, seeding; relays the runner stream and persists it for replay |
| **runner** | Go + `creack/pty` + ansible-core + git | Clones repos, installs `ansible-galaxy` requirements, executes `ansible-playbook` inside a PTY |
| **postgres** | PostgreSQL 16 | Projects, repositories, credentials, templates, runs, users |

The SPA is compiled into the api binary (`go:embed`), so there is **no nginx/web container**.
The runner is the only component that touches Ansible/git, so the Ansible version is pinned
in its image — full compatibility regardless of the host (here: a RHEL 7 host with no Python 3,
Node, Go or Ansible installed; everything lives in containers).

## Quick start

Requires only **Docker** (or a **Kubernetes** cluster). Nothing else on the host — no Node,
Go, Python or Ansible. No docker-compose.

### Docker

```bash
git clone <this repo> ansible-ui && cd ansible-ui
make up        # builds the two images, then runs postgres + runner + api as 3 containers
```

Open **http://localhost:8080** and **create the first admin account** (first-run setup).
Config is via env (see [`deploy/docker-run.sh`](deploy/docker-run.sh)):
`WEB_PORT`, `APP_SECRET`, `POSTGRES_PASSWORD`, `REGISTRY`, `TAG`.

On first boot the api seeds a **Demo** project and starter templates — hit
**New run → Hello World → Launch** and watch the terminal.

To run from a **Git repository**: add it under **Repositories** (with a Key Store credential
for private repos), **Sync** it, then create a **Project** from one of its subfolders — each
run pulls the latest commit and installs any `requirements.yml` via `ansible-galaxy` first.

### Kubernetes (Helm)

The chart runs the api + runner as **one pod (runner is a sidecar)** sharing `/data`, with a
bundled PostgreSQL (or point at your own via `externalDatabase.url`).

```bash
helm upgrade --install ansible-ui deploy/helm/ansible-ui \
  -n ansible-ui --create-namespace \
  --set appSecret=$(openssl rand -hex 16) \
  --set image.registry=ghcr.io/you        # your pushed images

kubectl -n ansible-ui port-forward svc/ansible-ui 8080:80   # then open http://localhost:8080
```

Everything is configurable in
[`deploy/helm/ansible-ui/values.yaml`](deploy/helm/ansible-ui/values.yaml): images,
`appSecret`/`existingSecret`, bundled vs external Postgres, persistence, Service type,
Ingress + TLS, resources.

### Air-gapped (offline) install

For hosts with **no internet or registry access**, build a single self-contained
tarball on a connected machine and copy it across:

```bash
bash deploy/bundle.sh          # → dist/ansible-ui-airgapped-<tag>-<date>.tgz
```

The bundle `docker save`s the api + runner + postgres images (+ checksum/manifest)
alongside the run script, an offline installer, and the Helm chart. On the air-gapped
host:

```bash
tar xzf ansible-ui-airgapped-*.tgz && cd ansible-ui-airgapped-*
APP_SECRET="$(head -c32 /dev/urandom | base64)" bash install.sh   # loads images offline, then runs
```

`install.sh` verifies the checksum, `docker load`s the images (no pull), and starts the
stack — `bash install.sh load` only loads, `down` stops. For Kubernetes, load
`images.tar.gz` onto every node (or push to a private registry) then `helm install`.

### Make shortcuts

```bash
make up            # build images + run (Docker)
make down          # stop
make smoke         # demo-playbook smoke test
make verify        # auth + git-repo + run-from-repo
make helm-lint     # lint the Helm chart
make helm-install  # install into Kubernetes
```

## The demo playbooks

The seeded **Demo** project is a progression from trivial to advanced — every playbook is
**localhost-safe** (`connection: local`, writes only under `/tmp`) so it runs with no remote
hosts or credentials. See [`examples/project/`](examples/project/).

| # | Playbook | Teaches |
|---|----------|---------|
| 01 | `01-hello.yml` | ping, debug, loops |
| 02 | `02-facts.yml` | fact gathering, `set_fact` |
| 03 | `03-variables-loops.yml` | vars, loops, conditionals, `register`, `assert` |
| 04 | `04-files-and-templates.yml` | copy/template/lineinfile/blockinfile + handlers + idempotency |
| 05 | `05-handlers.yml` | `notify`, `listen`, `flush_handlers` |
| 06 | `06-roles.yml` | role composition (`common`, `webserver`, `monitoring`) |
| 07 | `07-blocks-rescue.yml` | `block`/`rescue`/`always` |
| 08 | `08-async-parallel.yml` | `async`/`poll`, `async_status` |
| 09 | `09-vault.yml` | secrets, `no_log`, Ansible Vault |
| 10 | `10-full-stack.yml` | pre/post tasks, tag-driven roles, a verification gate |

## Features

- **Git repositories** — point at GitHub / GitLab / Bitbucket / internal git. One repo, many
  project folders; per-run pull; `ansible-galaxy` requirements installed automatically.
- **Key Store** — SSH keys, logins and vault passwords, **encrypted at rest** (AES-256-GCM).
- **Environments** — reusable extra-vars + process env vars, attached to templates.
- **Survey variables** — typed operator prompts (text/int/enum/secret) at launch.
- **Schedules** — cron-driven template runs with pause/resume and next-run times.
- **Authentication** — local users (bcrypt), **LDAP** and **OIDC SSO**, first-run setup,
  sessions, **API tokens** (bearer), **webhooks** to trigger templates, guarded API + RBAC.
- **Live terminal** — real PTY, real ANSI colour, resizes with your window, cancel mid-run.
- **Run history & replay** — every run's full output is stored; reopen a finished run and
  it replays instantly. Open a *running* run from another tab and it catches you up, then
  streams live.
- **Templates** — save a project + playbook + inventory + options as a one-click run config.
- **Projects** — browse and edit playbooks/roles/inventories in the built-in file editor.
- **Inventories** — managed in the UI or as files; pick per-run.
- **Ad-hoc runs** — choose limit, tags, skip-tags, extra-vars (JSON or `key=value`),
  check/dry-run, diff, and verbosity.
- **Live everywhere** — lists and the dashboard update over a WebSocket event bus.

## Development

Backend (cross-compiles to Linux; the runner needs Linux for the PTY):

```bash
GOOS=linux go build ./...
```

Frontend (Vite dev server with API proxy):

```bash
cd web && npm install && npm run dev    # proxies /api + /ws to $VITE_API_TARGET (default :8080)
```

See [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) for the request/stream flow and
[`docs/API.md`](docs/API.md) for the HTTP + WebSocket contract.

## License

MIT.
