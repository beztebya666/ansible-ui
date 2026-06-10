# API reference

Complete REST + WebSocket reference for **ansible-ui**, generated from the live API catalogue (`GET /api/meta/endpoints`) - the same data powering the interactive **API Explorer** at `/api` in the running app. **178 endpoints** across 19 groups.

- **Base URL:** `/api`
- **Auth:** 🔒 endpoints accept a session cookie or `Authorization: Bearer <api-token>`.
- **Access column:** `public` = no auth | `auth` = any signed-in user | **`admin`** = admin/capability-gated.

> The in-app **API Explorer** (`/api`) lets you try every endpoint live and browse entity schemas (the **Models** tab).

## Contents

- [Meta](#meta) (30)
- [Auth](#auth) (21)
- [Projects](#projects) (27)
- [Inventories](#inventories) (7)
- [Templates](#templates) (7)
- [Runs](#runs) (13)
- [Workflows](#workflows) (10)
- [Schedules](#schedules) (5)
- [Repositories](#repositories) (7)
- [Key Store](#key-store) (10)
- [Environments](#environments) (5)
- [Hosts](#hosts) (5)
- [Notifications](#notifications) (6)
- [Integrations](#integrations) (5)
- [Runners](#runners) (4)
- [Applications](#applications) (4)
- [Template Views](#template-views) (4)
- [Users & Tokens](#users--tokens) (6)
- [WebSocket](#websocket) (2)


## Meta

| Method | Path | Access | Summary |
|--------|------|--------|---------|
| `GET` | `/api/activity` | auth | Activity / audit feed |
| `GET` | `/api/audit/export` | **admin** | Download the full audit log as CSV (compliance) |
| `GET` | `/api/cluster` | **admin** | Cluster/HA dashboard — leader flag, per-runner capacity (active/max), and the live queued + running tasks |
| `GET` | `/api/compliance/report` | **admin** | Consolidated compliance report (JSON) over ?from..?to (RFC3339; default last 30d): access-control matrix (users/roles/MFA), activity-by-action, runs-by-status, the full secret-access trail, and notification-delivery counts. |
| `GET` | `/api/health` | public | Liveness probe |
| `GET` | `/api/insights` | auth | Run analytics over a recent window: per-day counts by outcome, success rate, avg duration + dispatch wait. |
| `POST` | `/api/mcp` | auth | Model Context Protocol (MCP) server for AI agents — JSON-RPC 2.0 (initialize / tools/list / tools/call). Tools: list_projects, list_templates, run_template, get_run, list_runs. Auth via session or Bearer API token; tools act as that user (project capabilities enforced). |
| `PUT` | `/api/meta/docs` | **admin** | Enable/disable the API Explorer at runtime |
| `GET` | `/api/meta/endpoints` | auth | This API catalog (powers the Explorer) |
| `GET` | `/api/runs/export` | **admin** | Download run history as CSV (optional ?projectId) |
| `GET` | `/api/secret-access-logs` | **admin** | Secret-access audit trail — one row per access to secret material (action run-secrets \| pubkey \| credential-use, with actor, credential, detail). Optional ?limit (default 100, max 500). Trimmed by the audit-retention policy. |
| `GET` | `/api/settings/auth-mapping` | **admin** | LDAP/OIDC group → role mapping rules + the LDAP group attr / OIDC groups claim + ldapDebug flag |
| `PUT` | `/api/settings/auth-mapping` | **admin** | Set group→role mapping (applied on each LDAP/OIDC login; admin wins, else default role) + toggle ldapDebug (verbose LDAP login logging). Note: local accounts are always tried first, so a directory outage never locks them out. |
| `GET` | `/api/settings/flags` | **admin** | Feature flags |
| `PUT` | `/api/settings/flags` | **admin** | Set feature flags (e.g. allow non-admins to create projects) |
| `GET` | `/api/settings/galaxy` | **admin** | Private Ansible Galaxy / Automation Hub auth (injected as ANSIBLE_GALAXY_SERVER_* for ansible-galaxy install) |
| `PUT` | `/api/settings/galaxy` | **admin** | Set private galaxy server URL + token + extra ansible-galaxy CLI args |
| `GET` | `/api/settings/notify-proxy` | **admin** | Outbound proxy for notification egress (alert-proxy) |
| `PUT` | `/api/settings/notify-proxy` | **admin** | Route chat/webhook notifications through an http/https/socks5 proxy (empty = direct) |
| `GET` | `/api/settings/retention` | **admin** | Retention policy in days (0 = keep forever) |
| `PUT` | `/api/settings/retention` | **admin** | Set retention; a sweep runs hourly + on save |
| `GET` | `/api/settings/smtp` | **admin** | Global SMTP config (email-OTP login codes + default for email notifications) |
| `PUT` | `/api/settings/smtp` | **admin** | Set global SMTP |
| `GET` | `/api/settings/syslog` | **admin** | Syslog / log-export config |
| `PUT` | `/api/settings/syslog` | **admin** | Log export — forward each finished run to a syslog collector (RFC 5424) and/or an HTTP event collector (Splunk HEC: Authorization: Splunk <token>) |
| `GET` | `/api/settings/tasks` | **admin** | Task limits |
| `PUT` | `/api/settings/tasks` | **admin** | Set task limits — maxDurationSec auto-cancels an overrunning run; longRunAlertSec notifies long-running-subscribed channels; maxParallel is a global cap on concurrent runs (new launches → 409 when reached). 0 = off/unlimited. |
| `GET` | `/api/stats` | auth | Dashboard counters + recent runs |
| `GET` | `/api/system` | **admin** | System information — version, runtime, the runner's `ansible --version`, enabled auth (incl. RADIUS) + notification capabilities, task limits, feature flags, runner fleet |
| `GET` | `/metrics` | public | Prometheus metrics (runs by status, active/queued/awaiting, runners + load). Public unless METRICS_TOKEN is set (then Bearer). |

## Auth

| Method | Path | Access | Summary |
|--------|------|--------|---------|
| `POST` | `/api/auth/2fa/disable` | auth | Turn 2FA off (requires a current code) |
| `POST` | `/api/auth/2fa/email/disable` | auth | Disable email-OTP login for the current user |
| `POST` | `/api/auth/2fa/email/enable` | auth | Enable email-OTP login for the current user (needs an email + configured SMTP) |
| `POST` | `/api/auth/2fa/enable` | auth | Verify a code against the pending secret and turn 2FA on |
| `POST` | `/api/auth/2fa/setup` | auth | Begin TOTP enrolment — returns a base32 secret + otpauth:// URI (not yet enabled) |
| `GET` | `/api/auth/bitbucket/callback` | public | Bitbucket OAuth callback (redirect) |
| `GET` | `/api/auth/bitbucket/login` | public | Begin Bitbucket OAuth sign-in (redirect) |
| `GET` | `/api/auth/github/callback` | public | GitHub OAuth callback (redirect) |
| `GET` | `/api/auth/github/login` | public | Begin GitHub OAuth sign-in (redirect) |
| `POST` | `/api/auth/login` | public | Log in (sets the session cookie). If the account has 2FA on, returns {twoFactorRequired:true} (TOTP) or {emailOtpRequired:true} (email) — repeat with the `code`. |
| `POST` | `/api/auth/logout` | auth | Log out (clears the session) |
| `GET` | `/api/auth/me` | auth | The current user |
| `GET` | `/api/auth/oidc/callback` | public | OIDC callback (PKCE verify, required-claim restriction, account-linking by email, sign in) |
| `GET` | `/api/auth/oidc/login` | public | Begin OIDC SSO with PKCE (redirect to the IdP) |
| `GET` | `/api/auth/oidc/logout` | public | Sign out + RP-initiated IdP logout (redirects to the provider's end_session_endpoint if discovered) |
| `POST` | `/api/auth/saml/acs` | public | SAML Assertion Consumer Service — the IdP POSTs the signed SAMLResponse here; links by email + signs in |
| `GET` | `/api/auth/saml/login` | public | Begin SAML SSO (SP-initiated AuthnRequest, redirect to the IdP) |
| `GET` | `/api/auth/saml/metadata` | public | SAML 2.0 SP metadata XML (register this at the IdP) |
| `POST` | `/api/auth/setup` | public | Create the first admin (first run only) |
| `GET` | `/api/auth/status` | public | Setup/login/app state + enabled providers |
| `POST` | `/api/users/{id}/2fa/reset` | **admin** | Admin: clear a user's 2FA (account recovery) |

## Projects

| Method | Path | Access | Summary |
|--------|------|--------|---------|
| `GET` | `/api/projects` | auth | List projects (tenants) |
| `POST` | `/api/projects` | auth | Create a project |
| `POST` | `/api/projects/import` | **admin** | Import a project bundle into a new project (remaps name refs to fresh ids; credential/repo links must be re-added) |
| `DELETE` | `/api/projects/{id}` | auth | Delete a project (project admin) |
| `GET` | `/api/projects/{id}` | auth | Get a project (+ detected playbooks & inventory files) |
| `PUT` | `/api/projects/{id}` | auth | Update a project (name / description / slug) — project admin |
| `GET` | `/api/projects/{id}/branches` | auth | Branches the UI committed + pushed for this project (with PR/MR links) |
| `GET` | `/api/projects/{id}/export` | auth | Export a project's structure (inventories/templates/workflows/schedules) as a portable JSON bundle — refs by name; secrets excluded |
| `DELETE` | `/api/projects/{id}/file` | auth | Delete a file or folder |
| `GET` | `/api/projects/{id}/file` | auth | Read a file |
| `PUT` | `/api/projects/{id}/file` | auth | Write a file |
| `GET` | `/api/projects/{id}/files` | auth | Project file tree |
| `POST` | `/api/projects/{id}/git-commit` | auth | Git-backed projects: commit one or more edited files onto a NEW branch and push it (never the protected base branch). Returns the branch + a PR/MR compare URL. |
| `GET` | `/api/projects/{id}/inventories` | auth | List inventories |
| `POST` | `/api/projects/{id}/inventories` | auth | Create an inventory — type static/file/dynamic/url/cloud. For url, content is an HTTP(S) URL fetched at launch. runnerTag pins runs to a matching runner (affinity); connCredentialIds are Key Store SSH creds ansible connects with. |
| `GET` | `/api/projects/{id}/members` | auth | List project members (per-project RBAC) |
| `POST` | `/api/projects/{id}/members` | auth | Add/update a member's role (project admin); role = viewer\|editor\|admin OR a custom-role id |
| `DELETE` | `/api/projects/{id}/members/{userId}` | auth | Remove a project member (project admin) |
| `GET` | `/api/projects/{id}/playbooks` | auth | List detected playbooks |
| `POST` | `/api/projects/{id}/rename` | auth | Rename/move a file or folder |
| `GET` | `/api/projects/{id}/requirements` | auth | Detected ansible-galaxy requirements (roles/collections from requirements.yml) |
| `GET` | `/api/projects/{id}/templates` | auth | Templates in a project |
| `POST` | `/api/projects/{id}/upload` | auth | Upload a .tar.gz/.zip archive into a local project (multipart 'file'; optional 'password' for encrypted zips) |
| `GET` | `/api/roles` | auth | List custom roles (granular capability sets: view/run/edit/manage) |
| `POST` | `/api/roles` | **admin** | Create a custom role (admin). Permissions are any of view/run/edit/manage. |
| `DELETE` | `/api/roles/{id}` | **admin** | Delete a custom role (admin) |
| `PUT` | `/api/roles/{id}` | **admin** | Update a custom role (admin) |

## Inventories

| Method | Path | Access | Summary |
|--------|------|--------|---------|
| `DELETE` | `/api/inventories/{id}` | auth | Delete an inventory |
| `GET` | `/api/inventories/{id}` | auth | Get an inventory |
| `PUT` | `/api/inventories/{id}` | auth | Update an inventory |
| `POST` | `/api/inventories/{id}/facts/gather` | auth | Gather Ansible facts for the inventory's hosts (runner `ansible -m setup`) and store them in the central host registry. Needs the project `run` capability. Optional ?pattern (default all). |
| `GET` | `/api/inventories/{id}/hosts` | auth | Resolve the inventory's groups + hosts via ansible-inventory (the runner). Works for static/file/dynamic; cloud/url resolve at run time only. |
| `DELETE` | `/api/inventories/{id}/monitor` | auth | Disable monitoring for the inventory's hosts |
| `POST` | `/api/inventories/{id}/monitor` | auth | Enable reachability monitoring for the inventory's hosts (needs `edit`); the leader periodically pings them and alerts subscribed channels on up→down. Optional ?threshold (consecutive failures before DOWN, default 3). |

## Templates

| Method | Path | Access | Summary |
|--------|------|--------|---------|
| `GET` | `/api/templates` | auth | List templates (scoped to active project) |
| `POST` | `/api/templates` | auth | Create a template (any app). type: task\|build\|deploy — deploy ships the latest successful run of buildTemplateId, injecting build_version/build_commit/build_run_id as extra-vars. |
| `GET` | `/api/templates/all` | auth | List templates across EVERY project the caller can run in (each hydrated with projectName) — powers the cross-project workflow-step picker |
| `DELETE` | `/api/templates/{id}` | auth | Delete a template |
| `GET` | `/api/templates/{id}` | auth | Get a template |
| `PUT` | `/api/templates/{id}` | auth | Update a template |
| `POST` | `/api/templates/{id}/run` | auth | Launch a run from a template. Optional launch-time overrides for any prompted field; vaults (credential ids) replaces the template's vault list. If the template defines allowed inventories (inventoryIds), an inventoryId outside that set (and the default) is rejected with 400. Terraform/OpenTofu templates may set `tfBackend` (an HTTP state-backend URL) — the runner inits with a `backend "http"` at that address. |

## Runs

| Method | Path | Access | Summary |
|--------|------|--------|---------|
| `DELETE` | `/api/runs` | auth | Clear finished runs (optionally ?projectId=…); active/queued runs are kept |
| `GET` | `/api/runs` | auth | List runs (scoped to active project) |
| `POST` | `/api/runs` | auth | Launch an ad-hoc run |
| `DELETE` | `/api/runs/{id}` | auth | Delete a single run from history |
| `GET` | `/api/runs/{id}` | auth | Get a run (+ active flag) |
| `POST` | `/api/runs/{id}/approve` | auth | Approve a run awaiting approval (project admin) → it dispatches |
| `GET` | `/api/runs/{id}/artifacts` | auth | Files captured from the run's working dir (per the template's artifact globs) |
| `GET` | `/api/runs/{id}/artifacts/{artifactId}` | auth | Download one captured artifact (attachment) |
| `POST` | `/api/runs/{id}/cancel` | auth | Cancel an active or queued run |
| `GET` | `/api/runs/{id}/log` | auth | Structured log: output + per-line completion timestamps (for the live log view) |
| `GET` | `/api/runs/{id}/output` | auth | Full captured terminal output (text) |
| `POST` | `/api/runs/{id}/reject` | auth | Reject a run awaiting approval (project admin) → canceled |
| `GET` | `/api/runs/{id}/secrets` | **admin** | Reveal the run's real secret extra-var values (admin only; the run/log otherwise show them masked). Audited. |

## Workflows

| Method | Path | Access | Summary |
|--------|------|--------|---------|
| `GET` | `/api/projects/{id}/workflows` | auth | List a project's workflows (sequential template pipelines) |
| `POST` | `/api/projects/{id}/workflows` | auth | Create a workflow. Steps run in order, gated by condition (on_success/on_failure/always) + an optional `when` guard (key \| !key \| key==v \| key!=v vs the workflow variables). `variables` are injected as extra-vars into every step; a step can export more via aui_output.json. A step's templateId may belong to ANOTHER project (cross-project workflow) — the caller must have run access there; the step runs in the template's project. |
| `GET` | `/api/workflow-runs` | auth | List workflow runs (optional ?projectId / ?workflowId) |
| `GET` | `/api/workflow-runs/{id}` | auth | Get a workflow run with its per-step status + run ids |
| `DELETE` | `/api/workflows/{id}` | auth | Delete a workflow |
| `GET` | `/api/workflows/{id}` | auth | Get a workflow |
| `PUT` | `/api/workflows/{id}` | auth | Update a workflow |
| `POST` | `/api/workflows/{id}/rollback/{versionId}` | auth | Roll the workflow back to a past version (the current definition is snapshotted first, so rollback is itself reversible) |
| `POST` | `/api/workflows/{id}/run` | auth | Run a workflow — launches the first runnable step; each step's outcome advances the pipeline. Optional `variables` override the workflow defaults for this run. |
| `GET` | `/api/workflows/{id}/versions` | auth | Version history — an immutable snapshot of the workflow definition is captured before each edit/rollback |

## Schedules

| Method | Path | Access | Summary |
|--------|------|--------|---------|
| `GET` | `/api/schedules` | auth | List schedules |
| `POST` | `/api/schedules` | auth | Create a schedule (cron or run-once) targeting a template OR a workflow |
| `DELETE` | `/api/schedules/{id}` | auth | Delete a schedule |
| `GET` | `/api/schedules/{id}` | auth | Get a schedule |
| `PUT` | `/api/schedules/{id}` | auth | Update a schedule |

## Repositories

| Method | Path | Access | Summary |
|--------|------|--------|---------|
| `GET` | `/api/repositories` | auth | List git repositories |
| `POST` | `/api/repositories` | auth | Add a repository. `cacheEnabled` (default true) keeps a warm mirror that the leader background-syncs ~every 10 min. |
| `DELETE` | `/api/repositories/{id}` | auth | Delete a repository |
| `GET` | `/api/repositories/{id}` | auth | Get a repository |
| `PUT` | `/api/repositories/{id}` | auth | Update a repository (incl. cacheEnabled) |
| `POST` | `/api/repositories/{id}/sync` | auth | Clone/pull now (manual). Cached repos also sync in the background. |
| `GET` | `/api/repositories/{id}/tree` | auth | Repo file tree |

## Key Store

| Method | Path | Access | Summary |
|--------|------|--------|---------|
| `GET` | `/api/credentials` | auth | List credentials (secrets never returned) |
| `POST` | `/api/credentials` | auth | Create a credential |
| `DELETE` | `/api/credentials/{id}` | auth | Delete a credential |
| `PUT` | `/api/credentials/{id}` | auth | Update a credential |
| `GET` | `/api/credentials/{id}/pubkey` | **admin** | Derive the SSH public key (authorized_keys format) from an SSH credential's private key |
| `GET` | `/api/secret-backends` | auth | List external secret managers (tokens never returned) |
| `POST` | `/api/secret-backends` | **admin** | Add a backend — type vault / aws / azure / gcp / cyberark. token holds the secret credential (vault token · aws secret key · azure client secret · gcp service-account JSON · cyberark Conjur API key). azure: address=vault URL, tenantId, accessKeyId=client id. gcp: accessKeyId=project id. cyberark: address=Conjur URL, namespace=account, accessKeyId=login (host/…); a referenced secret's path is the Conjur variable id. |
| `POST` | `/api/secret-backends/test` | **admin** | Test connectivity/auth to a backend (body = backend config; id+empty token uses the stored token) |
| `DELETE` | `/api/secret-backends/{id}` | **admin** | Delete a backend |
| `PUT` | `/api/secret-backends/{id}` | **admin** | Update a backend (empty token keeps the stored one) |

## Environments

| Method | Path | Access | Summary |
|--------|------|--------|---------|
| `GET` | `/api/environments` | auth | List environments (variable groups) |
| `POST` | `/api/environments` | auth | Create an environment |
| `DELETE` | `/api/environments/{id}` | auth | Delete an environment |
| `GET` | `/api/environments/{id}` | auth | Get an environment |
| `PUT` | `/api/environments/{id}` | auth | Update an environment |

## Hosts

| Method | Path | Access | Summary |
|--------|------|--------|---------|
| `GET` | `/api/hosts` | auth | Central host registry — every host with stored facts (+ summary: os/distro/ip/kernel + gatheredAt) |
| `DELETE` | `/api/hosts/{host}` | **admin** | Remove a host from the registry |
| `GET` | `/api/hosts/{host}` | auth | Full gathered facts for one host |
| `GET` | `/api/monitors` | auth | List host monitors (status up/down/unknown, consecutive failures, last-checked/last-up) |
| `POST` | `/api/monitors/check` | **admin** | Run a reachability check now (the leader also runs it ~every 2 min) |

## Notifications

| Method | Path | Access | Summary |
|--------|------|--------|---------|
| `GET` | `/api/notification-logs` | **admin** | Delivery audit log — one row per dispatch attempt (channel, run, event, ok/error). Optional ?limit (default 100, max 500). Trimmed by the audit-retention policy. |
| `GET` | `/api/notifications` | auth | List notification channels |
| `POST` | `/api/notifications` | auth | Create a channel — type telegram/slack/teams/discord/rocketchat/googlechat/gotify/ntfy/pushover/dingtalk/pagerduty/opsgenie/webhook/email. Optional `projectId` scopes it to one project (else all). Optional `template` customises the body with {{run}}/{{status}}/{{project}}/{{app}}/{{playbook}}/{{commit}}/{{version}}/{{actor}}/{{exitCode}} vars. |
| `DELETE` | `/api/notifications/{id}` | auth | Delete a channel |
| `PUT` | `/api/notifications/{id}` | auth | Update a channel |
| `POST` | `/api/notifications/{id}/test` | auth | Send a test notification |

## Integrations

| Method | Path | Access | Summary |
|--------|------|--------|---------|
| `GET` | `/api/integrations` | auth | List webhook integrations |
| `POST` | `/api/integrations` | auth | Create a webhook integration — targets a template (templateId) OR a workflow (workflowId) |
| `DELETE` | `/api/integrations/{id}` | auth | Delete a webhook |
| `PUT` | `/api/integrations/{id}` | auth | Update a webhook |
| `POST` | `/api/webhooks/{token}` | public | PUBLIC inbound webhook — fires the template OR workflow (authorised by the URL token + optional HMAC/Basic) |

## Runners

| Method | Path | Access | Summary |
|--------|------|--------|---------|
| `GET` | `/api/runners` | auth | List runners with derived online/offline status + tags |
| `POST` | `/api/runners/register` | public | Runner self-registration (auth: X-Runner-Token header, not a user session). Upserts by URL. maxConcurrent caps simultaneous jobs (0 = unlimited). |
| `DELETE` | `/api/runners/{id}` | **admin** | Remove a runner from the registry (not the built-in) |
| `POST` | `/api/runners/{id}/heartbeat` | public | Runner heartbeat (auth: X-Runner-Token header) |

## Applications

| Method | Path | Access | Summary |
|--------|------|--------|---------|
| `GET` | `/api/applications` | auth | List execution backends (built-ins + custom), with active state + base args |
| `POST` | `/api/applications` | **admin** | Register a custom application (arbitrary binary + base args) |
| `DELETE` | `/api/applications/{id}` | **admin** | Delete a custom application |
| `PUT` | `/api/applications/{id}` | **admin** | Update an application (toggle active, tweak base args) |

## Template Views

| Method | Path | Access | Summary |
|--------|------|--------|---------|
| `GET` | `/api/views` | auth | List saved views |
| `POST` | `/api/views` | auth | Create a view |
| `DELETE` | `/api/views/{id}` | auth | Delete a view |
| `PUT` | `/api/views/{id}` | auth | Update a view |

## Users & Tokens

| Method | Path | Access | Summary |
|--------|------|--------|---------|
| `GET` | `/api/tokens` | auth | List your API tokens |
| `POST` | `/api/tokens` | auth | Create a bearer token (shown once) |
| `DELETE` | `/api/tokens/{id}` | auth | Revoke a token |
| `GET` | `/api/users` | auth | List users |
| `POST` | `/api/users` | **admin** | Create a user |
| `DELETE` | `/api/users/{id}` | **admin** | Delete a user |

## WebSocket

| Method | Path | Access | Summary |
|--------|------|--------|---------|
| `GET` | `/ws/events` | auth | Live event bus (run/project/activity updates) |
| `GET` | `/ws/runs/{id}` | auth | Live PTY stream for a run (replay + late-join) |

## Examples


### `POST /api/auth/login`

Log in (sets the session cookie). If the account has 2FA on, returns {twoFactorRequired:true} (TOTP) or {emailOtpRequired:true} (email) — repeat with the `code`.

Request:

```json
{"username":"admin","password":"secret123","code":"123456"}
```

Response:

```json
{"id":"usr_…","username":"admin","role":"admin"}
```


### `POST /api/runs`

Launch an ad-hoc run

Request:

```json
{"projectId":"prj_…","app":"bash","playbook":"scripts/deploy.sh","extraVars":{},"timezone":"Europe/Moscow"}
```

Response:

```json
{"id":"run_…","status":"pending"}
```


### `GET /api/runs/{id}/output`

Full captured terminal output (text)


## Models

The API Explorer also documents **14 entity schemas** (the *Models* tab) - each field with its JSON name and type. Examples: `Project`, `Template`, `Inventory`, `Run`, `Workflow`, `WorkflowRun`, `Schedule`, `Credential`, `NotificationChannel`, `Runner`, `Environment`, `Repository` .... Browse them at `/api`.


---
*Generated from the running API (`/api/meta/endpoints`). Canonical source: `internal/api/apidoc.go`. Regenerate after adding endpoints.*
