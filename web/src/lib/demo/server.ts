// The in-browser "server": maps fetch(path) → the demo DB. Reads are served from
// localStorage; writes mutate it and emit live events; launching a run starts a
// simulated PTY stream (see socket.ts). Anything unmatched returns an empty-ish
// shape so no page can crash.
import { getDB, saveDB, emitDemo, genId, nowISO } from "./db";
import { LOGS } from "./runlogs";
import { DEMO_ENDPOINTS } from "./apidocs";

type Body = Record<string, unknown> | undefined;
interface Ctx { params: Record<string, string>; query: URLSearchParams; body: Body; header: Record<string, string>; }
type Result = unknown; // returned value → JSON, or sentinels below
const NO_CONTENT = Symbol("204");
const text = (s: string) => ({ __text: s });
const notFound = () => ({ __status: 404, error: "not found" });

const idMatch = (p: string, path: string): Record<string, string> | null => {
  const pp = p.split("/"), ap = path.split("/");
  if (pp.length !== ap.length) return null;
  const params: Record<string, string> = {};
  for (let i = 0; i < pp.length; i++) {
    if (pp[i].startsWith(":")) params[pp[i].slice(1)] = decodeURIComponent(ap[i]);
    else if (pp[i] !== ap[i]) return null;
  }
  return params;
};

// small CRUD helpers over a collection array
const find = <T extends { id: string }>(arr: T[], id: string) => arr.find((x) => x.id === id);
const upd = <T extends { id: string; updatedAt?: string }>(arr: T[], id: string, body: Body): T | undefined => {
  const o = find(arr, id);
  if (o) Object.assign(o, body, { updatedAt: nowISO() });
  return o;
};
const del = (arr: { id: string }[], id: string) => {
  const i = arr.findIndex((x) => x.id === id);
  if (i >= 0) arr.splice(i, 1);
};

function startLiveRun(run: Record<string, unknown>) {
  const db = getDB();
  db.runLogs[run.id as string] = LOGS.deploy();
  // finish after the stream has had time to play
  window.setTimeout(() => {
    const r = find(db.runs, run.id as string);
    if (r && r.status === "running") {
      r.status = "success";
      r.finishedAt = nowISO();
      r.exitCode = 0;
      r.stats = { hosts: 2, ok: 8, changed: 3, unreachable: 0, failed: 0, skipped: 0, rescued: 0, ignored: 0 };
      db.activity.unshift({ id: genId("act"), actor: r.triggeredBy as string || "demo", action: "run.finish", target: r.name as string, detail: "success", createdAt: nowISO() });
      saveDB();
      emitDemo("run.updated");
      emitDemo("activity");
    }
  }, 13_000);
}

// ---- the route table (first match wins) ----
type Handler = (c: Ctx) => Result;
const R: [string, string, Handler][] = [];
const on = (method: string, pattern: string, h: Handler) => R.push([method, pattern, h]);

const db = () => getDB();
const me = () => db().users[0];

// meta / auth
on("GET", "/api/health", () => ({ status: "ok", service: "api" }));
on("GET", "/api/auth/status", () => ({ needsSetup: false, authenticated: true, user: me(), providers: { local: true, ldap: true, oidc: true, saml: true, github: true, bitbucket: true }, oidcAutoLogin: false, docsEnabled: db().settings.docsEnabled }));
on("GET", "/api/auth/me", () => me());
on("POST", "/api/auth/login", () => me());
on("POST", "/api/auth/logout", () => ({ ok: true }));
on("POST", "/api/auth/setup", () => me());
on("POST", "/api/auth/2fa/setup", () => ({ secret: "JBSWY3DPEHPK3PXP", otpauthUrl: "otpauth://totp/ansible-ui:demo?secret=JBSWY3DPEHPK3PXP&issuer=ansible-ui" }));
on("POST", "/api/auth/2fa/enable", () => { me().twoFactorEnabled = true; saveDB(); return { enabled: true }; });
on("POST", "/api/auth/2fa/disable", () => { me().twoFactorEnabled = false; saveDB(); return { enabled: false }; });
on("POST", "/api/auth/2fa/email/enable", () => { me().emailOtpEnabled = true; saveDB(); return { emailOtpEnabled: true }; });
on("POST", "/api/auth/2fa/email/disable", () => { me().emailOtpEnabled = false; saveDB(); return { emailOtpEnabled: false }; });
on("POST", "/api/users/:id/2fa/reset", () => ({ enabled: false }));

// settings
const setting = (key: keyof ReturnType<typeof db>["settings"], path: string) => {
  on("GET", path, () => db().settings[key]);
  on("PUT", path, (c) => { (db().settings[key] as unknown) = key === "notifyProxy" ? (c.body as { proxyUrl?: string })?.proxyUrl ?? "" : { ...(db().settings[key] as object), ...(c.body as object) }; saveDB(); return db().settings[key]; });
};
setting("smtp", "/api/settings/smtp");
setting("galaxy", "/api/settings/galaxy");
setting("authMapping", "/api/settings/auth-mapping");
setting("syslog", "/api/settings/syslog");
setting("flags", "/api/settings/flags");
setting("retention", "/api/settings/retention");
setting("tasks", "/api/settings/tasks");
on("GET", "/api/settings/notify-proxy", () => ({ proxyUrl: db().settings.notifyProxy }));
on("PUT", "/api/settings/notify-proxy", (c) => { db().settings.notifyProxy = (c.body as { proxyUrl?: string })?.proxyUrl ?? ""; saveDB(); return { proxyUrl: db().settings.notifyProxy }; });
on("GET", "/api/meta/endpoints", () => ({ enabled: db().settings.docsEnabled, baseUrl: "/api", authNote: "session cookie or Bearer API token", endpoints: DEMO_ENDPOINTS, models: [] }));
on("PUT", "/api/meta/docs", (c) => { db().settings.docsEnabled = !!(c.body as { enabled?: boolean })?.enabled; saveDB(); return { docsEnabled: db().settings.docsEnabled }; });

// users & tokens
on("GET", "/api/users", () => db().users);
on("POST", "/api/users", (c) => { const u = { id: genId("usr"), role: "user", twoFactorEnabled: false, createdAt: nowISO(), ...(c.body as object) } as never; db().users.push(u); saveDB(); return u; });
on("DELETE", "/api/users/:id", (c) => { del(db().users, c.params.id); saveDB(); return NO_CONTENT; });
on("GET", "/api/tokens", () => db().tokens);
on("POST", "/api/tokens", (c) => { const b = c.body as { name: string; expiresDays?: number }; const tok = { id: genId("tok"), userId: me().id, name: b.name, prefix: "aui_" + Math.random().toString(16).slice(2, 6), createdAt: nowISO(), expiresAt: b.expiresDays ? new Date(Date.now() + b.expiresDays * 864e5).toISOString() : null, token: "aui_" + Math.random().toString(36).slice(2) + Math.random().toString(36).slice(2) }; db().tokens.unshift(tok as never); saveDB(); return tok; });
on("DELETE", "/api/tokens/:id", (c) => { del(db().tokens, c.params.id); saveDB(); return NO_CONTENT; });

// generic collections: [path, key, idPrefix]
const collections: [string, keyof ReturnType<typeof db>, string][] = [
  ["/api/credentials", "credentials", "cred"],
  ["/api/repositories", "repositories", "repo"],
  ["/api/schedules", "schedules", "sch"],
  ["/api/integrations", "integrations", "int"],
  ["/api/applications", "applications", "app"],
  ["/api/notifications", "notifications", "ntf"],
  ["/api/roles", "roles", "role"],
  ["/api/environments", "environments", "env"],
  ["/api/secret-backends", "secretBackends", "sb"],
  ["/api/views", "views", "view"],
];
for (const [path, key, prefix] of collections) {
  on("GET", path, (c) => {
    let arr = db()[key] as { templateId?: string }[];
    const tid = c.query.get("templateId");
    if (tid) arr = arr.filter((x) => x.templateId === tid);
    return arr;
  });
  on("POST", path, (c) => { const o = { id: genId(prefix), createdAt: nowISO(), updatedAt: nowISO(), ...(c.body as object) } as never; (db()[key] as unknown[]).unshift(o); saveDB(); return o; });
  on("GET", `${path}/:id`, (c) => find(db()[key] as { id: string }[], c.params.id) ?? notFound());
  on("PUT", `${path}/:id`, (c) => { const o = upd(db()[key] as { id: string }[], c.params.id, c.body); saveDB(); return o ?? notFound(); });
  on("DELETE", `${path}/:id`, (c) => { del(db()[key] as { id: string }[], c.params.id); saveDB(); return NO_CONTENT; });
}
on("GET", "/api/credentials/:id/pubkey", () => ({ type: "ssh-ed25519", publicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIDemoKeyForAnsibleUiDemoEnvironment42 deploy@ansible-ui" }));
on("POST", "/api/repositories/:id/sync", (c) => { const r = find(db().repositories, c.params.id); if (r) { r.status = "ready"; r.lastSyncedAt = nowISO(); r.lastCommit = Math.random().toString(16).slice(2, 9); saveDB(); emitDemo("repository.updated"); } return { repository: r, commit: r?.lastCommit, message: "demo: synced" }; });
on("GET", "/api/repositories/:id/tree", () => demoTree());
on("POST", "/api/notifications/:id/test", () => ({ sent: true }));
on("GET", "/api/notification-logs", () => db().notificationLogs);
on("GET", "/api/secret-access-logs", () => db().secretAccessLogs);
on("POST", "/api/secret-backends/test", () => ({ ok: true, message: "Connected — 12 secrets readable at the configured mount." }));
on("GET", "/api/activity", () => db().activity.slice(0, Number(/* limit */ 0) || 200));

// projects
on("GET", "/api/projects", () => db().projects);
on("POST", "/api/projects", (c) => { const p = { id: genId("prj"), slug: ((c.body as { name?: string })?.name || "project").toLowerCase().replace(/\s+/g, "-"), description: "", path: "/data/projects/new", sourceType: "local", myRole: "admin", createdAt: nowISO(), updatedAt: nowISO(), ...(c.body as object) } as never; db().projects.unshift(p); saveDB(); emitDemo("project.created"); return p; });
on("GET", "/api/projects/:id", (c) => { const p = find(db().projects, c.params.id); return p ? { ...p, playbooks: ["deploy.yml", "facts.yml", "migrate.yml", "build.yml"], inventoryFiles: ["production.ini", "staging.ini"] } : notFound(); });
on("PUT", "/api/projects/:id", (c) => { const o = upd(db().projects, c.params.id, c.body); saveDB(); return o ?? notFound(); });
on("DELETE", "/api/projects/:id", (c) => { del(db().projects, c.params.id); saveDB(); emitDemo("project.created"); return NO_CONTENT; });
on("GET", "/api/projects/:id/export", (c) => ({ name: find(db().projects, c.params.id)?.name, version: "1.0", templates: db().templates.filter((t) => t.projectId === c.params.id) }));
on("POST", "/api/projects/import", () => { const p = { id: genId("prj"), slug: "imported", name: "Imported project", description: "from a bundle", path: "/data/projects/imported", sourceType: "local", myRole: "admin", createdAt: nowISO(), updatedAt: nowISO() } as never; db().projects.unshift(p); saveDB(); emitDemo("project.created"); return p; });
on("GET", "/api/projects/:id/members", (c) => db().members.filter((m) => m.projectId === c.params.id));
on("POST", "/api/projects/:id/members", (c) => { const b = c.body as { userId: string; role: string }; const u = find(db().users, b.userId); db().members.push({ projectId: c.params.id, userId: b.userId, username: u?.username || "user", email: u?.email, role: b.role as never, createdAt: nowISO() }); saveDB(); return { ok: true }; });
on("DELETE", "/api/projects/:id/members/:uid", (c) => { const i = db().members.findIndex((m) => m.projectId === c.params.id && m.userId === c.params.uid); if (i >= 0) db().members.splice(i, 1); saveDB(); return { ok: true }; });
on("GET", "/api/projects/:id/files", () => demoTree());
on("GET", "/api/projects/:id/requirements", () => ({ files: ["requirements.yml"], items: [{ kind: "collection", name: "community.general", version: ">=8.0.0", source: "galaxy", url: "https://galaxy.ansible.com/community/general" }, { kind: "collection", name: "ansible.posix", source: "galaxy", url: "https://galaxy.ansible.com/ansible/posix" }, { kind: "role", name: "geerlingguy.nginx", source: "galaxy", url: "https://galaxy.ansible.com/geerlingguy/nginx" }] }));
on("GET", "/api/projects/:id/file", (c) => ({ path: c.query.get("path") || "deploy.yml", size: 420, content: DEMO_FILE }));
on("PUT", "/api/projects/:id/file", () => ({ saved: true }));
on("DELETE", "/api/projects/:id/file", () => ({ deleted: true }));
on("POST", "/api/projects/:id/rename", (c) => ({ path: (c.body as { to?: string })?.to || "", renamed: true }));
on("POST", "/api/projects/:id/git-commit", (c) => { const b = c.body as { branch: string; files: { path: string }[]; message: string }; const br = { id: genId("br"), projectId: c.params.id, branch: b.branch, commit: Math.random().toString(16).slice(2, 9), prUrl: "https://github.com/acme/infra/pull/" + (140 + Math.floor(Math.random() * 50)), files: b.files.map((f) => f.path), actor: me().username, createdAt: nowISO() }; db().branches.unshift(br as never); saveDB(); emitDemo("branches"); return { branch: b.branch, commit: br.commit, pushed: true, prUrl: br.prUrl }; });
on("GET", "/api/projects/:id/branches", (c) => db().branches.filter((b) => b.projectId === c.params.id));

// inventories (project-scoped create/list + flat get/put/delete)
on("GET", "/api/projects/:id/inventories", (c) => db().inventories.filter((i) => i.projectId === c.params.id));
on("POST", "/api/projects/:id/inventories", (c) => { const o = { id: genId("inv"), projectId: c.params.id, type: "static", createdAt: nowISO(), updatedAt: nowISO(), ...(c.body as object) } as never; db().inventories.unshift(o); saveDB(); return o; });
on("PUT", "/api/inventories/:id", (c) => { const o = upd(db().inventories, c.params.id, c.body); saveDB(); return o ?? notFound(); });
on("DELETE", "/api/inventories/:id", (c) => { del(db().inventories, c.params.id); saveDB(); return NO_CONTENT; });
on("GET", "/api/inventories/:id/hosts", () => ({ groups: [{ name: "web", hosts: ["web-01.prod", "web-02.prod"] }, { name: "db", hosts: ["db-01.prod"] }], hosts: ["web-01.prod", "web-02.prod", "db-01.prod"], total: 3 }));
on("POST", "/api/inventories/:id/facts/gather", () => ({ gathered: ["web-01.prod", "web-02.prod", "db-01.prod"], count: 3 }));
on("POST", "/api/inventories/:id/monitor", () => ({ monitored: 3 }));
on("DELETE", "/api/inventories/:id/monitor", () => ({ removed: 3 }));
on("GET", "/api/monitors", () => db().monitors);
on("POST", "/api/monitors/check", () => { emitDemo("activity"); return { checked: true }; });

// hosts
on("GET", "/api/hosts", () => db().hosts);
on("GET", "/api/hosts/:host", (c) => db().hosts.find((h) => h.host === c.params.host) ?? notFound());
on("DELETE", "/api/hosts/:host", (c) => { const i = db().hosts.findIndex((h) => h.host === c.params.host); if (i >= 0) db().hosts.splice(i, 1); saveDB(); return NO_CONTENT; });

// templates
on("GET", "/api/templates", (c) => { const pid = c.query.get("projectId"); return pid ? db().templates.filter((t) => t.projectId === pid) : db().templates; });
on("GET", "/api/templates/all", () => db().templates.map((t) => ({ ...t, projectName: find(db().projects, t.projectId)?.name })));
on("GET", "/api/templates/:id", (c) => find(db().templates, c.params.id) ?? notFound());
on("POST", "/api/templates", (c) => { const o = { id: genId("tpl"), surveyVars: [], limit: "", tags: "", skipTags: "", extraVars: {}, check: false, diff: false, verbosity: 0, createdAt: nowISO(), updatedAt: nowISO(), ...(c.body as object) } as never; db().templates.unshift(o); saveDB(); return o; });
on("PUT", "/api/templates/:id", (c) => { const o = upd(db().templates, c.params.id, c.body); saveDB(); return o ?? notFound(); });
on("DELETE", "/api/templates/:id", (c) => { del(db().templates, c.params.id); saveDB(); return NO_CONTENT; });
on("POST", "/api/templates/:id/run", (c) => launchRun(c.params.id, c.body));

// workflows
on("GET", "/api/workflows", () => db().workflows.map((w) => ({ ...w, projectName: find(db().projects, w.projectId)?.name })));
on("GET", "/api/projects/:id/workflows", (c) => db().workflows.filter((w) => w.projectId === c.params.id));
on("POST", "/api/projects/:id/workflows", (c) => { const o = { id: genId("wf"), projectId: c.params.id, steps: [], variables: {}, description: "", createdAt: nowISO(), updatedAt: nowISO(), ...(c.body as object) } as never; db().workflows.unshift(o); saveDB(); emitDemo("workflow"); return o; });
on("GET", "/api/workflows/:id", (c) => find(db().workflows, c.params.id) ?? notFound());
on("PUT", "/api/workflows/:id", (c) => { const o = upd(db().workflows, c.params.id, c.body); saveDB(); emitDemo("workflow"); return o ?? notFound(); });
on("DELETE", "/api/workflows/:id", (c) => { del(db().workflows, c.params.id); saveDB(); emitDemo("workflow"); return { deleted: true }; });
on("GET", "/api/workflows/:id/versions", (c) => db().workflowVersions.filter((v) => v.workflowId === c.params.id));
on("POST", "/api/workflows/:id/rollback/:vid", (c) => { const w = find(db().workflows, c.params.id); saveDB(); emitDemo("workflow"); return w ?? notFound(); });
on("POST", "/api/workflows/:id/run", (c) => { const w = find(db().workflows, c.params.id); const wr = { id: genId("wfr"), workflowId: c.params.id, projectId: w?.projectId || "prj_prod", name: w?.name || "Workflow", triggeredBy: me().username, status: "running", workflowName: w?.name, projectName: find(db().projects, w?.projectId || "")?.name, createdAt: nowISO(), startedAt: nowISO(), steps: (w?.steps || []).map((s, i) => ({ ...s, status: i === 0 ? "running" : "pending" })) }; db().workflowRuns.unshift(wr as never); saveDB(); emitDemo("workflow"); return wr; });
on("GET", "/api/workflow-runs", (c) => { let a = db().workflowRuns; const pid = c.query.get("projectId"), wid = c.query.get("workflowId"); if (pid) a = a.filter((x) => x.projectId === pid); if (wid) a = a.filter((x) => x.workflowId === wid); return a; });
on("GET", "/api/workflow-runs/:id", (c) => find(db().workflowRuns, c.params.id) ?? notFound());

// runs
on("GET", "/api/runs", (c) => { let a = [...db().runs]; const pid = c.query.get("projectId"), st = c.query.get("status"), lim = c.query.get("limit"); if (pid) a = a.filter((r) => r.projectId === pid); if (st) a = a.filter((r) => r.status === st); a.sort((x, y) => (y.createdAt > x.createdAt ? 1 : -1)); if (lim) a = a.slice(0, Number(lim)); return a; });
on("DELETE", "/api/runs", (c) => { const pid = c.query.get("projectId"); const before = db().runs.length; db().runs = db().runs.filter((r) => (pid ? r.projectId !== pid : false)); saveDB(); emitDemo("run.updated"); return { deleted: before - db().runs.length }; });
on("GET", "/api/runs/:id", (c) => { const r = find(db().runs, c.params.id); if (!r) return notFound(); return { run: { ...r, projectName: find(db().projects, r.projectId)?.name }, active: ["running", "queued", "pending", "awaiting"].includes(r.status) }; });
on("GET", "/api/runs/:id/secrets", () => ({ ANSIBLE_VAULT_PASSWORD: "••••••••", API_KEY: "••••••••" }));
on("POST", "/api/runs", (c) => launchRun((c.body as { templateId?: string })?.templateId || "", c.body, true));
on("POST", "/api/runs/:id/cancel", (c) => { const r = find(db().runs, c.params.id); if (r) { r.status = "canceled"; r.finishedAt = nowISO(); saveDB(); emitDemo("run.updated"); } return { status: "canceled" }; });
on("POST", "/api/runs/:id/approve", (c) => { const r = find(db().runs, c.params.id); if (r) { r.status = "running"; r.startedAt = nowISO(); startLiveRun(r as never); saveDB(); emitDemo("run.updated"); } return { status: "running" }; });
on("POST", "/api/runs/:id/reject", (c) => { const r = find(db().runs, c.params.id); if (r) { r.status = "canceled"; r.finishedAt = nowISO(); saveDB(); emitDemo("run.updated"); } return { status: "canceled" }; });
on("DELETE", "/api/runs/:id", (c) => { del(db().runs, c.params.id); saveDB(); emitDemo("run.updated"); return { deleted: 1 }; });
on("GET", "/api/runs/:id/log", (c) => {
  const r = find(db().runs, c.params.id);
  // While a run is live, return empty so LogView animates from the WS stream
  // (the stored log reconciles once it finishes).
  if (r && ["running", "queued", "pending", "awaiting"].includes(r.status)) return { output: "", times: [] };
  const out = db().runLogs[c.params.id] || "";
  const lines = out.split("\n").length; const t0 = Date.now() - 60000;
  return { output: out, times: Array.from({ length: lines }, (_, i) => t0 + i * 250) };
});
on("GET", "/api/runs/:id/output", (c) => text(db().runLogs[c.params.id] || ""));
on("GET", "/api/runs/:id/artifacts", (c) => { const r = find(db().runs, c.params.id); return r && r.status === "success" && r.app === "ansible" ? [{ id: "art_" + c.params.id, runId: c.params.id, name: "deploy-report.json", size: 2048, createdAt: r.finishedAt || nowISO() }] : []; });

// dashboards
on("GET", "/api/stats", () => stats());
on("GET", "/api/insights", (c) => insights(Number(c.query.get("days")) || 14));
on("GET", "/api/cluster", () => cluster());
on("GET", "/api/system", () => system());
on("GET", "/api/runners", () => db().runners);
on("DELETE", "/api/runners/:id", (c) => { del(db().runners, c.params.id); saveDB(); emitDemo("runners"); return NO_CONTENT; });

// ---- computed views ----
function launchRun(templateId: string, body: Body, adhoc = false): Result {
  const t = find(db().templates, templateId);
  const ov = (body || {}) as Record<string, unknown>;
  const run = {
    id: genId("run"), projectId: t?.projectId || (ov.projectId as string) || "prj_prod", templateId: templateId || null,
    name: (ov.name as string) || t?.name || "Ad-hoc run", triggeredBy: me().username, app: t?.app || (ov.app as string) || "ansible",
    action: t?.action, commit: "a1b2c3d", version: "#" + (480 + Math.floor(Math.random() * 20)), runnerId: "runner_eu", runnerName: "eu-west-1",
    playbook: t?.playbook || (ov.playbook as string) || "deploy.yml", status: t?.requiresApproval ? "awaiting" : "running",
    exitCode: null, limit: (ov.limit as string) || t?.limit || "", tags: (ov.tags as string) || t?.tags || "", skipTags: "", extraVars: (ov.extraVars as object) || {},
    check: !!ov.check, diff: !!(ov.diff ?? t?.diff), verbosity: (ov.verbosity as number) ?? t?.verbosity ?? 0, args: [],
    stats: { hosts: 0, ok: 0, changed: 0, unreachable: 0, failed: 0, skipped: 0, rescued: 0, ignored: 0 },
    createdAt: nowISO(), startedAt: nowISO(), finishedAt: null,
  };
  db().runs.unshift(run as never);
  db().activity.unshift({ id: genId("act"), actor: me().username, action: "run.launch", target: run.name, detail: run.version || "", createdAt: nowISO() });
  if (run.status === "running") startLiveRun(run);
  saveDB();
  emitDemo("run.updated"); emitDemo("activity");
  void adhoc;
  return run;
}

function stats() {
  const runs = db().runs;
  const byState: Record<string, number> = {};
  for (const r of runs) byState[r.status] = (byState[r.status] || 0) + 1;
  return { projects: db().projects.length, templates: db().templates.length, runsTotal: runs.length, runsByState: byState, active: (byState.running || 0) + (byState.queued || 0) + (byState.awaiting || 0), recentRuns: [...runs].sort((a, b) => (b.createdAt > a.createdAt ? 1 : -1)).slice(0, 8).map((r) => ({ ...r, projectName: find(db().projects, r.projectId)?.name })) };
}

function insights(days: number) {
  const runs = db().runs;
  const perDay: { date: string; success: number; failed: number; other: number; total: number }[] = [];
  for (let i = days - 1; i >= 0; i--) {
    const d = new Date(Date.now() - i * 864e5).toISOString().slice(0, 10);
    const dayRuns = runs.filter((r) => r.createdAt.slice(0, 10) === d);
    const success = dayRuns.filter((r) => r.status === "success").length;
    const failed = dayRuns.filter((r) => r.status === "failed").length;
    perDay.push({ date: d, success, failed, other: dayRuns.length - success - failed, total: dayRuns.length });
  }
  const finished = runs.filter((r) => r.status === "success" || r.status === "failed");
  const success = finished.filter((r) => r.status === "success").length;
  const byTpl = new Map<string, { name: string; runs: number; ok: number }>();
  for (const r of finished) {
    if (!r.templateId) continue;
    const e = byTpl.get(r.templateId) || { name: find(db().templates, r.templateId)?.name || r.name, runs: 0, ok: 0 };
    e.runs++; if (r.status === "success") e.ok++; byTpl.set(r.templateId, e);
  }
  return { days, perDay, total: finished.length, successRate: finished.length ? success / finished.length : 1, avgDurationSec: 92, avgWaitSec: 4, byTemplate: Array.from(byTpl, ([templateId, v]) => ({ templateId, templateName: v.name, runs: v.runs, successRate: v.runs ? v.ok / v.runs : 1, avgDurationSec: 80 + Math.floor(Math.random() * 60) })).sort((a, b) => b.runs - a.runs).slice(0, 6) };
}

function cluster() {
  const runs = db().runs;
  const running = runs.filter((r) => r.status === "running");
  const queued = runs.filter((r) => r.status === "queued" || r.status === "awaiting");
  return { leader: true, runners: db().runners.map((r) => ({ id: r.id, name: r.name, online: r.status === "online", builtin: r.builtin, tags: r.tags, active: running.filter((x) => x.runnerId === r.id).length, maxConcurrent: r.maxConcurrent || 4 })), queued, running, counts: { queued: queued.length, running: running.length, awaiting: runs.filter((r) => r.status === "awaiting").length, runnersOnline: db().runners.filter((r) => r.status === "online").length } };
}

function system() {
  return { system: { version: "1.0.0", goVersion: "go1.25", platform: "linux/amd64" }, ansible: "ansible-core 2.20.6", database: { dialect: "postgres (demo: in-browser)" }, auth: { local: true, ldap: true, oidc: true, saml: true }, notifications: db().notifications.map((n) => ({ type: n.type, configured: true })), cluster: { highAvailability: true }, runners: { remoteRunners: true, total: db().runners.length, online: db().runners.filter((r) => r.status === "online").length }, taskSettings: { maxDurationSec: db().settings.tasks.maxDurationSec }, featureFlags: db().settings.flags as unknown as Record<string, boolean> };
}

function demoTree() {
  return [
    { name: "deploy.yml", path: "deploy.yml", type: "file", size: 1240 },
    { name: "facts.yml", path: "facts.yml", type: "file", size: 640 },
    { name: "requirements.yml", path: "requirements.yml", type: "file", size: 180 },
    { name: "group_vars", path: "group_vars", type: "dir", children: [{ name: "all.yml", path: "group_vars/all.yml", type: "file", size: 320 }] },
    { name: "roles", path: "roles", type: "dir", children: [{ name: "common", path: "roles/common", type: "dir", children: [{ name: "tasks", path: "roles/common/tasks", type: "dir", children: [{ name: "main.yml", path: "roles/common/tasks/main.yml", type: "file", size: 210 }] }] }] },
  ];
}

const DEMO_FILE = `---
- name: Deploy web tier
  hosts: web
  become: true
  vars:
    app_version: "{{ version | default('1.4.2') }}"
  tasks:
    - name: Pull application artifact
      ansible.builtin.get_url:
        url: "https://artifacts.acme.io/app-{{ app_version }}.tar.gz"
        dest: /opt/app/releases/
      notify: restart app

    - name: Render config from template
      ansible.builtin.template:
        src: app.conf.j2
        dest: /etc/app/app.conf

    - name: Wait for health endpoint
      ansible.builtin.uri:
        url: http://localhost:8080/healthz
      register: health
      until: health.status == 200
      retries: 5
  handlers:
    - name: restart app
      ansible.builtin.systemd: { name: app, state: restarted }
`;

// ---- the fetch shim entry point ----
export async function demoFetch(input: RequestInfo | URL, init?: RequestInit): Promise<Response> {
  const rawUrl = typeof input === "string" ? input : input instanceof URL ? input.toString() : input.url;
  const method = (init?.method || (typeof input !== "string" && !(input instanceof URL) ? input.method : "GET") || "GET").toUpperCase();
  const u = new URL(rawUrl, location.origin);
  const path = u.pathname;
  let body: Body;
  if (init?.body && typeof init.body === "string") { try { body = JSON.parse(init.body); } catch { body = undefined; } }
  const header: Record<string, string> = {};
  const h = init?.headers;
  if (h) { if (h instanceof Headers) h.forEach((v, k) => (header[k.toLowerCase()] = v)); else for (const [k, v] of Object.entries(h as Record<string, string>)) header[k.toLowerCase()] = v; }

  // non-/api → let it through (assets etc.)
  if (!path.startsWith("/api/")) return realFetch(input, init);

  await new Promise((r) => setTimeout(r, 60 + Math.random() * 120)); // a little latency for realism

  for (const [m, pattern, handler] of R) {
    if (m !== method) continue;
    const params = idMatch(pattern, path);
    if (!params) continue;
    let out: Result;
    try { out = handler({ params, query: u.searchParams, body, header }); }
    catch (e) { return json({ error: String(e) }, 500); }
    if (out === NO_CONTENT) return new Response(null, { status: 204 });
    if (out && typeof out === "object" && "__text" in out) return new Response((out as { __text: string }).__text, { status: 200, headers: { "content-type": "text/plain" } });
    if (out && typeof out === "object" && "__status" in out) return json(out, (out as { __status: number }).__status);
    return json(out, 200);
  }
  console.warn("[demo] unhandled", method, path);
  return json(method === "GET" ? [] : { ok: true }, 200);
}

const realFetch = window.fetch.bind(window);
function json(data: unknown, status: number) {
  return new Response(JSON.stringify(data), { status, headers: { "content-type": "application/json" } });
}
