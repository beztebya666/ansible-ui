# Roadmap — toward Semaphore parity (and beyond)

The goal is **full feature parity with Ansible Semaphore, then well beyond it.**
This document tracks status in buckets: **✅ Done · 📋 Next up (doing now) · 🧭 Planned (later) ·
✨ Beyond Semaphore.**

---

## ✅ Done (shipped)

**Core execution**
- Projects, Inventories, Task templates, ad-hoc runs.
- Live PTY terminal (real ANSI colour) over WebSocket; run history + instant replay + late-join.
- PLAY RECAP parsing, run cancellation (Ctrl-C), dashboard.
- **Live everywhere** — WebSocket event bus invalidates lists/detail/dashboard the moment a run
  changes, backed by a ~2 s polling fallback so nothing is ever stale without an F5.
- **Consistent status everywhere** — one colour-tinted `StatusBadge` (pending/running/success/
  failed/canceled) on every run row, run detail and summary. Rows are fully clickable to drill in.
- **Multi-tenant** — project = tenant with a sidebar switcher; every request carries `X-Project-Id`
  and lists are project-scoped (shared resources stay visible via `project_id IS NULL`).
- **Distributed runners** — execution agents self-register with the control plane (`API_URL` +
  `RUNNER_ADVERTISE_URL` + `RUNNER_TAGS`, shared `RUNNER_TOKEN`) and heartbeat; a **Runners** page
  shows each with online/offline status, tags, platform and last-seen. A template targets a **runner
  pool by tag** (`runnerTag`); the run is dispatched to a matching online runner (the built-in runner
  serves the default tag), recorded as `runnerId` on the run, with logs streamed back over the same
  WebSocket. No online runner for a requested tag aborts the launch clearly. For **git-backed**
  projects the control plane syncs the repository **onto the chosen runner's own filesystem** before
  executing, so remote runners need no shared storage (local-directory projects still require the
  built-in runner and abort clearly otherwise). Each runner advertises a **concurrency cap**
  (`RUNNER_CONCURRENCY`, 0 = unlimited); when a pool is full a launch is **queued** and dispatched
  automatically as a slot frees up (in-memory FIFO; the spec — which carries secrets — is never
  persisted, only a `queued` run row). E2E-verified: a tagged git-backed run executed on a second runner
  with **no shared `/data`** (it cloned the repo itself); with the runner capped at 1, a second run sat
  **queued** and started the moment the first finished (both attributed to the runner); default ran on
  the built-in; an unknown tag and a local-dir project on a remote runner both returned 409.

**Multi-app (beyond Ansible)**
- Execution backends: **Ansible**, **Terraform / OpenTofu / Terragrunt** (plan/apply/destroy,
  workspace, auto-approve), **Bash / PowerShell / Python** scripts.
- **Configurable Applications** — toggle the built-ins, tweak their base args, or register your own
  custom binary + args (Applications page).

**Source control & credentials** (Semaphore: Repositories + Key Store)
- **Repositories** — Git sources (GitHub / GitLab / Bitbucket / internal, air-gapped); branch
  selection, per-repo sync, status + last commit.
- **One repo → many projects**: a project is a *subfolder* of a repository.
- **Edit git-backed files via PR** — editing a file in a git-backed project (a direct write would be
  wiped by the pre-run `git reset --hard`) instead **commits onto a NEW branch and pushes it**, never
  the (usually protected) base/master, then surfaces a one-click **Open a PR/MR** link. Runner-side
  commit+push with the repo's Key Store credentials; base-branch pushes are refused. E2E-verified: edit
  → new branch carries the change, master untouched.
- **Local directory source** — Git-free projects: upload a `.tar.gz` / `.tgz` / `.tar` / `.zip`
  archive into a local project (format detected by magic bytes; zip read via io.ReaderAt so large
  archives don't load into memory; zip-slip-guarded; 1 GiB cap). **Password-protected zips supported**
  (ZipCrypto + AES via `yeka/zip`): the UI prompts for the password and retries; wrong password
  re-prompts. Files + playbooks appear immediately. E2E-verified (tar.gz, zip, 73 MiB, encrypted+password).
- **ansible-galaxy** — `requirements.yml` (roles/collections, incl. AWS collections) auto-installed
  before each repo-backed run, streamed into the same terminal.
- **Key Store** — SSH keys, username/password, vault passwords, **encrypted at rest** (AES-256-GCM);
  used for private Git auth and `--vault-password-file`.
- **External secret managers** — connect **HashiCorp Vault** (KV v2) *or* **AWS Secrets Manager** as a
  *secret backend*, each with a **Test connection** check. Vault: address / mount / namespace / token
  (+ optional self-signed TLS). AWS: region / access-key-id / secret-access-key (signed with a
  hand-rolled **Signature V4**, no SDK; optional custom endpoint). The secret credential is encrypted
  at rest and never returned. An environment then references secrets (`VAR ← backend · path · field`,
  as env-var or extra-var); values are fetched **fresh at launch** — nothing secret is stored locally,
  only the reference. A resolution failure aborts the launch. E2E-verified: Vault against a live server;
  AWS SigV4 accepted by the real endpoint (fake creds → `UnrecognizedClientException`, i.e. signature
  structurally valid).
- **Secrets never leak — masked by default, revealed on demand** — resolved secret values (env
  var-secrets + external secrets) reach the runner via a transient **0600 extra-vars file** (`-e @file`),
  so raw values never touch the command line, the run **log**, `run.args` or `ps`. The persisted
  `run.ExtraVars` / echoed command show non-secret vars in full and each secret **masked** (`••••••`) —
  the operator sees *which* secret is applied. An **admin** can reveal the real values on demand
  (`GET /api/runs/{id}/secrets`, AES-decrypted server-side, **audited** `run.secretsRevealed`; non-admin
  → 403) via an Eye button on the run detail. E2E-verified: `assert` of the real value passes while
  `grep` of the log finds nothing; admin reveal returns the value, viewer gets 403.

**Templates (full run config)**
- Per-template: inventory, limit, tags/skip-tags, extra-vars, check/diff, verbosity.
- **CLI args** (raw passthrough), **vault credential**, **environment**, **survey variables**,
  **suppress-success notifications**, **allow-parallel**, **auto-run-on-commit**, infra
  **workspace / auto-approve**. Saved views with search + app filter.
- **Template types** — **Task** (ordinary run) · **Build** (each run gets an incrementing artifact
  number `#N`) · **Deploy** (ships the latest *successful* build of a chosen build template, injecting
  `build_version` / `build_commit` / `build_run_id` as extra-vars and tagging the run `→ #N`). Type
  badges on cards, artifact shown on the run detail. E2E-verified: build #1/#2 → deploy consumed #2.
  Templates list also has a Semaphore-style **type filter** (All / Task / Build / Deploy) and a
  **VERSION chip** per card (a build's own latest `#N`; the build a deploy would roll out), hydrated in
  one query via `LatestBuildVersions`. The template form picks the type via prominent **TASK / BUILD /
  DEPLOY tabs at the top** (icons + one-line descriptions), matching Semaphore's New-Template dialog —
  the type was previously an easy-to-miss dropdown mid-form.

**Auth**
- Local users (bcrypt), first-run admin setup, HttpOnly-cookie sessions, guarded API + WebSocket,
  global RBAC (admin/user), user management.
- **Per-project RBAC / teams** — assign users a project role (**viewer** read-only · **editor** run &
  edit · **admin** manage members) on a project's **Members** tab. Enforced server-side on **runs,
  templates, inventories, schedules, file edits/uploads and project settings** (schedules are gated too,
  so a viewer can't escalate by scheduling a run); a project with no members stays open (backward
  compatible); global admins always have full access. E2E-verified: viewer→403 on every mutation
  (template/inventory/schedule create+update+delete, run launch, file write), editor→allowed,
  admin-only for member management + project settings.
- **LDAP** bind+search, **OIDC SSO** (discovery + auth-code), and **GitHub / Bitbucket** OAuth2
  sign-in ("Sign in with GitHub/Bitbucket") — config-gated by client id/secret; external users upserted.

**Environments, Surveys & Schedules**
- **Environments** — reusable extra-vars **and** process env vars (lowest precedence).
- **Survey variables** — typed operator prompts (text/int/enum/secret) → extra-vars at launch.
- **Schedules** — 5-field cron, in-process scheduler, next/last fire times, pause/resume.
- **Auto-run-on-commit** — the scheduler polls repos that back `autorunOnCommit` templates (~60 s,
  a no-op unless a template opts in) and fires them when the repo's commit advances (actor `autorun`).
  E2E-verified: a new commit auto-launched the template.

**API & Integrations**
- **API tokens** — sha256-hashed bearer tokens accepted alongside the session cookie.
- **Integrations** — inbound per-template webhook URLs trigger runs (GitHub/GitLab/CI push).
- **API Explorer** — every endpoint with auth/admin requirements, editable request + **live try-it**,
  JSON highlighting; Reset restores the request and clears the response.

**Audit & observability**
- **Audit log** — who / what / when across runs, changes and sign-ins; searchable, category filter,
  live (visible to all users). Action codes humanized + bilingual.
- **Comprehensive coverage** — every mutating action records activity: create/update/delete for
  projects, templates, repositories (+ sync), credentials, environments, schedules, integrations,
  applications, notifications, inventories; file saves; users; API tokens; sign-in/out/setup;
  run launch/finish/delete/clear.
- **Mini-audit everywhere** — the actor (who triggered it) shows on every run row, the run detail
  ("Triggered by"), the dashboard Activity feed and the Audit log — not just one page.

**Run experience**
- **Structured Semaphore-style Log view** — per-line **timestamp gutter**, full SGR ANSI (16/bright/
  256/truecolor, bold/dim/italic/underline/reverse) from a single shared palette, bright high-contrast
  text on near-black, banners clipped to one line (no wrap, no h-scroll). Bold brightens 0–7 → 8–15
  like a real terminal, so ansible's grey `task path:` lines (`-vv`) are visible.
- **Live structured streaming** — the log streams the PTY over `/ws/runs/{id}` line-by-line (smooth,
  immediate) and reconciles to the stored output + server timestamps on finish; a slow poll is the
  socket-down fallback (live-everywhere bar). Opens with a short **runner preamble** (queued / started
  / inventory) instead of cold-starting on ansible's banner.
- **Clean, parity output** — runner pinned to **ansible-core 2.20.x** so the `-vv` banner / task paths
  match the reference; demo `ansible.cfg` carries no profile_tasks/timer noise; static inventories
  load the project's `group_vars`/`host_vars` (turnkey, no failed asserts).
- **Beautiful COMMAND block** — syntax-highlighted (binary / flags / paths), the inventory + playbook
  are **clickable** to the file editor (no underline), wraps only **between** arguments, never truncates.
- **File explorer with edit/manage** — browse the project tree, edit files (syntax-highlighted,
  fullscreen), and **rename / delete** any file or folder via minimal hover actions (styled prompt /
  confirm, live tree refresh, project-root-guarded). **Instant client-side search** on the Playbooks /
  Files / Inventories tabs (filters + auto-expands matching folders).
- **Auto-detected inventories** — inventory files in the project (`*.ini`, `hosts`, anything under
  `inventory*/`) are surfaced automatically with an "auto" badge + one-click "add as reusable
  inventory"; the New-inventory file/dynamic picker selects a file from the project tree (no typing).
  Added inventories are named **`<folder>/<file>`** (e.g. `dev/inv-k8s.yml`) so several same-named
  files stay tellable apart in the launcher's inventory dropdown.
- **Richer run preamble** — the log opens with task/queue, project, app · playbook, inventory, synced
  commit and the exact `$ ansible-playbook …` command before the tool output (Semaphore-style).
- **Clickable playbook** in the run detail → opens it in the file editor (deep-link).
- **Roles & collections panel** — parses `requirements.yml` and shows each role/collection as a
  clickable badge (galaxy / git → source page; local → file editor).
- **Short unique run id** in the detail header (click-to-copy) and run rows, to tell tasks apart.
- **Fullscreen console** (Esc to exit) + a clean in-app **Raw log** view (`/runs/:id/raw`); the
  **file editor** has the same expand-to-fullscreen toggle (Esc to exit).
- Run-detail values wrap in full (never truncated, no "…"); uniform run-row heights.
- **Delete runs** — per-run (hover trash + confirm) and admin **Clear history** (keeps active runs).

**Notifications**
- **Telegram / Slack / webhook / email (SMTP)** channels alerting on run completion, per-event filters
  + test send. SMTP supports optional auth (PlainAuth) and auto-STARTTLS.

**UX / presentation**
- **Theme toggle** (dark / light) persisted to `localStorage`, applied to `<html data-theme>`.
- **Time format** — 12h / 24h selector (default 24h) + timezone override for run timestamps.
- **Project edit** — rename, change tag (slug) and description; pencil + red-trash actions.
- **Full bilingual EN/RU** across the **entire UI** — navigation, page chrome, status badges, run
  detail, relative times, audit labels, **every form, modal, tooltip and placeholder**: admin forms
  (Schedules, Environments, Key Store, Login, Integrations, Applications), **Settings** (preferences,
  notifications, users, API tokens, docs toggle), **Repositories**, **Projects + inventory** editors,
  the **Templates editor**, the **launcher** + run-prompt dialog, the **API Explorer** chrome, and the
  per-app field labels/hints (`appText`). Audited end-to-end + RU-verified by screenshots; only brand
  names (GitHub/Slack…) and technical tokens (`--extra-vars`, SMTP, Bearer) stay literal.
- **No native HTML UI** — custom-styled dropdowns, checkboxes/toggles, a **toaster** (success/error)
  and **confirm/prompt modals** replace every native `<select>`, `<input type=checkbox>`, `alert()`,
  `confirm()` and `prompt()`. Feedback is in-app + themed everywhere.
- **No GPU-dependent effects** (no blur) — plain darkening; brand links to Dashboard; app chrome is
  not drag-selectable.

**Deployment**
- No docker-compose: SPA embedded in the api (2 containers: api + runner sidecar). Helm chart for
  Kubernetes; `docker run` path for non-k8s. All env/values configurable.

---

## ✅ Recently completed (this cycle)

- **Launch-time prompts + multi-vault** — the per-template `prompts` open a pre-filled launch dialog
  (inventory / branch / limit / tags / skip-tags / CLI args / verbosity / **vault credentials**); the
  multi-vault picker lets the operator choose exactly which vaults to unlock, replacing the template
  list. Verified with real ansible-vault decryption (right vault succeeds, wrong fails).
- **Edit git-backed files → PR** — git-backed edits commit onto a NEW branch + push (never the protected
  base) and offer a one-click Open-a-PR link. Verified: change on the new branch, master untouched.
- **Secret hygiene** — secrets masked in the log / `run.ExtraVars` / `run.args` by default, delivered to
  the runner via a 0600 `-e @file`; **admin-only on-demand reveal** (audited). Closed a real leak where
  resolved secret values were printed in the run log and persisted in the run record.
- **RBAC tightened** — per-project gating now also covers templates / inventories / schedules (so a
  viewer can't escalate by scheduling a run); verified viewer→403 / editor→allowed across the board.
- **Quality audit** — EN↔RU i18n parity check, full API-Explorer route coverage (every route documented),
  queued-run cancellation + consistent terminal-state audit/notifications, `queued` status surfaced on
  the dashboard. `go vet` clean.

---

## ✅ Production-grade operations (shipped this cycle)

Parity with Semaphore — plus the planned next-gen (distributed runners, external secrets, template
types) — is shipped; this batch hardened the product for real multi-user teams and day-2 operations.
Every item below is **E2E-verified** (own screenshots + API/runtime checks).

- **Run approval gates** ✅ — a template (`requiresApproval`) holds every launch in **`awaiting`**
  (held in memory like the dispatch queue; never auto-runs) until a **project admin** **approves**
  (→ dispatched, honouring runner capacity/queue) or **rejects** (→ canceled). Applies to scheduled /
  webhook launches too. Audited (`run.awaiting` / `run.approved` / `run.rejected`). UI: a Requires-
  approval template toggle + Approve/Reject buttons + an `Awaiting approval` status badge. E2E-verified:
  launch→awaiting, approve→success, reject→canceled, re-approve a finished run→409.

- **GitOps editing polish** ✅ — a **diff view** before commit (LCS line diff, per-file collapsible),
  **multi-file staged commits** (a "stage" cart commits several edited files onto one branch in one
  commit), and a **Pushed branches** panel listing every branch the UI created (files, author, when, PR
  link). E2E-verified: a.yml + b.yml committed together onto `aui/multi`, master untouched.

- **Four external secret backends — hand-rolled, no cloud SDK** ✅ — **Azure Key Vault** (AAD
  client-credentials → vault data plane) and **GCP Secret Manager** (service-account JWT, RS256-signed by
  hand → OAuth token → `:access`) join **Vault** + **AWS** (SigV4). Test-connection + reference-at-launch
  + masked/admin-reveal as before. E2E-verified that each reaches the real provider with throwaway creds
  (Azure → `AADSTS90002 tenant not found`; GCP → `Invalid grant: account not found`).

- **Turnkey cloud inventory — AWS EC2 + GCP Compute + Azure (trio complete)** ✅ — a first-class **Cloud**
  inventory type: pick provider + a Key Store credential and the runner **generates the inventory-plugin
  config**, passing cloud creds as **process env** (never into extra-vars/log) — no hand-written YAML.
  **AWS** (`amazon.aws.aws_ec2`, login_password keys → `AWS_*`), **GCP** (`google.cloud.gcp_compute`, a
  `gcp` SA-JSON credential → `GCP_*`), **Azure** (`azure.azcollection.azure_rm`, an `azure` SP credential
  → `AZURE_*`). Runner ships boto3 + google.cloud/google-auth + azure.azcollection (pinned SDK incl.
  azure-cli-core). E2E-verified for all three: plugin generated `.aui-…<provider>.yml`, loaded + reached
  the cloud (AWS `AuthFailure`, GCP `invalid_grant`, Azure `login.microsoftonline.com` tenant rejection),
  secret never in the log.

- **Metrics & SLOs (Prometheus `/metrics`)** ✅ — a hand-rolled exposition (no client lib): live snapshot
  of `ansibleui_runs{status}`, `_runs_active`, `_dispatch_queue_depth`, `_runs_awaiting_approval`,
  `_projects`, `_templates`, `_runners{state}`, `_runner_load{runner}`, `_build_info`. Outside `/api` (no
  session guard) so Prometheus can scrape it; public unless `METRICS_TOKEN` is set. E2E-verified live
  (caught `running 1` mid-run, `success` incremented after).

- **Insights / trend views** ✅ — an in-app **Аналитика** page (`GET /api/insights`): summary cards
  (total runs, success rate, avg duration, avg dispatch wait) + a hand-rolled **stacked bar chart** of
  runs-per-day by outcome (no chart lib), with project + window (7/14/30d) filters and live refresh.
  E2E-verified: 4✓/1✗ → 80% success.

- **Run artifacts (any runner)** ✅ — a template lists **artifact globs** (e.g. `reports/*.html`); the
  **runner** globs its working dir after the run and **streams each file back over the run socket**
  (`FrameArtifact`), so artifacts are captured **wherever the run executed** — built-in **or remote**
  (no shared `/data` needed). The api saves + indexes them (`run_artifacts`); the run detail lists each
  as a one-click **download**. Subdir paths preserved; deduped; per-file (12 MB over WS) + per-run caps;
  path-escape guards on both ends; files removed on run delete. E2E-verified: built-in captured
  `report.txt`; a **remote `edge` runner** (its own filesystem, no shared `/data`) captured + uploaded
  `report.txt` (download returned the exact bytes); bogus id → 404.

- **Notification templating** ✅ — each channel has an optional **message template** with
  `{{run}}/{{status}}/{{project}}/{{app}}/{{playbook}}/{{commit}}/{{version}}/{{actor}}/{{exitCode}}`
  placeholders; empty = the default body. E2E-verified via a webhook capture (rendered body arrived
  `RUN=nightly STATUS=success PROJ=Demo…`).

- **Retention, export & backup** ✅ — admin **retention policy** (Settings → Retention & export):
  auto-delete finished runs / audit older than N days (0 = forever; `app_settings`), swept on save, on
  boot, hourly — deleting a run drops its artifacts; active/queued never touched. **CSV export** of the
  full audit log + run history (admin, streamed). **Backup/restore** documented (pg_dump + artifacts tar
  + `APP_SECRET` caveat). E2E-verified: a backdated run swept while a recent one survived; viewer → 403.

- **High availability — multi-replica safe** ✅ (both steps) —
  - **Step 1 · scheduler leader election:** the scheduler loop (cron / autorun / retention / queue
    drain) runs only on the replica holding a **Postgres session advisory lock**
    (`store.TryAcquireLeadership`); standbys retry every 30s and take over when the lock frees (crash →
    session ends → lock auto-releases). Verified with 2 replicas: schedules fired **once**, failover
    ~30s, old leader rejoined as standby (no split-brain).
  - **Step 2 · externalized dispatch queue:** the in-memory `dispatchQ` / `runnerLoad` /
    `pendingApproval` are **gone**. A launch persists the run as `queued` (or `awaiting`) plus an
    **AES-encrypted launch envelope** (exec spec + repo-sync spec); the **leader's drainer** claims
    queued runs atomically (`queued → pending` via `UPDATE … WHERE status='queued'`) and dispatches
    them, reading **runner load straight from the runs table** (shared across replicas). So capacity is
    enforced globally and queued/awaiting runs **survive a restart**. E2E-verified: single-replica has
    no regression (launch, capacity/queue, approval, cancel-queued); a queued run **survived an API
    restart** and dispatched; with **2 replicas** a `cap=1` runner ran **only one** job at a time
    (not one-per-replica), the leader draining the shared queue.

- **Two-factor authentication (TOTP)** ✅ — opt-in 2FA for local accounts: a hand-rolled **RFC-6238
  TOTP** (no dependency; SHA-1, 30s, 6 digits, ±1 window) works with Google Authenticator / Authy /
  1Password. Settings → enrol (shows the base32 key + `otpauth://` URI) → verify a code → enabled; the
  secret is **AES-encrypted at rest**. Login then needs the code (`{twoFactorRequired:true}` → resubmit
  with `code`). Admins can **reset** a user's 2FA for recovery (audited `auth.2faEnabled/Disabled/Reset`).
  Password-only accounts are unaffected. E2E-verified: enrol → password-only login returns
  `twoFactorRequired`, wrong code → 401, valid code → session; the non-2FA admin still logs in; admin
  reset restores password-only login. UI: setup card + login code step, both EN/RU.

- **COMMAND-block overflow fix** ✅ — long inventory paths (`.aui-…azure_rm.yml`) now wrap inside the box
  (`break-all` + `min-w-0`) instead of crossing the border.

---

## 📋 Next up — *doing now*

Everything in **Done** above is E2E-verified, including **HA (both steps)** — the dispatch queue is
externalized and capacity is shared, so the API is multi-replica-safe. The remaining bets are new
features rather than parity gaps.

## 🧭 Feature-gap backlog (what Semaphore / the field has and we don't)

Synthesised from Semaphore's **roadmap** (v2.18–v2.21), the **Pro/Enterprise** feature list, the in-app
**System Information** + **HA Cluster** screens, and the `/vs/{awx,tower,rundeck,gitlab,jenkins,spacelift,
gaia}` comparison pages. Everything already shipped (above) is excluded. `◻️` = not built yet · `◐` =
partial. Tier tags: `(Sem-Pro)` `(Sem-Ent)` are where Semaphore gates it — we can ship them open. 100+
items is expected; pick the highest-leverage ones first.

### A. Workflows / pipelines *(flagship — Semaphore v2.21)* — **A1 shipped ✅**
- ✅ **Workflow templates** — chain templates into a pipeline (`workflows` table, steps jsonb; CRUD UI
  with an ordered step editor: name + template + condition + reorder/remove).
- ✅ **Workflow execution engine + run dashboard** — `startWorkflow` → leader-aware run launch per step;
  the run-terminal hook (`advanceWorkflow`) advances the pipeline; a **Workflows** page shows recent
  **workflow runs** with per-step status dots + links to each step's run, live over WS.
- ✅ **Conditional branching** — `on_success` / `on_failure` / `always` per step (evaluated against the
  pipeline state). E2E-verified: happy path ran build→deploy + **skipped** the on-failure step; failure
  path **failed** build → **skipped** deploy → ran the **on-failure rollback**.
- ✅ **API + cron triggers** — `POST /api/workflows/{id}/run`, **and a Schedule can target a workflow**
  (`schedules.workflow_id`; the scheduler's `fireSchedule` branches workflow vs template). UI: the schedule
  form has a Template/Workflow target toggle. Verified: a once-schedule on a workflow fired on the tick →
  a workflow run `triggeredBy=scheduler`. **Inbound webhook → workflow** also shipped: a webhook
  integration can target a workflow (`integrations.workflow_id`); `handleWebhook` starts the pipeline
  (payload → workflow variables). UI: a Template/Workflow toggle on the integration form. Verified: POST
  to the public webhook → a workflow run `triggeredBy=webhook`.
- ✅ **Variable passing between nodes** (full — A2a + A2b):
  - **Workflow variables (A2a):** `variables` (key=value) on the workflow → injected as extra-vars into
    **every step**, overridable per-run via `POST /workflows/{id}/run {variables}`. UI key=value editor.
  - **Step output → next steps (A2b):** a step writes a flat JSON object to **`aui_output.json`** in its
    working dir; the runner captures it (artifact mechanism), the engine merges its keys into the workflow
    run's `variables`, and later steps receive them as extra-vars.
  - Verified E2E: a `produce` step exported `{built_version:9.9.9}` → the `consume` step ran with
    `-e {"built_version":"9.9.9"}` and printed it; a workflow-level default + a launch override also won.
- ✅ **Parallel task execution** (fan-out) — a step marked `parallel` joins the previous step's **wave**;
  the engine launches the whole wave at once and only advances when **every** step in it finishes
  (concurrent terminal hooks serialised by `wfMu`). UI: a per-step ∥ toggle + `∥` separators in the
  dashboard. Verified: two parallel `sleep` steps ran **simultaneously** (both running mid-flight) and the
  next sequential step waited for both — ~12 s vs ~15-18 s sequential.
- ✅ Conditional **`when` guards** on steps (beyond success/failure/always) — an optional per-step
  expression (`key` / `!key` / `key == v` / `key != v`) evaluated against the workflow variables (incl.
  launch overrides + step outputs); a runnable step is skipped when it's false. UI: a `when` input per
  step. Verified: with `env=prod`, a `when: env == prod` step ran and `env == staging` skipped; a launch
  override flipped them.
- ✅ **Visual DAG editor** `(Sem-Pro)` — each workflow card now renders its pipeline as a **visual graph**
  (`WorkflowGraph`): steps grouped into sequential **waves** (Start → wave → … → End with → arrows), parallel
  steps stacked in a lane (∥), each node a card with the step/template name, a condition-coloured border
  (green on_success / red on_failure / amber always) + icon, and `when:` / inventory / env override chips.
  Replaces the old flat chip row. Verified via puppeteer screenshot (build → deploy-eu∥deploy-us → rollback
  [when env==prod] → notify → End). *(Drag-drop graph editing of steps is still the ordered-list form.)*
- ✅ Per-node overrides `(Sem-Pro)` — a step can override the template's **inventory** and **environment**
  (`inventoryId` / `environmentId`); e.g. one deploy template → a staging step then a prod step, each with
  its own inventory + environment. UI: per-step inventory + environment Selects. Verified: override steps
  ran with `inventory_name: inv-prod` and `tier: prod` while the default steps used the template's.
  ✅ **per-step vault override done** — a step also carries `vaultCredentialId` (WorkflowStep + WorkflowRunStep,
  jsonb-persisted); `launchTemplateRun` takes a `vaultOverride` that **replaces** the template's vault
  credential(s) for that step. UI: a third per-step Select ("Vault: template default" → a vault credential) +
  a `vault` chip on the step summary (EN+RU). E2E-verified: a step overriding the vault on a vault-less
  template ran with that credential — the secret-access log recorded `credential-use … vault password used by
  run …` for the override cred. Workflow-level **approval gates** work via a step whose template `requiresApproval`.
- ✅ **Workflow versioning + rollback** `(Sem-Ent)` — every edit (and rollback) first snapshots the current
  definition into `workflow_versions` (auto-incrementing version, name/desc/steps/variables/actor). A "Versions"
  (History) action on each workflow lists the history; **rollback** restores a past version (snapshotting the
  current first, so it's reversible). `GET /api/workflows/{id}/versions` + `POST …/rollback/{versionId}`.
  Verified E2E: A→B→C produced v1(A)/v2(B); rollback to v1 restored name "A" + step "s1". **Workflow RBAC** is
  already covered by the per-project capability gates (`requireProjectCap` CapEdit on every workflow mutation,
  incl. run/edit/rollback). ✅ **Cross-project workflows done** — a step's `templateId` may belong to ANOTHER
  project; the engine already loads step templates globally and `buildRunFromTemplate` attributes the run to the
  template's project, so a cross-project step **runs in the template's project**. Added: a per-step picker that
  lists templates across every project the caller can run in (`GET /api/templates/all`, each hydrated with
  `projectName`; the step Select labels them `Project / Template`), and an RBAC gate — `validateCrossProjectSteps`
  rejects (403) a create/update whose step targets a project the actor lacks **run** access to. E2E-verified: a
  workflow in a NEW empty project, with a step targeting the Demo project's template, ran the step **in the Demo
  project** (run `projectId=demo`, success) while the new project had 0 runs; the picker showed
  "Demo · Ansible Examples / Hello World".

### B. Inventory *(Semaphore v2.19)*
- ✅ **Multiple inventories per template** + runtime selection — a template stores a curated
  `inventoryIds` list (alongside the default); the editor has a checkbox multi-select, and the run dialog
  constrains the inventory picker to that list (default + curated) so you choose at launch (e.g. one deploy
  template → staging or prod). Verified: list stored + round-tripped; a run picking the prod inventory used it.
- ✅ Optional inventory field — a template/run needs no inventory (the launcher offers "project default
  / none"; `-i` is only added when one is set, else the project's `inventory.ini` or ansible's implicit
  localhost is used). Verified: a run with no inventory selected succeeded.
- ✅ **URL / HTTP inventory source** — an inventory of type `url` whose content is an HTTP(S) URL,
  fetched at launch (30s/5 MB cap) and written as the inventory. Verified: a play targeting a group that
  existed only in the URL-served inventory ran successfully (`ran-on-localhost`, `ok=1`).
- ✅ **View inventory content in the UI** (resolved hosts/groups) — a "View hosts" (Network icon) action on
  each inventory opens a modal that resolves it via the runner's `ansible-inventory --list` (new runner
  endpoint `POST /v1/inventory/list` + `runnerclient.ListInventory`), so static/file/dynamic inventories show
  their real groups → hosts exactly as a run would target them (children groups included). `GET /api/inventories/{id}/hosts`
  parses ansible's JSON into `{groups:[{name,hosts,children}],hosts,total}`. Cloud/url note that they resolve
  at run time only. Verified E2E: the seeded file inventory resolved 4 hosts across web/db/local/datacenter;
  a static multi-group inventory resolved web(2)+db(1)=3.
- ✅ **SSH connection credentials / multiple SSH keys per inventory** `(Sem-Pro)` — a (non-cloud) inventory
  can reference one or MANY Key Store SSH credentials (`inventories.conn_credential_ids` jsonb). At launch
  `launchRun` resolves their private keys → `ExecSpec.SSHPrivateKeys` + sets `--user` from the first
  credential's login; the runner's `startAnsible` writes each key to a 0600 temp file and loads them into a
  per-run **ssh-agent** (`SSH_AUTH_SOCK`, `ANSIBLE_HOST_KEY_CHECKING=False`), so ansible authenticates to the
  hosts and **tries each key**. Keys never touch the command line/log (agent only) and travel in the
  AES-encrypted launch envelope; the agent is killed + key files removed when the run ends. UI: a multi-select
  "SSH connection keys" + the connection user on the inventory form (EN+RU). E2E-verified against a real sshd
  host (`authorized_keys`-only): a run WITH the inventory's key → success (`connected-as-deploy`, ok=2); after
  stripping it → `UNREACHABLE … Permission denied (publickey)`. *(Finer per-HOST credential overrides remain
  available the ansible-native way — per-host `ansible_user`/connection vars in the inventory content.)*
- ✅ Expose inventory name in the task/run context — a run with a named inventory injects
  `inventory_name` as an extra-var (doesn't clobber a user var). Verified: `debug: var=inventory_name`
  printed `"inventory_name": "production"`.
- ✅ **Restrict allowed inventories per template** `(Sem-Ent)` — a template's `inventoryIds` is the allowed
  set; the UI picker is already constrained to it, and now the **server enforces** it: `templateInventoryAllowed`
  (the default `inventoryId` ∪ `inventoryIds`; empty list = unrestricted) gates both the interactive launch
  (`handleRunTemplate` → 400) and the workflow/schedule path (`launchTemplateRun` → error), so an API client
  can't bypass the picker. Unit-tested (8 cases) + verified E2E: disallowed inventory → 400
  "inventory not allowed for this template", allowed + default → 202 queued.
- ✅ Central host registry + **host fact storage & visualisation** `(Sem-Ent)` — a **Hosts** page (registry)
  listing every host with stored Ansible facts + a summary (OS/distro/IP/kernel/gathered-at) and a per-host
  searchable fact browser. Facts are gathered on demand: "Gather facts" (in an inventory's hosts modal) →
  `POST /api/inventories/{id}/facts/gather` → the runner runs `ansible <pattern> -m setup --tree` (new
  `/v1/facts/gather` endpoint + `runnerclient.GatherFacts`) → each host's `ansible_facts` is upserted into
  `host_facts` (keyed by host). `GET /api/hosts` (+ `/{host}`, admin DELETE). Verified E2E: gathering the
  demo inventory stored 4 hosts; the registry showed OS/kernel and the per-host view exposed
  `ansible_architecture`/`ansible_python_version`/etc.
- ✅ **Inventory-to-runner affinity** `(Sem-Pro)` — an inventory carries a `runnerTag` (`inventories.runner_tag`,
  idempotent migration); when a run uses that inventory `launchRun` makes the inventory's tag **win over** the
  template/run tag, so its hosts dispatch to a runner advertising the tag (e.g. a runner inside that network
  segment). UI: a "Runner tag (affinity)" field on the inventory form (EN+RU); also threaded through project
  export/import + apidoc. E2E-verified on the VM with a git-backed project + a tagged `edge-runner`: a template
  with **no** runner tag whose inventory is tagged `edge` ran on **edge-runner**; the same playbook with an
  untagged inventory ran on the **built-in** runner.
- ✅ **Inventory monitoring** — real-time reachability monitoring + proactive alerts + custom thresholds.
  "Monitor" on an inventory's hosts modal creates a per-host monitor (`host_monitors`); the **leader** pings
  them ~every 2 min (runner `ansible -m ping --tree` via `/v1/host/ping`), updating up/down/unknown +
  consecutive-failure count, and on the **up→down transition** (after `threshold` consecutive fails, default 3)
  fires a **`host-down`** notification to subscribed channels (recovery → `host-up`); both new events are in
  the channel event picker. The **Hosts** page shows a live status column (up/down/unknown). `POST /api/monitors/check`
  runs a check on demand. Verified E2E: a TEST-NET host (192.0.2.1) went **down** (consecFails 1, threshold 1)
  and a capture channel received the host-down alert, while the local host stayed **up**.

### C. Auth, SSO & RBAC
- ✅ **LDAP / OIDC group → role mapping** `(Sem-Ent)` — admin-editable rules (`group → role`) in
  app_settings, applied on each external login: the user's groups (LDAP `memberOf`-style attr / OIDC
  groups claim, both configurable) are matched (case-insensitive substring, so `admins` matches
  `cn=admins,…`) and the **highest** role wins (admin > user), else the default role; the stored role is
  updated so the directory stays authoritative. Settings UI card. Mapping logic + claim extraction
  **unit-tested**; settings round-trip verified. *(Full IdP-flow E2E needs a live directory.)*
- ✅ **Custom roles** beyond viewer/editor/admin `(Sem-Ent)` — capability-based RBAC. Four granular caps
  (**view/run/edit/manage**); the 3 built-ins map to fixed cap sets, and admins define **custom roles**
  (`custom_roles` table + `/api/roles` CRUD + a "Custom roles" card in Settings with permission checkboxes).
  A project member's role may now be a built-in **or a custom-role id**; every project-scoped guard was
  converted from rank comparison to a capability check (`requireProjectCap` / `projectCaps`) — run launches
  need `run`, edits need `edit`, member/settings need `manage`. Members tab's role picker lists custom roles.
  Verified E2E: an `operator` role = {view,run} could **launch a run (202) but not edit a template (403)** —
  run-without-edit, impossible under the old rank model; built-ins + global-admin unchanged.
- ✅ **SAML** authentication (AWX/Tower) — a SAML 2.0 Service Provider (`crewjam/saml`, the SP-side XML-sig
  is library-handled, not hand-rolled). Endpoints: `/api/auth/saml/metadata` (SP metadata to register at the
  IdP), `/api/auth/saml/login` (SP-initiated AuthnRequest → redirect), `/api/auth/saml/acs` (the IdP POSTs the
  signed SAMLResponse → links by email / upserts → group→role mapping from group attrs → sign in). SP keypair
  is configurable or auto-generated; config `SAML_ENABLED/IDP_METADATA_URL/ROOT_URL/ENTITY_ID/USERNAME_ATTR/DEFAULT_ROLE`.
  Login page shows a "Sign in with SAML" button; System Info reports it. Verified against a real test IdP
  (simplesamlphp): IdP metadata parsed at boot, SP metadata served (EntityDescriptor + ACS), and `/saml/login`
  302-redirects to the IdP SSO URL with a `SAMLRequest`. *(Full assertion round-trip needs a browser login.)*
- ✅ **Email OTP** login — a per-user second factor that emails a single-use 6-digit code (sha256-hashed,
  10-min expiry, `email_otps` table) instead of TOTP, via a new **global SMTP** config (Settings, admin).
  Login returns `{emailOtpRequired}` → resubmit with the code. Admin 2FA-reset clears it. E2E-verified with
  mailpit: code emailed + accepted, wrong code 401, **main admin (no 2FA) password login unaffected**.
- ✅ **RADIUS** auth (Tower) — hand-rolled **RFC 2865** Access-Request (PAP, no SDK) in `internal/api/radius.go`:
  builds the packet (User-Name + obfuscated User-Password + NAS-Identifier), sends UDP, verifies the
  Response-Authenticator, and on Access-Accept upserts the user (like LDAP). Wired into `handleLogin` **after
  local + LDAP** (so a RADIUS outage never locks out a local admin; connectivity failures → `errRADIUSUnavailable`
  → fallback WARN). Config via `RADIUS_ENABLED/SERVER/SECRET/NAS_ID/DEFAULT_ROLE`; shown in System Info.
  Password obfuscation unit-tested; verified E2E against a RADIUS server (independent Python impl that decrypted
  the PAP password): raduser/radpass → 200, wrong → 401, local admin → 200.
- ✅ **TACACS+** auth (Tower) — hand-rolled **RFC 8907** ASCII-login (no SDK), `internal/api/tacacs.go`:
  12-byte header + chained-MD5 body obfuscation; flow AUTHEN START(username) → REPLY GETPASS → CONTINUE(password)
  → REPLY PASS/FAIL over TCP. Wired into `handleLogin` **after local/LDAP/RADIUS** (outage → `errTACACSUnavailable`
  → local fallback). Config `TACACS_ENABLED/SERVER/SECRET/DEFAULT_ROLE`; shown in System Info. Verified E2E
  against an independent Python TACACS+ server (decrypted the password): tacuser/tacpass → 200, wrong → 401,
  local admin → 200.
- ✅ **OIDC polish: PKCE, restrict login by claim, SSO auto-login, provider logout, account linking.**
  **PKCE** — the login flow generates a verifier (cookie) + S256 `code_challenge`, verified on Exchange
  (`oauth2.GenerateVerifier`/`S256ChallengeOption`/`VerifierOption`). **Restrict by claim** — `auth.oidc_required_claim`
  ("key" / "key=value", e.g. `hd=example.com`, `groups=eng`) gates the callback (`claimSatisfies`, unit-tested).
  **SSO auto-login** — `auth.oidc_auto_login` → `/api/auth/status.oidcAutoLogin`; the Login page redirects
  straight to the IdP (escape hatch `?local=1`). **Provider logout** — `/api/auth/oidc/logout` clears the
  session + redirects to the discovered `end_session_endpoint` (RP-initiated). **Account linking** — the
  callback links by email (`GetUserByEmail`) to an existing account instead of creating a duplicate. Settings
  in the Group→role card. Verified: PKCE params in the real Google-issuer redirect, autoLogin flag round-trip,
  claim predicate unit-tested.
- ✅ **LDAP local-auth fallback + debug logging** — login already tries local accounts *first*, so a directory
  outage never locks out a local admin (the fallback). `authenticateLDAP` now distinguishes
  **connectivity/config failures** (dial/StartTLS/service-bind/search) — wrapped in `errLDAPUnavailable` — from
  genuine auth rejections; `handleLogin` logs a WARN "ldap unavailable — local accounts can still sign in"
  on the former. An admin-toggled **`auth.ldap_debug`** setting (in the Group→role card) logs every step
  (dial/bind/search/user-bind) at INFO so directory problems are diagnosable without raising the global level.
  Verified E2E with LDAP pointed at a dead host: admin/local login → 200 (fallback), a directory user → 401
  with `[ldap] dialing` → `[ldap] dial failed` → the fallback WARN in the log.
- ✅ **Pluggable authentication architecture** `(Sem-Ent)` — satisfied by the comprehensive multi-provider
  login chain: local → LDAP → RADIUS → TACACS+ (ordered, with local-first fallback), plus redirect SSO via
  OIDC / SAML / GitHub / Bitbucket, layered with 2FA (TOTP) and Email-OTP. Each method is independently
  config-gated and pluggable in/out; `handleLogin` tries the password backends in order and the SSO providers
  are separate endpoints. *(A runtime external-plugin loader SDK is intentionally out of scope — the built-in
  set covers the enterprise need.)*
- ✅ Feature flag: **"Non-admin can create project"** toggle (Settings → Feature flags; project creation
  is admin-only unless on — `app_settings` `flags.nonadmin_create_project`). Verified: viewer→403 off, 201 on.

### D. Secrets, Key Store & Repositories *(Semaphore v2.18–v2.19)*
- ✅ **Persistent Repository Cache** — a per-repo **`cacheEnabled`** flag (default on) keeps a warm mirror
  that the **leader background-syncs** (`syncCachedRepos` in the scheduler tick, ~every 10 min, due-gated by
  `lastSyncedAt` so idle repos aren't hammered) in addition to the existing **manual "Sync now"** and the
  per-run pull. Sync **status** (syncing/ready/error + lastCommit + lastSyncedAt + lastError) is persisted
  (`SetRepoStatus`) and shown per-repo (status dot + "cached" chip + toggle). Verified E2E: a freshly-added
  cache repo reached `ready` with a real commit + `lastSyncedAt` within ~35 s **with no manual sync call**
  (the background sweeper did it). ✅ **Dirty-tree / force-push recovery + stale-clone GC done:** the runner's
  repo update path (`git.go Sync`) now forces the checkout to EXACTLY match origin — `fetch --prune --force`,
  `checkout -f -B`, `reset --hard origin/<branch>`, then `git clean -fd` — so a dirty working tree (leftover
  untracked files from a prior run) and a force-pushed (rewritten-history) upstream both recover into a
  pristine, reproducible tree. A leader sweep `gcStaleRepoClones` (hourly + on becoming leader) removes
  orphaned `DataDir/repos/<id>` checkouts whose repository no longer exists. E2E-verified: after dirtying the
  checkout AND force-pushing a rewritten commit, a re-sync went `ready` with the new commit, the local mod
  discarded, the stale untracked file cleaned, and the new origin file present; and an orphan clone dir was
  removed on api restart while the live repo's clone was kept.
- ✅ **User-owned (personal) secrets** — a credential can be marked **personal** (`credentials.owner_user_id`
  = the creator); `ListCredentials` returns shared + project-scoped + the caller's own personal, and never
  another user's personal. UI: a "Personal" toggle on create + a Personal badge on the row. E2E-verified:
  a viewer can't see the admin's personal cred (but sees shared); the admin can't see the viewer's.
- ✅ **Credentials for `requirements.yml`** — auth for private galaxy collections/roles + custom
  `ansible-galaxy` CLI args. Admin sets a galaxy server URL + token + extra args (Settings → "Private
  Galaxy / Automation Hub", `app_settings` `galaxy.*`, `internal/api/galaxy.go`); at launch `galaxyEnv`
  injects `ANSIBLE_GALAXY_SERVER_LIST/_URL/_TOKEN` into the run env and `galaxyArgs` appends to the
  runner's `ansible-galaxy install` (`spec.GalaxyArgs` → `StartGalaxy`). E2E-verified against a mock
  galaxy server: `ansible-galaxy install` hit the configured URL carrying `Authorization: Token <token>`
  and the custom arg reached the CLI (`unrecognized arguments: --bogus-zzz-flag`). The runner already
  auto-detects per-playbook requirements files (`GalaxyRequirements`: requirements.yml/.yaml +
  roles/ & collections/ variants).
- ✅ Show SSH **public key** — `GET /api/credentials/{id}/pubkey` derives the authorized_keys-format
  public key from a stored SSH private key (x/crypto/ssh; handles passphrases); a "Show public key"
  button on the credential opens a copyable view. Verified: derived key == `ssh-keygen -y`.
- ✅ **SSH certificate** support — an SSH credential can carry an OpenSSH user certificate (the
  CA-signed `*-cert.pub`). It's stored as **public metadata** (`credentials.ssh_certificate` plaintext
  column, returned to clients — not in the encrypted secret blob) and threaded onto `GitSyncSpec`/
  `GitCommitSpec`. When the runner uses the key for git, `sshGitCommand` (git.go) writes the cert to a
  temp file and presents it via `ssh -o CertificateFile=…`, so a key signed by a trusted CA
  authenticates even when its bare public key isn't an installed authorized_key. UI: an "SSH
  certificate" textarea on the credential form + a "cert" badge on the row (EN+RU). **E2E-verified**
  against an sshd git server that trusts a CA and has `AuthorizedKeysFile none` (only CA-signed certs
  can authenticate): a repo sync WITH the cert → `ready` (cloned); after stripping the cert → `error`
  / `Permission denied (publickey)`. ssh `-vvv` confirmed `Offering public key: …-cert.pub ED25519-CERT`.
- ✅ Docker / Kubernetes secrets — any config value reads a `<KEY>_FILE` env var pointing at a mounted
  secret file (e.g. `APP_SECRET_FILE`, `DATABASE_URL_FILE`), trimmed, falling back to the plain var then
  the default. Single chokepoint in `config.env()`. Unit-tested (`config_test.go`).
- ✅ **Secret access audit trail** `(Sem-Ent)` — dedicated `secret_access_log` (actor, action, credential,
  detail, time), kept separate from the activity feed. Recorded at every access to secret material:
  `pubkey` (deriving a key's public half), `run-secrets` (admin revealing a run's secret vars), and
  `credential-use` (a run consuming a Key Store vault credential — actor = the run's triggeredBy). Admin-only
  `GET /api/secret-access-logs` → a live-polling "Secret access audit" card in Settings; trimmed by the
  audit-retention sweep. Verified E2E: deriving an SSH credential's pubkey logged
  `{actor:admin,action:pubkey,credential:sa-test}`; viewer → 403, admin → 200. *(Global access keys =
  already covered by shared credentials — project_id NULL makes a key usable across all projects.)*
- ✅ **CyberArk integration** (Tower) — a **CyberArk Conjur** secret backend (`cyberark`) in the existing
  secret-backends framework (hand-rolled REST, no SDK): `conjurAuthenticate` exchanges the API key for a
  base64 access token (`POST /authn/{account}/{login}/authenticate`, `Accept-Encoding: base64`), then
  `conjurSecretRead` GETs `/secrets/{account}/variable/{path}` with `Authorization: Token token="…"`. Field
  reuse: address=Conjur URL, namespace=account, accessKeyId=login (host/…), token=API key; a referenced
  secret's path is the variable id. Test + resolve + UI form (+ i18n) wired like Vault/AWS/Azure/GCP. Verified
  E2E against a mock Conjur: test→ok (wrong key→401), and a run with an env external-secret ref authenticated
  + read `myapp/db/password` and injected it as an extra-var.

### E. Notifications *(Semaphore v2.19 — we have Telegram/Slack/Webhook/Email)*
- ✅ More channels — **Microsoft Teams · Discord · Gotify · Rocket.Chat · Ntfy · Google Chat · Pushover ·
  DingTalk · PagerDuty · Opsgenie** added (hand-rolled posts in `notify.go`; per-channel config fields in the UI).
  E2E-verified via a capture server: each posts its correct format (Discord `{content}`, Teams MessageCard,
  Gotify `/message`+`X-Gotify-Key`, ntfy body+`Title`, DingTalk `{msgtype:text}`, **PagerDuty** Events
  API v2 `{routing_key,event_action:trigger,payload:{severity,source},dedup_key}` with severity from the
  run status, and **Opsgenie** Alert API v2 `POST /v2/alerts` with `Authorization: GenieKey <apiKey>` +
  `{message,description,priority,alias,source}` (priority P1 on failed/canceled else P3) — both with an
  overridable endpoint for EU-region/on-prem/testing).
- ✅ **Per-project channel selection** — a channel optionally scopes to one project (`project_id`,
  null = all); dispatch skips channels bound to a different project. UI: a project Select on the channel
  form + a scope chip on each row. Verified: a Demo run hit the Demo-scoped + all-projects channels, not
  a channel scoped to another project.
- ✅ **"Fixed" notifications** — a channel can subscribe to the synthetic **`fixed`** event; it fires
  only on a **recovery** (success after the template's previous terminal run failed/canceled), and the
  message reads "recovered ✓". Recoveries bypass the suppress-success flag. Verified: fail→recover hit a
  `fixed`-only channel exactly once; a plain success did not. ✅ **email-on-success** is covered by the
  per-channel event subscription — a channel (incl. email/SMTP) with `success` in its `Events` fires on every
  successful run (`eventMatches`), so no separate redundant toggle is needed. Verified: a `success`-subscribed
  webhook channel got exactly one hit on a successful run.
- ✅ **Disable all notifications per template** — a "Mute all notifications" toggle on the template
  (`suppress_all_notifications`); `dispatchNotifications` returns early for it. Verified via a capture
  server: a muted template's run produced **0** webhook hits, a normal template's run produced **1**.
- ✅ **Long-running-task alerts** `(Sem-Pro)` — a `tasks.long_run_alert_sec` threshold (Settings → Task
  settings); the **leader** scheduler scans `running` runs older than it (`ListLongRunningRuns`) and fires
  a one-time **`long-running`** notification (de-duped in-memory, cleared on finish) to channels subscribed
  to that event. Verified: a `sleep 60` run triggered the alert ~14 s after a 5 s threshold.
- ✅ **Rich outbound webhook payload** — the generic `webhook` channel now posts the structured run
  fields (`run/status/project/app/playbook/commit/version/actor/exitCode/id`) alongside the existing
  `title/text/link` (backward-compatible) — usable by automation. Verified via a capture: all fields present.
- ◐ Telegram **thread/topic** support — a `threadId` config field sets `message_thread_id` (forum topics);
  shipped + UI field (verified structurally; needs a live bot to capture).
- ✅ **Notification egress proxy (alert-proxy)** — outbound chat/webhook alerts can route through an
  HTTP/HTTPS/SOCKS5 proxy for restricted/egress-controlled networks. `notify.SetProxy` swaps the package
  HTTP client's transport (atomic pointer, race-free with in-flight dispatches); applied at boot + on save
  from the `notify.proxy_url` setting (admin `GET/PUT /api/settings/notify-proxy`). UI: a "Notification
  egress proxy" card in Settings (EN+RU). Email egress is unaffected (direct SMTP). E2E-verified: with the
  proxy set, a successful run's notification routed **through** a forward-proxy container (1 `PROXY_FORWARD`,
  then delivered to the capture server); clearing it restored direct egress.
- ✅ **Notification audit log + role-based notification access** `(Sem-Ent)` — every dispatch attempt is
  recorded in `notification_log` (channel id/name/type, run id/name, project, event, ok + error) inside
  `dispatchNotifications`; admin-only `GET /api/notification-logs` (viewer → 403, admin → 200) surfaces it in
  a live-polling "Notification delivery log" card in Settings (uniform StatusBadge: delivered/failed, error
  inline). Trimmed by the audit-retention sweep. Verified E2E via capture: a live channel logged `ok:true`,
  a dead-host channel logged `ok:false` with the dial error; viewer got 403, admin 200.

### F. Observability & logging
- ✅ **Syslog forwarding / log export** — Settings → Syslog: each finished run is shipped to an external
  collector as an **RFC 5424** message (UDP/TCP, configurable app-tag; severity from status, MSG carries
  run/status/project/app/exit/by/id). Verified against a UDP capture: `<131>`(err)/`<134>`(info) lines
  arrived with the right fields. Plus an **HTTP event-collector sink** (Splunk HEC shape:
  `{event,sourcetype,time}` + `Authorization: Splunk <token>`; generic collectors accept the same) —
  verified against a capture: POST to `/services/collector` with the HEC auth header + JSON event.
- ✅ Built-in dashboard analytics — the Insights page now has a **per-template SLO breakdown** (runs,
  success-rate, avg duration over the window; busiest first, success-rate colour-coded) on top of the
  per-day chart + totals. `RunInsights` adds the grouped query. Verified: ins-ok 2 runs/100%, ins-fail
  1 run/0%. *(per-runner trends could follow the same pattern.)*

### G. HA & cluster
- ✅ **HA Cluster Dashboard** — a **Cluster** page (`GET /api/cluster`, admin): **leader/standby** badge
  for this replica (a `leader atomic.Bool` set by `leadAndSchedule`), per-runner **capacity** cards
  (active/max with a fill bar, online dot, tags) and live **Queued / Running** task tables (task ·
  project · runner · status · when), polling every 2s. Verified: with an `edge` runner capped at 1, a
  `sleep` run showed edge at **1/1** + the running task; the curl showed running=1/queued=1. *(Semaphore's
  per-node "claims/aliases" need a replica registry — covered once we add multi-node node tracking.)*
- ✅ **Multi-replica run-stream routing** — `/ws/runs/{id}` now works from ANY replica with **no sticky
  ingress or pub/sub bus**: when a run isn't streaming on the receiving replica but its DB status is
  non-terminal, `handleRunWS` falls back to `tailRunWS` (ws.go), which replays + polls the
  Postgres-persisted output (the owning replica already flushes it ~1/s via `UpdateRunLog`) and pushes new
  bytes until a terminal status. Cross-replica **cancel** too: a non-owning replica's cancel flags
  `runs.cancel_requested`; the owning replica's `manager.watchCancel` goroutine polls it and cancels the live
  process. The last piece for true active-active. **E2E-verified with two replicas** (api1 leader + api2
  standby, shared DB): a run launched on the cluster streamed 1566 bytes of live output + `end` when tailed
  from the **standby** api2; a cancel POSTed to api2 ended the run `canceled` (the leader picked up the flag).
- ✅ Optional Redis-backed task pool — **N/A by design**: the dispatch queue + run-stream routing are all
  Postgres-backed (claimable queue + incremental output tail), so no Redis is needed for active-active.

### H. Task settings & limits *(Semaphore System Information screen)*
- ✅ **Max task duration** — Settings → Task settings (minutes; 0 = unlimited; `tasks.max_duration_sec`).
  Enforced **on the runner** (`ExecSpec.TimeoutSec` → it cancels the process + prints a notice). Verified:
  a 5 s limit auto-canceled a `sleep 20` run at ~5 s (status failed, "exceeded max duration" in the log).
- ✅ **Max tasks per template** — a numeric `maxConcurrent` per template (0 = unlimited); `launchRun`
  blocks (409) when the template already has that many queued/active runs (`CountActiveRunsByTemplate`).
  UI: a "Max concurrent runs" input (shown when allow-parallel is on). Verified: cap 1 → 2nd launch 409
  while the 1st ran, allowed after it finished; cap 2 → 3rd launch 409.
- ✅ **Global max parallel runs** — a system-wide cap `tasks.max_parallel` (0 = unlimited) mirroring the
  per-template one: `launchRun` checks `store.CountActiveRuns` (all queued/pending/running across every
  project) and rejects a launch with 409 when at the cap. Admin sets it in the Task-settings card next to
  max-duration / long-run-alert. Verified E2E: cap 1 → run #1 queued (202), an immediate run #2 → 409
  "system is at its global max parallel runs (1)"; reset to 0 → launches flow again.

### I. Diagnostics, API & portability
- ✅ **System Information page** — Settings → System information (`GET /api/system`, admin): version /
  Go / platform, DB dialect, **enabled auth** (local/TOTP/LDAP/OIDC/GitHub/Bitbucket/Email-OTP) +
  **notification capabilities** (configured per channel type), HA + remote-runner status + online count,
  task limits, feature flags — ✓/✗ rows matching Semaphore's modal.
- ✅ **API "Models" / schemas** in the Explorer — a **Models** tab listing 14 entities (Project, Template,
  Inventory, Run, Workflow, …) with each field's name + type. Schemas are **reflected from the Go structs'
  json tags** (`apimodels.go`), so they never drift; types render friendly (string / integer / boolean /
  timestamp / `X[]` / `X?` nullable / nested struct names). Served in the `/api/meta/endpoints` payload.
  Verified: 14 models, accurate fields (incl. freshly-added ones). Closes the "у нас нет Models" gap.
- ✅ **Project export / import** — `GET /api/projects/{id}/export` serialises a project (inventories,
  templates, workflows, recurring schedules) to a portable JSON bundle with **name-based cross-refs**;
  `POST /api/projects/import` recreates it in a new project, remapping names→fresh ids (inventory links,
  deploy→build links, workflow step templates, schedules). Secrets/credential/repo links are excluded
  (re-link after). UI: Export button per project card + Import on the Projects header. E2E-verified: a
  build+deploy+workflow+schedule project round-tripped with every cross-ref correctly remapped. ✅ **CLI
  done** — `deploy/aui-cli.sh` (`list` / `export <projectId>` / `import`) is a thin Bearer-token wrapper over
  the same endpoints, so a bundle can be version-controlled, promoted between instances
  (`aui-cli.sh export … | aui-cli.sh --url https://staging … import`), or wired into CI. E2E-verified: exported
  the demo (inv=1, tpl=12) and imported it into a fresh project, then swept. (Parity — Semaphore ships CLI
  export/import in its open-source core too.)
- ✅ **AI assistant via MCP** — a Model Context Protocol server (`POST /api/mcp`, Streamable HTTP, JSON-RPC 2.0:
  `initialize` / `tools/list` / `tools/call` / `ping`) exposing the control plane to an AI agent. Tools:
  **list_projects, list_templates, run_template, get_run, list_runs**. Auth reuses the API session/Bearer-token
  guard, so the agent acts as a real user and **project capabilities are enforced** (run_template checks
  `CapRun`). Documented in the API Explorer. Verified E2E with an API token: no-auth → 401; initialize returned
  protocolVersion 2024-11-05 + serverInfo; tools/list returned all 5; `tools/call run_template` launched a real
  run (isError:false).

### J. Execution backends
- ✅ **Pulumi** support — a new `pulumi` app alongside Ansible/Terraform/OpenTofu/Terragrunt/scripts. The
  runner installs the Pulumi CLI (best-effort, like the Terraform fallback; the **YAML runtime is bundled**
  so no extra language toolchain is needed) and `startPulumi` runs `pulumi stack select --create <workspace>`
  then **preview / up / destroy** against a **local file backend** (`PULUMI_BACKEND_URL=file://…`, empty
  passphrase) — no Pulumi Cloud login. App picker + per-app actions (preview/up/destroy) in the template form
  & launcher; a localhost-safe YAML demo (`examples/project/pulumi/`) + a seeded "Pulumi · preview" starter.
  Verified E2E on the VM: the preview run succeeded — created stack `dev`, rendered the `message` output.
- ✅ Terraform **HTTP backend** `(Sem-Pro)` — a terraform/tofu template can set an HTTP **state backend URL**
  (`tfBackend` on the template); at run the runner drops a `backend "http" {}` override and inits with
  `-backend-config=address=/lock_address=/unlock_address=<url>`, so state lives at that URL (the override is
  removed afterwards, preserving the exit code). UI: an "HTTP state backend" field on the template form
  (terraform/tofu only). Verified E2E against a capture backend: the run configured the http backend and the
  capture received the full state protocol — `GET → LOCK → GET → UNLOCK /aui-state` — with plan succeeding.

### K. Enterprise / compliance *(longer-horizon)*
- ✅ Detailed **compliance reporting** (PCI-DSS / HIPAA / SOX / FedRAMP-style audit exports) (Tower) — a
  consolidated, date-ranged JSON report `GET /api/compliance/report?from=&to=` (admin; default last 30d,
  downloads as `compliance-<date>.json`) aggregating: the **access-control matrix** (every user + role + 2FA/
  email-OTP posture), **activity-by-action** counts, **runs-by-status** counts, the **complete secret-access
  trail** in range, and notification-delivery counts (incl. failures) — built on the audit surfaces shipped
  this cycle ([[secret-backends]] secret-access log + the notification-delivery log). Per-surface CSV exports
  (`/api/audit/export`, `/api/runs/export`) remain. UI: a "Compliance report" button beside the CSV exports.
  Verified E2E: the report returned the admin access row, activity/run breakdowns, the pubkey secret-access
  event, and the attachment header; viewer → 403, admin → 200.
- ✅ **Air-gapped (offline) install bundle** — `deploy/bundle.sh` produces a single self-contained
  `dist/ansible-ui-airgapped-<tag>-<date>.tgz` that `docker save`s the api + runner + postgres images
  (+ SHA-256 checksum + MANIFEST of image ids/sizes) alongside `docker-run.sh`, the Helm chart, and an
  offline `install.sh`. On a host with **no internet/registry**, `install.sh` verifies the checksum,
  `docker load`s the images (no pull), and starts the stack (`load`/`down` sub-commands too). E2E-verified
  on the VM: built a 740 MB bundle, extracted it, and ran `install.sh load` → checksum `OK` + `Loaded image:`
  for all three. Documented in README (Air-gapped install). *(Multi-instance "licensing" is a
  commercial-policy choice, not an engineering gap — intentionally not built; the product imposes no
  license/seat enforcement.)*
- ◐ External logging integrations — the log-export HTTP sink (§F) now has a **format** selector:
  **Splunk HEC** (`{event,…}` + `Authorization: Splunk <token>`), **Elasticsearch** (flat doc → `…/_doc`
  with the token used verbatim as the auth header, e.g. `ApiKey …`), and **generic**. Verified via capture:
  ES sends a flat doc + `ApiKey abc`, Splunk wraps + `Splunk …`. **PagerDuty + Opsgenie** incident channels
  shipped (see §E) — both verified via capture with overridable endpoints.

### Small follow-ups (already-noted)
- ✅ `ClearRuns` artifact-dir sweep — bulk clear now returns the deleted ids and removes each
  `DataDir/artifacts/<id>` dir (verified: artifact dir gone after clear).
- ✅ **QR code on 2FA enrolment** — the setup endpoint now renders the otpauth URL to a PNG QR
  (`github.com/skip2/go-qrcode`, server-side) and returns it as a `qrDataUri` data: URI alongside the
  secret; the TwoFactor card shows the QR to scan with a "or enter this key manually" fallback. Verified
  E2E (on a throwaway user): the setup response carried a valid base64 PNG (magic `89 50 4e 47`).

---

## ✨ Beyond Semaphore (our differentiators — all shipped ✅)

- ✅ A genuinely native terminal (real PTY, not a log tail) with replay & late-join.
- ✅ Live-everywhere UI over a WebSocket event bus — consistent colour-coded status, no manual refresh.
- ✅ Multi-app execution (Ansible **and** Terraform/OpenTofu/Terragrunt/Bash/PowerShell/Python) from one
  control plane, with user-configurable backends.
- ✅ **Distributed runner pools** with tags, concurrency caps + queue, and **per-runner repo sync** (no
  shared storage needed).
- ✅ **GitOps file editing** — edit a repo file in the UI and it lands as a pushed branch + PR, never a
  silently-wiped local change.
- ✅ **Secrets masked-by-default with audited admin reveal** — visibility without leaking plaintext into
  logs or the database.
- ✅ Single-command, fully containerised deploy that needs nothing on the host but Docker.
- ✅ **Four external secret backends, all hand-rolled (no cloud SDK)** — Vault · AWS SigV4 · Azure AAD ·
  GCP SA-JWT.
- ✅ **Turnkey cloud inventory across AWS EC2 / GCP Compute / Azure** — the runner generates the plugin
  config; creds flow as process env, never into the log.
- ✅ **Built-in observability** — Prometheus `/metrics` + an in-app Insights / trends page.
- ✅ **Leader-elected HA scheduling** — multi-replica-safe cron / autorun / retention via a Postgres
  advisory lock.

---

## L. Release & repo readiness (open-sourcing for git)

Polish for publishing the repo publicly. Tracked here so the autonomous loop finishes them too.

- ✅ **Git hygiene / no junk** — `.gitignore` extended (embedded SPA `internal/webui/dist/` with a
  `.gitkeep` so `go:embed all:dist` still compiles, `*.bin`, `/tmp-demo/`, air-gapped bundles, local
  `.claude/settings.local.json`+`*.lock`); a `.gitattributes` forces **LF** on `*.sh`/Dockerfiles/YAML/Go
  (dev is Windows, runtime is Linux — CRLF would break scripts in the container). Verified: `go build ./...`
  still compiles with `internal/webui/dist` containing only `.gitkeep` (fresh-clone safe).
- ✅ **README actualisation + screenshots / GIFs** — added the vs-Semaphore comparison table + a Screenshots
  section: an animated **GIF of the live PTY stream** (puppeteer frames assembled via Pillow) + 5 real
  screenshots (terminal, dashboard, workflows DAG, cluster, insights) committed under `docs/img/`. Also
  enriched the run-log git preamble (repo URL/branch + commit + message) and defaulted the demo
  Full-Stack/Roles templates to `-vv` for Semaphore-parity output (task paths + `[core]` banner) — see
  [[semaphore-reference]].
- ✅ **Docker + Helm actualisation** — `docker-run.sh` now forwards `SAML_SP_CERT`/`SAML_SP_KEY`/`DOCS_ENABLED`
  (the only env vars it lacked). The Helm chart was stale (LDAP/OIDC/GitHub/Bitbucket only) — added **RADIUS,
  TACACS+, SAML** auth blocks (+ `radius-secret`/`tacacs-secret`/`saml-sp-key` in the chart Secret),
  `DOCS_ENABLED`, and a generic `extraEnv` passthrough; bumped the chart to **0.3.0**. (Galaxy + notify-proxy
  need no env — they're runtime DB settings.) Verified: `helm lint` clean, `helm template` renders every new
  env when its provider is enabled, and a from-scratch `docker-run.sh up` came up healthy with data intact.
