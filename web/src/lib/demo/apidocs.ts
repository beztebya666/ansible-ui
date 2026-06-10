// A representative slice of the real /api/meta/endpoints catalogue so the demo's
// API Explorer is populated and its "try it" calls hit the in-browser mock.
import type { EndpointDoc } from "../api";

const e = (method: string, path: string, group: string, summary: string, auth = true, admin = false): EndpointDoc =>
  ({ method, path, group, summary, auth, admin });

export const DEMO_ENDPOINTS: EndpointDoc[] = [
  e("GET", "/api/health", "Meta", "Liveness probe", false),
  e("GET", "/api/stats", "Meta", "Dashboard counters"),
  e("GET", "/api/insights", "Meta", "Run analytics over a recent window"),
  e("GET", "/api/cluster", "Meta", "HA dashboard — leader, runner capacity, live queue", true, true),
  e("GET", "/api/system", "Meta", "System / build / feature info"),
  e("GET", "/api/activity", "Meta", "Activity / audit feed"),
  e("POST", "/api/mcp", "Meta", "Model Context Protocol server for AI agents"),
  e("GET", "/api/audit/export", "Meta", "Download the audit log as CSV", true, true),

  e("POST", "/api/auth/login", "Auth", "Sign in (returns the user or a 2FA challenge)", false),
  e("GET", "/api/auth/me", "Auth", "The current user"),
  e("POST", "/api/auth/2fa/setup", "Auth", "Begin TOTP enrolment (returns a QR)"),
  e("GET", "/api/users", "Auth", "List users", true, true),
  e("POST", "/api/users", "Auth", "Create a user", true, true),
  e("GET", "/api/tokens", "Auth", "List API tokens"),
  e("POST", "/api/tokens", "Auth", "Mint an API token"),

  e("GET", "/api/projects", "Projects", "List projects"),
  e("POST", "/api/projects", "Projects", "Create a project"),
  e("GET", "/api/projects/{id}/export", "Projects", "Export a project bundle"),
  e("POST", "/api/projects/{id}/git-commit", "Projects", "Commit edited files to a new branch + open a PR"),
  e("GET", "/api/projects/{id}/members", "Projects", "List project members"),

  e("GET", "/api/templates", "Templates", "List templates (optionally by project)"),
  e("POST", "/api/templates", "Templates", "Create a template"),
  e("POST", "/api/templates/{id}/run", "Templates", "Launch a run from a template"),

  e("GET", "/api/runs", "Runs", "List runs (filter by project/status)"),
  e("POST", "/api/runs", "Runs", "Launch an ad-hoc run"),
  e("GET", "/api/runs/{id}", "Runs", "Get a run + whether it's active"),
  e("GET", "/api/runs/{id}/log", "Runs", "Stored output + per-line timestamps"),
  e("POST", "/api/runs/{id}/cancel", "Runs", "Cancel a running task"),
  e("POST", "/api/runs/{id}/approve", "Runs", "Approve a gated run"),

  e("GET", "/api/workflows", "Workflows", "List all workflows"),
  e("POST", "/api/workflows/{id}/run", "Workflows", "Run a workflow pipeline"),
  e("GET", "/api/workflow-runs", "Workflows", "List workflow runs"),

  e("GET", "/api/schedules", "Schedules", "List schedules (cron + one-time)"),
  e("GET", "/api/integrations", "Integrations", "List webhook integrations"),
  e("GET", "/api/inventories/{id}/hosts", "Inventories", "Parsed hosts/groups for an inventory"),
  e("GET", "/api/credentials", "Key Store", "List credentials (no secrets)"),
  e("GET", "/api/secret-backends", "Key Store", "External secret managers (Vault/AWS/Azure/GCP)", true, true),
  e("GET", "/api/environments", "Environments", "List environments (vars + secrets)"),
  e("GET", "/api/hosts", "Hosts", "Stored host facts"),
  e("GET", "/api/notifications", "Notifications", "List notification channels", true, true),
  e("GET", "/api/runners", "Runners", "List runners + capacity", true, true),
  e("GET", "/api/roles", "Auth", "Custom project roles", true, true),
  e("GET", "/api/meta/endpoints", "Meta", "This API catalogue (powers the Explorer)"),

  e("GET", "/ws/events", "WebSocket", "Live event bus (lists/dashboard update with no refresh)"),
  e("GET", "/ws/runs/{id}", "WebSocket", "Live PTY stream for a run (the terminal)"),
];
