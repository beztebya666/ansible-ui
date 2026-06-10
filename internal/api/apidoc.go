package api

import (
	"net/http"
)

// ParamDoc documents a path or query parameter.
type ParamDoc struct {
	Name string `json:"name"`
	In   string `json:"in"` // path | query
	Desc string `json:"desc"`
}

// EndpointDoc is one documented API route for the built-in API Explorer.
type EndpointDoc struct {
	Method   string     `json:"method"`
	Path     string     `json:"path"`
	Group    string     `json:"group"`
	Summary  string     `json:"summary"`
	Auth     bool       `json:"auth"`  // requires a session/token
	Admin    bool       `json:"admin"` // requires the admin role
	Params   []ParamDoc `json:"params,omitempty"`
	Request  string     `json:"request,omitempty"`  // example request body (JSON)
	Response string     `json:"response,omitempty"` // example response (JSON)
}

const docsSettingKey = "docs_enabled"

// effectiveDocsEnabled returns the runtime docs flag: a DB override (set from the
// UI) takes precedence over the env/Helm default (cfg.DocsEnabled).
func (s *Server) effectiveDocsEnabled(r *http.Request) bool {
	if v, ok, _ := s.store.GetSetting(r.Context(), docsSettingKey); ok {
		return v == "true"
	}
	return s.cfg.DocsEnabled
}

// handleAPIDocs serves the endpoint catalog (the data behind the API Explorer).
func (s *Server) handleAPIDocs(w http.ResponseWriter, r *http.Request) {
	if !s.effectiveDocsEnabled(r) {
		writeErr(w, http.StatusForbidden, "API Explorer is disabled")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"enabled":   true,
		"baseUrl":   "/api",
		"authNote":  "🔒 endpoints accept a session cookie or `Authorization: Bearer <api-token>`.",
		"endpoints": apiCatalog,
		"models":    s.apiModels(),
	})
}

// handleSetDocs toggles the API Explorer at runtime (admin only); overrides env.
func (s *Server) handleSetDocs(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var in struct {
		Enabled bool `json:"enabled"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	val := "false"
	if in.Enabled {
		val = "true"
	}
	if err := s.store.SetSetting(r.Context(), docsSettingKey, val); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"docsEnabled": in.Enabled})
}

// apiCatalog is the hand-curated description of every route. Auth flags mirror
// authGuard (isPublicPath): everything is 🔒 except health, auth status/login/
// setup, OIDC, and inbound webhooks.
var apiCatalog = []EndpointDoc{
	// ---- Meta / health ----
	{Method: "GET", Path: "/api/health", Group: "Meta", Summary: "Liveness probe", Response: `{"status":"ok","service":"api"}`},
	{Method: "GET", Path: "/metrics", Group: "Meta", Summary: "Prometheus metrics (runs by status, active/queued/awaiting, runners + load). Public unless METRICS_TOKEN is set (then Bearer)."},
	{Method: "POST", Path: "/api/mcp", Group: "Meta", Auth: true, Summary: "Model Context Protocol (MCP) server for AI agents — JSON-RPC 2.0 (initialize / tools/list / tools/call). Tools: list_projects, list_templates, run_template, get_run, list_runs. Auth via session or Bearer API token; tools act as that user (project capabilities enforced).", Request: `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"run_template","arguments":{"templateId":"tpl_…"}}}`},
	{Method: "GET", Path: "/api/insights", Group: "Meta", Auth: true, Summary: "Run analytics over a recent window: per-day counts by outcome, success rate, avg duration + dispatch wait.",
		Params:   []ParamDoc{{Name: "days", In: "query", Desc: "window in days (default 14, max 90)"}, {Name: "projectId", In: "query", Desc: "scope to one project"}},
		Response: `{"days":14,"perDay":[{"date":"2026-06-08","success":3,"failed":1,"other":0,"total":4}],"total":12,"successRate":91.6,"avgDurationSec":7.2,"avgWaitSec":0.4,"byTemplate":[{"templateName":"Deploy","runs":8,"successRate":100,"avgDurationSec":6.1}]}`},
	{Method: "GET", Path: "/api/stats", Group: "Meta", Auth: true, Summary: "Dashboard counters + recent runs",
		Response: `{"projects":2,"templates":14,"runsTotal":42,"runsByState":{"success":40,"failed":2},"active":0,"recentRuns":[]}`},
	{Method: "GET", Path: "/api/meta/endpoints", Group: "Meta", Auth: true, Summary: "This API catalog (powers the Explorer)"},
	{Method: "PUT", Path: "/api/meta/docs", Group: "Meta", Auth: true, Admin: true, Summary: "Enable/disable the API Explorer at runtime",
		Request: `{"enabled":true}`, Response: `{"docsEnabled":true}`},
	{Method: "GET", Path: "/api/activity", Group: "Meta", Auth: true, Summary: "Activity / audit feed",
		Params:   []ParamDoc{{Name: "limit", In: "query", Desc: "max entries (default 100)"}},
		Response: `[{"id":"act_…","action":"run.success","target":"Deploy prod","detail":"ansible","createdAt":"…"}]`},
	{Method: "GET", Path: "/api/audit/export", Group: "Meta", Auth: true, Admin: true, Summary: "Download the full audit log as CSV (compliance)"},
	{Method: "GET", Path: "/api/runs/export", Group: "Meta", Auth: true, Admin: true, Summary: "Download run history as CSV (optional ?projectId)"},
	{Method: "GET", Path: "/api/compliance/report", Group: "Meta", Auth: true, Admin: true, Summary: "Consolidated compliance report (JSON) over ?from..?to (RFC3339; default last 30d): access-control matrix (users/roles/MFA), activity-by-action, runs-by-status, the full secret-access trail, and notification-delivery counts.", Response: `{"generatedAt":"…","from":"…","to":"…","accessControl":[{"username":"admin","role":"admin","twoFactor":false}],"summary":{"users":1,"admins":1,"runs":3},"secretAccess":[]}`},
	{Method: "GET", Path: "/api/settings/retention", Group: "Meta", Auth: true, Admin: true, Summary: "Retention policy in days (0 = keep forever)", Response: `{"runsDays":90,"auditDays":365}`},
	{Method: "PUT", Path: "/api/settings/retention", Group: "Meta", Auth: true, Admin: true, Summary: "Set retention; a sweep runs hourly + on save", Request: `{"runsDays":90,"auditDays":365}`},
	{Method: "GET", Path: "/api/settings/flags", Group: "Meta", Auth: true, Admin: true, Summary: "Feature flags", Response: `{"nonadminCreateProject":false}`},
	{Method: "PUT", Path: "/api/settings/flags", Group: "Meta", Auth: true, Admin: true, Summary: "Set feature flags (e.g. allow non-admins to create projects)", Request: `{"nonadminCreateProject":true}`},
	{Method: "GET", Path: "/api/settings/tasks", Group: "Meta", Auth: true, Admin: true, Summary: "Task limits", Response: `{"maxDurationSec":0,"longRunAlertSec":0,"maxParallel":0}`},
	{Method: "PUT", Path: "/api/settings/tasks", Group: "Meta", Auth: true, Admin: true, Summary: "Set task limits — maxDurationSec auto-cancels an overrunning run; longRunAlertSec notifies long-running-subscribed channels; maxParallel is a global cap on concurrent runs (new launches → 409 when reached). 0 = off/unlimited.", Request: `{"maxDurationSec":3600,"longRunAlertSec":600,"maxParallel":10}`},
	{Method: "GET", Path: "/api/settings/syslog", Group: "Meta", Auth: true, Admin: true, Summary: "Syslog / log-export config", Response: `{"enabled":false,"address":"","protocol":"udp","tag":"ansible-ui"}`},
	{Method: "PUT", Path: "/api/settings/syslog", Group: "Meta", Auth: true, Admin: true, Summary: "Log export — forward each finished run to a syslog collector (RFC 5424) and/or an HTTP event collector (Splunk HEC: Authorization: Splunk <token>)", Request: `{"enabled":true,"address":"logs:514","protocol":"udp","tag":"ansible-ui","httpEnabled":true,"httpUrl":"https://splunk:8088/services/collector","httpToken":"<hec>","httpFormat":"splunk"}`},
	{Method: "GET", Path: "/api/settings/smtp", Group: "Meta", Auth: true, Admin: true, Summary: "Global SMTP config (email-OTP login codes + default for email notifications)", Response: `{"host":"smtp.example.com","port":"587","from":"ansible-ui@x"}`},
	{Method: "PUT", Path: "/api/settings/smtp", Group: "Meta", Auth: true, Admin: true, Summary: "Set global SMTP", Request: `{"host":"smtp.example.com","port":"587","from":"ansible-ui@x","username":"","password":""}`},
	{Method: "GET", Path: "/api/settings/galaxy", Group: "Meta", Auth: true, Admin: true, Summary: "Private Ansible Galaxy / Automation Hub auth (injected as ANSIBLE_GALAXY_SERVER_* for ansible-galaxy install)", Response: `{"serverUrl":"https://hub.example.com/api/galaxy/","token":"","cliArgs":""}`},
	{Method: "PUT", Path: "/api/settings/galaxy", Group: "Meta", Auth: true, Admin: true, Summary: "Set private galaxy server URL + token + extra ansible-galaxy CLI args", Request: `{"serverUrl":"https://hub.example.com/api/galaxy/","token":"<api-token>","cliArgs":"--ignore-certs"}`},
	{Method: "GET", Path: "/api/settings/notify-proxy", Group: "Meta", Auth: true, Admin: true, Summary: "Outbound proxy for notification egress (alert-proxy)", Response: `{"proxyUrl":""}`},
	{Method: "PUT", Path: "/api/settings/notify-proxy", Group: "Meta", Auth: true, Admin: true, Summary: "Route chat/webhook notifications through an http/https/socks5 proxy (empty = direct)", Request: `{"proxyUrl":"http://proxy.corp:3128"}`},
	{Method: "POST", Path: "/api/auth/2fa/email/enable", Group: "Auth", Auth: true, Summary: "Enable email-OTP login for the current user (needs an email + configured SMTP)"},
	{Method: "POST", Path: "/api/auth/2fa/email/disable", Group: "Auth", Auth: true, Summary: "Disable email-OTP login for the current user"},
	{Method: "GET", Path: "/api/settings/auth-mapping", Group: "Meta", Auth: true, Admin: true, Summary: "LDAP/OIDC group → role mapping rules + the LDAP group attr / OIDC groups claim + ldapDebug flag", Response: `{"rules":[{"group":"admins","role":"admin"}],"ldapGroupAttr":"memberOf","oidcGroupsClaim":"groups","ldapDebug":false}`},
	{Method: "PUT", Path: "/api/settings/auth-mapping", Group: "Meta", Auth: true, Admin: true, Summary: "Set group→role mapping (applied on each LDAP/OIDC login; admin wins, else default role) + toggle ldapDebug (verbose LDAP login logging). Note: local accounts are always tried first, so a directory outage never locks them out."},
	{Method: "GET", Path: "/api/system", Group: "Meta", Auth: true, Admin: true, Summary: "System information — version, runtime, the runner's `ansible --version`, enabled auth (incl. RADIUS) + notification capabilities, task limits, feature flags, runner fleet"},
	{Method: "GET", Path: "/api/cluster", Group: "Meta", Auth: true, Admin: true, Summary: "Cluster/HA dashboard — leader flag, per-runner capacity (active/max), and the live queued + running tasks"},

	// ---- Auth ----
	{Method: "GET", Path: "/api/auth/status", Group: "Auth", Summary: "Setup/login/app state + enabled providers",
		Response: `{"needsSetup":false,"authenticated":true,"user":{"id":"usr_…","username":"admin","role":"admin"},"providers":{"local":true,"ldap":false,"oidc":false},"docsEnabled":true}`},
	{Method: "POST", Path: "/api/auth/setup", Group: "Auth", Summary: "Create the first admin (first run only)",
		Request: `{"username":"admin","email":"admin@local","password":"secret123"}`, Response: `{"id":"usr_…","username":"admin","role":"admin"}`},
	{Method: "POST", Path: "/api/auth/login", Group: "Auth", Summary: "Log in (sets the session cookie). If the account has 2FA on, returns {twoFactorRequired:true} (TOTP) or {emailOtpRequired:true} (email) — repeat with the `code`.",
		Request: `{"username":"admin","password":"secret123","code":"123456"}`, Response: `{"id":"usr_…","username":"admin","role":"admin"}`},
	{Method: "POST", Path: "/api/auth/logout", Group: "Auth", Auth: true, Summary: "Log out (clears the session)", Response: `{"ok":true}`},
	{Method: "GET", Path: "/api/auth/me", Group: "Auth", Auth: true, Summary: "The current user"},
	{Method: "POST", Path: "/api/auth/2fa/setup", Group: "Auth", Auth: true, Summary: "Begin TOTP enrolment — returns a base32 secret + otpauth:// URI (not yet enabled)", Response: `{"secret":"JBSWY3DPEHPK3PXP","otpauthUrl":"otpauth://totp/…"}`},
	{Method: "POST", Path: "/api/auth/2fa/enable", Group: "Auth", Auth: true, Summary: "Verify a code against the pending secret and turn 2FA on", Request: `{"code":"123456"}`},
	{Method: "POST", Path: "/api/auth/2fa/disable", Group: "Auth", Auth: true, Summary: "Turn 2FA off (requires a current code)", Request: `{"code":"123456"}`},
	{Method: "POST", Path: "/api/users/{id}/2fa/reset", Group: "Auth", Auth: true, Admin: true, Summary: "Admin: clear a user's 2FA (account recovery)"},
	{Method: "GET", Path: "/api/auth/oidc/login", Group: "Auth", Summary: "Begin OIDC SSO with PKCE (redirect to the IdP)"},
	{Method: "GET", Path: "/api/auth/oidc/callback", Group: "Auth", Summary: "OIDC callback (PKCE verify, required-claim restriction, account-linking by email, sign in)"},
	{Method: "GET", Path: "/api/auth/oidc/logout", Group: "Auth", Summary: "Sign out + RP-initiated IdP logout (redirects to the provider's end_session_endpoint if discovered)"},
	{Method: "GET", Path: "/api/auth/saml/metadata", Group: "Auth", Summary: "SAML 2.0 SP metadata XML (register this at the IdP)"},
	{Method: "GET", Path: "/api/auth/saml/login", Group: "Auth", Summary: "Begin SAML SSO (SP-initiated AuthnRequest, redirect to the IdP)"},
	{Method: "POST", Path: "/api/auth/saml/acs", Group: "Auth", Summary: "SAML Assertion Consumer Service — the IdP POSTs the signed SAMLResponse here; links by email + signs in"},
	{Method: "GET", Path: "/api/auth/github/login", Group: "Auth", Summary: "Begin GitHub OAuth sign-in (redirect)"},
	{Method: "GET", Path: "/api/auth/github/callback", Group: "Auth", Summary: "GitHub OAuth callback (redirect)"},
	{Method: "GET", Path: "/api/auth/bitbucket/login", Group: "Auth", Summary: "Begin Bitbucket OAuth sign-in (redirect)"},
	{Method: "GET", Path: "/api/auth/bitbucket/callback", Group: "Auth", Summary: "Bitbucket OAuth callback (redirect)"},

	// ---- Users & tokens ----
	{Method: "GET", Path: "/api/users", Group: "Users & Tokens", Auth: true, Summary: "List users"},
	{Method: "POST", Path: "/api/users", Group: "Users & Tokens", Auth: true, Admin: true, Summary: "Create a user",
		Request: `{"username":"deploy","email":"d@x.io","password":"secret123","role":"user"}`},
	{Method: "DELETE", Path: "/api/users/{id}", Group: "Users & Tokens", Auth: true, Admin: true, Summary: "Delete a user",
		Params: []ParamDoc{{Name: "id", In: "path", Desc: "user id"}}},
	{Method: "GET", Path: "/api/tokens", Group: "Users & Tokens", Auth: true, Summary: "List your API tokens"},
	{Method: "POST", Path: "/api/tokens", Group: "Users & Tokens", Auth: true, Summary: "Create a bearer token (shown once)",
		Request: `{"name":"ci-pipeline","expiresDays":90}`, Response: `{"id":"tok_…","token":"aui_…","prefix":"aui_…"}`},
	{Method: "DELETE", Path: "/api/tokens/{id}", Group: "Users & Tokens", Auth: true, Summary: "Revoke a token",
		Params: []ParamDoc{{Name: "id", In: "path", Desc: "token id"}}},

	// ---- Projects ----
	{Method: "GET", Path: "/api/projects", Group: "Projects", Auth: true, Summary: "List projects (tenants)"},
	{Method: "POST", Path: "/api/projects", Group: "Projects", Auth: true, Summary: "Create a project",
		Request: `{"name":"Production","description":"prod tenant","repositoryId":"","subPath":""}`},
	{Method: "GET", Path: "/api/projects/{id}", Group: "Projects", Auth: true, Summary: "Get a project (+ detected playbooks & inventory files)",
		Params: []ParamDoc{{Name: "id", In: "path", Desc: "project id"}}},
	{Method: "PUT", Path: "/api/projects/{id}", Group: "Projects", Auth: true, Summary: "Update a project (name / description / slug) — project admin",
		Request: `{"name":"Production","description":"prod tenant"}`},
	{Method: "DELETE", Path: "/api/projects/{id}", Group: "Projects", Auth: true, Summary: "Delete a project (project admin)",
		Params: []ParamDoc{{Name: "id", In: "path", Desc: "project id"}}},
	{Method: "GET", Path: "/api/projects/{id}/members", Group: "Projects", Auth: true, Summary: "List project members (per-project RBAC)"},
	{Method: "POST", Path: "/api/projects/{id}/members", Group: "Projects", Auth: true,
		Summary: "Add/update a member's role (project admin); role = viewer|editor|admin OR a custom-role id",
		Request: `{"userId":"usr_…","role":"editor"}`},
	{Method: "DELETE", Path: "/api/projects/{id}/members/{userId}", Group: "Projects", Auth: true, Summary: "Remove a project member (project admin)"},
	{Method: "GET", Path: "/api/roles", Group: "Projects", Auth: true, Summary: "List custom roles (granular capability sets: view/run/edit/manage)"},
	{Method: "POST", Path: "/api/roles", Group: "Projects", Auth: true, Admin: true, Summary: "Create a custom role (admin). Permissions are any of view/run/edit/manage.", Request: `{"name":"operator","permissions":["view","run"]}`},
	{Method: "PUT", Path: "/api/roles/{id}", Group: "Projects", Auth: true, Admin: true, Summary: "Update a custom role (admin)"},
	{Method: "DELETE", Path: "/api/roles/{id}", Group: "Projects", Auth: true, Admin: true, Summary: "Delete a custom role (admin)"},
	{Method: "GET", Path: "/api/projects/{id}/playbooks", Group: "Projects", Auth: true, Summary: "List detected playbooks"},
	{Method: "GET", Path: "/api/projects/{id}/requirements", Group: "Projects", Auth: true, Summary: "Detected ansible-galaxy requirements (roles/collections from requirements.yml)"},
	{Method: "GET", Path: "/api/projects/{id}/files", Group: "Projects", Auth: true, Summary: "Project file tree"},
	{Method: "GET", Path: "/api/projects/{id}/file", Group: "Projects", Auth: true, Summary: "Read a file",
		Params: []ParamDoc{{Name: "path", In: "query", Desc: "file path relative to project"}}},
	{Method: "PUT", Path: "/api/projects/{id}/file", Group: "Projects", Auth: true, Summary: "Write a file",
		Request: `{"path":"playbooks/site.yml","content":"- hosts: all\n  tasks: []"}`},
	{Method: "DELETE", Path: "/api/projects/{id}/file", Group: "Projects", Auth: true, Summary: "Delete a file or folder",
		Params: []ParamDoc{{Name: "path", In: "query", Desc: "file/dir path relative to project"}}},
	{Method: "POST", Path: "/api/projects/{id}/git-commit", Group: "Projects", Auth: true,
		Summary:  "Git-backed projects: commit one or more edited files onto a NEW branch and push it (never the protected base branch). Returns the branch + a PR/MR compare URL.",
		Request:  `{"files":[{"path":"playbooks/site.yml","content":"---\n…"}],"message":"tweak site.yml","branch":"aui/edit-site"}`,
		Response: `{"branch":"aui/edit-site","commit":"abc123","pushed":true,"prUrl":"https://github.com/o/r/compare/main...aui/edit-site?expand=1"}`},
	{Method: "GET", Path: "/api/projects/{id}/branches", Group: "Projects", Auth: true, Summary: "Branches the UI committed + pushed for this project (with PR/MR links)"},
	{Method: "POST", Path: "/api/projects/{id}/rename", Group: "Projects", Auth: true, Summary: "Rename/move a file or folder",
		Request: `{"from":"playbooks/old.yml","to":"playbooks/new.yml"}`},
	{Method: "POST", Path: "/api/projects/{id}/upload", Group: "Projects", Auth: true,
		Summary:  "Upload a .tar.gz/.zip archive into a local project (multipart 'file'; optional 'password' for encrypted zips)",
		Response: `{"extracted":12}  // or {"error":"…","encrypted":true} when a password is needed`},
	{Method: "GET", Path: "/api/projects/{id}/inventories", Group: "Projects", Auth: true, Summary: "List inventories"},
	{Method: "POST", Path: "/api/projects/{id}/inventories", Group: "Projects", Auth: true, Summary: "Create an inventory — type static/file/dynamic/url/cloud. For url, content is an HTTP(S) URL fetched at launch. runnerTag pins runs to a matching runner (affinity); connCredentialIds are Key Store SSH creds ansible connects with.",
		Request: `{"name":"prod","type":"static","content":"[web]\nweb01","runnerTag":"edge-dc1","connCredentialIds":["cred_..."]}`},
	{Method: "GET", Path: "/api/projects/{id}/export", Group: "Projects", Auth: true, Summary: "Export a project's structure (inventories/templates/workflows/schedules) as a portable JSON bundle — refs by name; secrets excluded"},
	{Method: "POST", Path: "/api/projects/import", Group: "Projects", Auth: true, Admin: true, Summary: "Import a project bundle into a new project (remaps name refs to fresh ids; credential/repo links must be re-added)"},
	{Method: "GET", Path: "/api/projects/{id}/templates", Group: "Projects", Auth: true, Summary: "Templates in a project"},

	// ---- Inventories ----
	{Method: "GET", Path: "/api/inventories/{id}", Group: "Inventories", Auth: true, Summary: "Get an inventory"},
	{Method: "GET", Path: "/api/inventories/{id}/hosts", Group: "Inventories", Auth: true, Summary: "Resolve the inventory's groups + hosts via ansible-inventory (the runner). Works for static/file/dynamic; cloud/url resolve at run time only.", Response: `{"groups":[{"name":"web","hosts":["web1","web2"]}],"hosts":["web1","web2"],"total":2}`},
	{Method: "POST", Path: "/api/inventories/{id}/facts/gather", Group: "Inventories", Auth: true, Summary: "Gather Ansible facts for the inventory's hosts (runner `ansible -m setup`) and store them in the central host registry. Needs the project `run` capability. Optional ?pattern (default all).", Response: `{"gathered":["localhost"],"count":1}`},
	{Method: "POST", Path: "/api/inventories/{id}/monitor", Group: "Inventories", Auth: true, Summary: "Enable reachability monitoring for the inventory's hosts (needs `edit`); the leader periodically pings them and alerts subscribed channels on up→down. Optional ?threshold (consecutive failures before DOWN, default 3).", Response: `{"monitored":4}`},
	{Method: "DELETE", Path: "/api/inventories/{id}/monitor", Group: "Inventories", Auth: true, Summary: "Disable monitoring for the inventory's hosts"},
	{Method: "GET", Path: "/api/monitors", Group: "Hosts", Auth: true, Summary: "List host monitors (status up/down/unknown, consecutive failures, last-checked/last-up)"},
	{Method: "POST", Path: "/api/monitors/check", Group: "Hosts", Auth: true, Admin: true, Summary: "Run a reachability check now (the leader also runs it ~every 2 min)"},
	{Method: "GET", Path: "/api/hosts", Group: "Hosts", Auth: true, Summary: "Central host registry — every host with stored facts (+ summary: os/distro/ip/kernel + gatheredAt)"},
	{Method: "GET", Path: "/api/hosts/{host}", Group: "Hosts", Auth: true, Summary: "Full gathered facts for one host"},
	{Method: "DELETE", Path: "/api/hosts/{host}", Group: "Hosts", Auth: true, Admin: true, Summary: "Remove a host from the registry"},
	{Method: "PUT", Path: "/api/inventories/{id}", Group: "Inventories", Auth: true, Summary: "Update an inventory",
		Request: `{"name":"prod","type":"file","content":"inventories/prod.ini"}`},
	{Method: "DELETE", Path: "/api/inventories/{id}", Group: "Inventories", Auth: true, Summary: "Delete an inventory"},

	// ---- Templates ----
	{Method: "GET", Path: "/api/templates", Group: "Templates", Auth: true, Summary: "List templates (scoped to active project)",
		Params: []ParamDoc{{Name: "projectId", In: "query", Desc: "filter by project (else X-Project-Id header)"}}},
	{Method: "GET", Path: "/api/templates/all", Group: "Templates", Auth: true, Summary: "List templates across EVERY project the caller can run in (each hydrated with projectName) — powers the cross-project workflow-step picker", Response: `[{"id":"tpl_…","name":"Deploy","projectId":"prj_…","projectName":"Infra"}]`},
	{Method: "POST", Path: "/api/templates", Group: "Templates", Auth: true,
		Summary: "Create a template (any app). type: task|build|deploy — deploy ships the latest successful run of buildTemplateId, injecting build_version/build_commit/build_run_id as extra-vars.",
		Request: `{"projectId":"prj_…","name":"Deploy","app":"ansible","playbook":"playbooks/site.yml","inventoryId":null,"diff":true,"type":"deploy","buildTemplateId":"tpl_…"}`},
	{Method: "GET", Path: "/api/templates/{id}", Group: "Templates", Auth: true, Summary: "Get a template"},
	{Method: "PUT", Path: "/api/templates/{id}", Group: "Templates", Auth: true, Summary: "Update a template"},
	{Method: "DELETE", Path: "/api/templates/{id}", Group: "Templates", Auth: true, Summary: "Delete a template"},
	{Method: "POST", Path: "/api/templates/{id}/run", Group: "Templates", Auth: true,
		Summary: "Launch a run from a template. Optional launch-time overrides for any prompted field; vaults (credential ids) replaces the template's vault list. If the template defines allowed inventories (inventoryIds), an inventoryId outside that set (and the default) is rejected with 400. Terraform/OpenTofu templates may set `tfBackend` (an HTTP state-backend URL) — the runner inits with a `backend \"http\"` at that address.",
		Request: `{"extraVars":{"app_version":"2.0.0"},"vaults":["cred_…"],"timezone":"Europe/Moscow"}`, Response: `{"id":"run_…","status":"pending"}`},

	// ---- Workflows ----
	{Method: "GET", Path: "/api/projects/{id}/workflows", Group: "Workflows", Auth: true, Summary: "List a project's workflows (sequential template pipelines)"},
	{Method: "POST", Path: "/api/projects/{id}/workflows", Group: "Workflows", Auth: true, Summary: "Create a workflow. Steps run in order, gated by condition (on_success/on_failure/always) + an optional `when` guard (key | !key | key==v | key!=v vs the workflow variables). `variables` are injected as extra-vars into every step; a step can export more via aui_output.json. A step's templateId may belong to ANOTHER project (cross-project workflow) — the caller must have run access there; the step runs in the template's project.",
		Request: `{"name":"build→deploy","variables":{"env":"prod"},"steps":[{"name":"build","templateId":"tpl_b","condition":"on_success"},{"name":"deploy","templateId":"tpl_d","condition":"on_success","when":"env == prod","inventoryId":"inv_prod","environmentId":"env_prod","vaultCredentialId":"cred_vault","parallel":true}]}`},
	{Method: "GET", Path: "/api/workflows/{id}", Group: "Workflows", Auth: true, Summary: "Get a workflow"},
	{Method: "PUT", Path: "/api/workflows/{id}", Group: "Workflows", Auth: true, Summary: "Update a workflow"},
	{Method: "DELETE", Path: "/api/workflows/{id}", Group: "Workflows", Auth: true, Summary: "Delete a workflow"},
	{Method: "POST", Path: "/api/workflows/{id}/run", Group: "Workflows", Auth: true, Summary: "Run a workflow — launches the first runnable step; each step's outcome advances the pipeline. Optional `variables` override the workflow defaults for this run.",
		Request: `{"variables":{"version":"2.0.0"}}`, Response: `{"id":"wfr_…","status":"running","variables":{"version":"2.0.0"},"steps":[{"name":"build","status":"running"}]}`},
	{Method: "GET", Path: "/api/workflows/{id}/versions", Group: "Workflows", Auth: true, Summary: "Version history — an immutable snapshot of the workflow definition is captured before each edit/rollback", Response: `[{"id":"wfv_…","version":2,"name":"deploy","steps":[],"actor":"admin","createdAt":"…"}]`},
	{Method: "POST", Path: "/api/workflows/{id}/rollback/{versionId}", Group: "Workflows", Auth: true, Summary: "Roll the workflow back to a past version (the current definition is snapshotted first, so rollback is itself reversible)"},
	{Method: "GET", Path: "/api/workflow-runs", Group: "Workflows", Auth: true, Summary: "List workflow runs (optional ?projectId / ?workflowId)"},
	{Method: "GET", Path: "/api/workflow-runs/{id}", Group: "Workflows", Auth: true, Summary: "Get a workflow run with its per-step status + run ids"},

	// ---- Template Views ----
	{Method: "GET", Path: "/api/views", Group: "Template Views", Auth: true, Summary: "List saved views"},
	{Method: "POST", Path: "/api/views", Group: "Template Views", Auth: true, Summary: "Create a view",
		Request: `{"name":"Terraform","app":"terraform","search":""}`},
	{Method: "PUT", Path: "/api/views/{id}", Group: "Template Views", Auth: true, Summary: "Update a view"},
	{Method: "DELETE", Path: "/api/views/{id}", Group: "Template Views", Auth: true, Summary: "Delete a view"},

	// ---- Runs ----
	{Method: "GET", Path: "/api/runs", Group: "Runs", Auth: true, Summary: "List runs (scoped to active project)",
		Params: []ParamDoc{{Name: "projectId", In: "query", Desc: "filter by project"}, {Name: "status", In: "query", Desc: "running|success|failed|canceled"}, {Name: "limit", In: "query", Desc: "max rows"}}},
	{Method: "POST", Path: "/api/runs", Group: "Runs", Auth: true, Summary: "Launch an ad-hoc run",
		Request:  `{"projectId":"prj_…","app":"bash","playbook":"scripts/deploy.sh","extraVars":{},"timezone":"Europe/Moscow"}`,
		Response: `{"id":"run_…","status":"pending"}`},
	{Method: "GET", Path: "/api/runs/{id}", Group: "Runs", Auth: true, Summary: "Get a run (+ active flag)"},
	{Method: "GET", Path: "/api/runs/{id}/output", Group: "Runs", Auth: true, Summary: "Full captured terminal output (text)"},
	{Method: "GET", Path: "/api/runs/{id}/log", Group: "Runs", Auth: true, Summary: "Structured log: output + per-line completion timestamps (for the live log view)"},
	{Method: "GET", Path: "/api/runs/{id}/artifacts", Group: "Runs", Auth: true, Summary: "Files captured from the run's working dir (per the template's artifact globs)",
		Response: `[{"id":"art_…","runId":"run_…","name":"reports/site.html","size":2048,"createdAt":"…"}]`},
	{Method: "GET", Path: "/api/runs/{id}/artifacts/{artifactId}", Group: "Runs", Auth: true, Summary: "Download one captured artifact (attachment)"},
	{Method: "GET", Path: "/api/runs/{id}/secrets", Group: "Runs", Auth: true, Admin: true,
		Summary:  "Reveal the run's real secret extra-var values (admin only; the run/log otherwise show them masked). Audited.",
		Response: `{"db_password":"s3cr3t"}`},
	{Method: "POST", Path: "/api/runs/{id}/approve", Group: "Runs", Auth: true, Summary: "Approve a run awaiting approval (project admin) → it dispatches", Response: `{"status":"approved"}`},
	{Method: "POST", Path: "/api/runs/{id}/reject", Group: "Runs", Auth: true, Summary: "Reject a run awaiting approval (project admin) → canceled", Response: `{"status":"rejected"}`},
	{Method: "POST", Path: "/api/runs/{id}/cancel", Group: "Runs", Auth: true, Summary: "Cancel an active or queued run", Response: `{"status":"canceling"}`},
	{Method: "DELETE", Path: "/api/runs/{id}", Group: "Runs", Auth: true, Summary: "Delete a single run from history"},
	{Method: "DELETE", Path: "/api/runs", Group: "Runs", Auth: true, Summary: "Clear finished runs (optionally ?projectId=…); active/queued runs are kept",
		Params: []ParamDoc{{Name: "projectId", In: "query", Desc: "limit to one project (else all)"}}},

	// ---- Key Store ----
	{Method: "GET", Path: "/api/credentials", Group: "Key Store", Auth: true, Summary: "List credentials (secrets never returned)"},
	{Method: "POST", Path: "/api/credentials", Group: "Key Store", Auth: true, Summary: "Create a credential",
		Request: `{"name":"prod-ssh","type":"ssh","login":"deploy","sshPrivateKey":"-----BEGIN…","sshCertificate":"ssh-rsa-cert-v01@openssh.com AAAA…","personal":false}`},
	{Method: "PUT", Path: "/api/credentials/{id}", Group: "Key Store", Auth: true, Summary: "Update a credential"},
	{Method: "DELETE", Path: "/api/credentials/{id}", Group: "Key Store", Auth: true, Summary: "Delete a credential"},
	{Method: "GET", Path: "/api/credentials/{id}/pubkey", Group: "Key Store", Auth: true, Admin: true, Summary: "Derive the SSH public key (authorized_keys format) from an SSH credential's private key", Response: `{"publicKey":"ssh-ed25519 AAAA… ansible-ui-prod-ssh","type":"ssh-ed25519"}`},

	// ---- Secret Backends (external managers) ----
	{Method: "GET", Path: "/api/secret-backends", Group: "Key Store", Auth: true, Summary: "List external secret managers (tokens never returned)"},
	{Method: "POST", Path: "/api/secret-backends", Group: "Key Store", Admin: true,
		Summary: "Add a backend — type vault / aws / azure / gcp / cyberark. token holds the secret credential (vault token · aws secret key · azure client secret · gcp service-account JSON · cyberark Conjur API key). azure: address=vault URL, tenantId, accessKeyId=client id. gcp: accessKeyId=project id. cyberark: address=Conjur URL, namespace=account, accessKeyId=login (host/…); a referenced secret's path is the Conjur variable id.",
		Request: `{"name":"aws-prod","type":"aws","region":"eu-central-1","accessKeyId":"AKIA…","token":"wJalr…"}`},
	{Method: "POST", Path: "/api/secret-backends/test", Group: "Key Store", Admin: true,
		Summary: "Test connectivity/auth to a backend (body = backend config; id+empty token uses the stored token)"},
	{Method: "PUT", Path: "/api/secret-backends/{id}", Group: "Key Store", Admin: true, Summary: "Update a backend (empty token keeps the stored one)"},
	{Method: "DELETE", Path: "/api/secret-backends/{id}", Group: "Key Store", Admin: true, Summary: "Delete a backend"},

	// ---- Runners (distributed execution) ----
	{Method: "GET", Path: "/api/runners", Group: "Runners", Auth: true, Summary: "List runners with derived online/offline status + tags"},
	{Method: "POST", Path: "/api/runners/register", Group: "Runners",
		Summary: "Runner self-registration (auth: X-Runner-Token header, not a user session). Upserts by URL. maxConcurrent caps simultaneous jobs (0 = unlimited).",
		Request: `{"name":"edge-1","url":"ws://edge-1:8081","tags":["edge","gpu"],"platform":"linux/amd64","version":"1.0","maxConcurrent":2}`},
	{Method: "POST", Path: "/api/runners/{id}/heartbeat", Group: "Runners", Summary: "Runner heartbeat (auth: X-Runner-Token header)"},
	{Method: "DELETE", Path: "/api/runners/{id}", Group: "Runners", Admin: true, Summary: "Remove a runner from the registry (not the built-in)"},

	// ---- Repositories ----
	{Method: "GET", Path: "/api/repositories", Group: "Repositories", Auth: true, Summary: "List git repositories"},
	{Method: "POST", Path: "/api/repositories", Group: "Repositories", Auth: true, Summary: "Add a repository. `cacheEnabled` (default true) keeps a warm mirror that the leader background-syncs ~every 10 min.",
		Request: `{"name":"infra","gitUrl":"https://github.com/org/infra.git","branch":"main","credentialId":null,"cacheEnabled":true}`},
	{Method: "GET", Path: "/api/repositories/{id}", Group: "Repositories", Auth: true, Summary: "Get a repository"},
	{Method: "PUT", Path: "/api/repositories/{id}", Group: "Repositories", Auth: true, Summary: "Update a repository (incl. cacheEnabled)"},
	{Method: "DELETE", Path: "/api/repositories/{id}", Group: "Repositories", Auth: true, Summary: "Delete a repository"},
	{Method: "POST", Path: "/api/repositories/{id}/sync", Group: "Repositories", Auth: true, Summary: "Clone/pull now (manual). Cached repos also sync in the background.", Response: `{"repository":{},"commit":"abc1234","message":"…"}`},
	{Method: "GET", Path: "/api/repositories/{id}/tree", Group: "Repositories", Auth: true, Summary: "Repo file tree"},

	// ---- Environments ----
	{Method: "GET", Path: "/api/environments", Group: "Environments", Auth: true, Summary: "List environments (variable groups)"},
	{Method: "POST", Path: "/api/environments", Group: "Environments", Auth: true, Summary: "Create an environment",
		Request: `{"name":"prod","extraVars":{"region":"eu"},"envVars":{"AWS_REGION":"eu"},"secrets":[{"name":"TOKEN","type":"env","value":"…"}]}`},
	{Method: "GET", Path: "/api/environments/{id}", Group: "Environments", Auth: true, Summary: "Get an environment"},
	{Method: "PUT", Path: "/api/environments/{id}", Group: "Environments", Auth: true, Summary: "Update an environment"},
	{Method: "DELETE", Path: "/api/environments/{id}", Group: "Environments", Auth: true, Summary: "Delete an environment"},

	// ---- Schedules ----
	{Method: "GET", Path: "/api/schedules", Group: "Schedules", Auth: true, Summary: "List schedules"},
	{Method: "POST", Path: "/api/schedules", Group: "Schedules", Auth: true, Summary: "Create a schedule (cron or run-once) targeting a template OR a workflow",
		Request: `{"templateId":"tpl_…","name":"nightly","cron":"0 2 * * *"}  // or {"workflowId":"wf_…","cron":"0 3 * * *"} or {"templateId":"…","once":true,"runAt":"2026-06-10T02:00:00Z"}`},
	{Method: "GET", Path: "/api/schedules/{id}", Group: "Schedules", Auth: true, Summary: "Get a schedule"},
	{Method: "PUT", Path: "/api/schedules/{id}", Group: "Schedules", Auth: true, Summary: "Update a schedule"},
	{Method: "DELETE", Path: "/api/schedules/{id}", Group: "Schedules", Auth: true, Summary: "Delete a schedule"},

	// ---- Integrations ----
	{Method: "GET", Path: "/api/integrations", Group: "Integrations", Auth: true, Summary: "List webhook integrations"},
	{Method: "POST", Path: "/api/integrations", Group: "Integrations", Auth: true, Summary: "Create a webhook integration — targets a template (templateId) OR a workflow (workflowId)",
		Request: `{"templateId":"tpl_…","name":"github-push","authMethod":"github","authSecret":"…","passPayload":true,"aliases":["deploy"]}`},
	{Method: "PUT", Path: "/api/integrations/{id}", Group: "Integrations", Auth: true, Summary: "Update a webhook"},
	{Method: "DELETE", Path: "/api/integrations/{id}", Group: "Integrations", Auth: true, Summary: "Delete a webhook"},
	{Method: "POST", Path: "/api/webhooks/{token}", Group: "Integrations", Summary: "PUBLIC inbound webhook — fires the template OR workflow (authorised by the URL token + optional HMAC/Basic)",
		Params: []ParamDoc{{Name: "token", In: "path", Desc: "the webhook token (or an alias)"}}, Response: `{"triggered":true,"runId":"run_…"}`},

	// ---- Notifications ----
	{Method: "GET", Path: "/api/notifications", Group: "Notifications", Auth: true, Summary: "List notification channels"},
	{Method: "POST", Path: "/api/notifications", Group: "Notifications", Auth: true, Summary: "Create a channel — type telegram/slack/teams/discord/rocketchat/googlechat/gotify/ntfy/pushover/dingtalk/pagerduty/opsgenie/webhook/email. Optional `projectId` scopes it to one project (else all). Optional `template` customises the body with {{run}}/{{status}}/{{project}}/{{app}}/{{playbook}}/{{commit}}/{{version}}/{{actor}}/{{exitCode}} vars.",
		Request: `{"type":"telegram","name":"ops","enabled":true,"events":["failed"],"config":{"botToken":"…","chatId":"…"},"template":"{{run}} → {{status}} ({{project}})"}`},
	{Method: "PUT", Path: "/api/notifications/{id}", Group: "Notifications", Auth: true, Summary: "Update a channel"},
	{Method: "DELETE", Path: "/api/notifications/{id}", Group: "Notifications", Auth: true, Summary: "Delete a channel"},
	{Method: "POST", Path: "/api/notifications/{id}/test", Group: "Notifications", Auth: true, Summary: "Send a test notification", Response: `{"sent":true}`},
	{Method: "GET", Path: "/api/notification-logs", Group: "Notifications", Auth: true, Admin: true, Summary: "Delivery audit log — one row per dispatch attempt (channel, run, event, ok/error). Optional ?limit (default 100, max 500). Trimmed by the audit-retention policy."},
	{Method: "GET", Path: "/api/secret-access-logs", Group: "Meta", Auth: true, Admin: true, Summary: "Secret-access audit trail — one row per access to secret material (action run-secrets | pubkey | credential-use, with actor, credential, detail). Optional ?limit (default 100, max 500). Trimmed by the audit-retention policy."},

	// ---- Applications (execution backends) ----
	{Method: "GET", Path: "/api/applications", Group: "Applications", Auth: true, Summary: "List execution backends (built-ins + custom), with active state + base args"},
	{Method: "POST", Path: "/api/applications", Group: "Applications", Auth: true, Admin: true,
		Summary: "Register a custom application (arbitrary binary + base args)",
		Request: `{"id":"helmfile","name":"Helmfile","bin":"helmfile","args":["apply"]}`},
	{Method: "PUT", Path: "/api/applications/{id}", Group: "Applications", Auth: true, Admin: true, Summary: "Update an application (toggle active, tweak base args)"},
	{Method: "DELETE", Path: "/api/applications/{id}", Group: "Applications", Auth: true, Admin: true, Summary: "Delete a custom application"},

	// ---- WebSockets ----
	{Method: "GET", Path: "/ws/events", Group: "WebSocket", Auth: true, Summary: "Live event bus (run/project/activity updates)"},
	{Method: "GET", Path: "/ws/runs/{id}", Group: "WebSocket", Auth: true, Summary: "Live PTY stream for a run (replay + late-join)"},
}
