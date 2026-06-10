// Typed client for the ansible-ui control-plane API.

export type RunStatus = "awaiting" | "queued" | "pending" | "running" | "success" | "failed" | "canceled";

export type AppId =
  | "ansible"
  | "terraform"
  | "tofu"
  | "terragrunt"
  | "pulumi"
  | "bash"
  | "powershell"
  | "python";

export interface Project {
  id: string;
  slug: string;
  name: string;
  description: string;
  path: string;
  sourceType: string;
  gitUrl?: string;
  gitBranch?: string;
  repositoryId?: string | null;
  subPath?: string;
  createdAt: string;
  updatedAt: string;
  playbooks?: string[];
  inventoryFiles?: string[];
  repositoryName?: string;
  myRole?: ProjectRole;
}

export interface User {
  id: string;
  username: string;
  email: string;
  role: string;
  twoFactorEnabled?: boolean;
  emailOtpEnabled?: boolean;
  createdAt: string;
}

export interface GroupRule {
  group: string;
  role: string;
}
export interface AuthMapping {
  rules: GroupRule[];
  ldapGroupAttr: string;
  oidcGroupsClaim: string;
  ldapDebug?: boolean;
  oidcRequiredClaim?: string;
  oidcAutoLogin?: boolean;
}

export interface SMTPConfig {
  host: string;
  port: string;
  from: string;
  username: string;
  password: string;
}

export interface GalaxyConfig {
  serverUrl: string;
  token: string;
  cliArgs: string;
}

export type ProjectRole = "viewer" | "editor" | "admin";
export interface ProjectMember {
  projectId: string;
  userId: string;
  username: string;
  email?: string;
  role: ProjectRole;
  createdAt: string;
}

export interface Credential {
  id: string;
  name: string;
  type: "ssh" | "login_password" | "vault" | "gcp" | "azure";
  login?: string;
  hasSecret: boolean;
  sshCertificate?: string;
  ownerUserId?: string | null;
  createdAt: string;
  updatedAt: string;
}

export interface Repository {
  id: string;
  name: string;
  gitUrl: string;
  branch: string;
  credentialId?: string | null;
  status: "unknown" | "syncing" | "ready" | "error";
  lastCommit?: string;
  lastError?: string;
  lastSyncedAt?: string | null;
  cacheEnabled?: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface AuthStatus {
  needsSetup: boolean;
  authenticated: boolean;
  user: User | null;
  providers?: { local: boolean; ldap: boolean; oidc: boolean; saml?: boolean; github?: boolean; bitbucket?: boolean };
  oidcAutoLogin?: boolean;
  docsEnabled?: boolean;
}

export interface ParamDoc {
  name: string;
  in: string;
  desc: string;
}

export interface EndpointDoc {
  method: string;
  path: string;
  group: string;
  summary: string;
  auth: boolean;
  admin: boolean;
  params?: ParamDoc[];
  request?: string;
  response?: string;
}

export interface ApiModelField {
  name: string;
  type: string;
}
export interface ApiModelDoc {
  name: string;
  description: string;
  fields: ApiModelField[];
}
export interface ApiDocs {
  enabled: boolean;
  baseUrl: string;
  authNote: string;
  endpoints: EndpointDoc[];
  models?: ApiModelDoc[];
}

export interface APIToken {
  id: string;
  userId: string;
  name: string;
  prefix: string;
  lastUsedAt?: string | null;
  expiresAt?: string | null;
  createdAt: string;
  token?: string; // present only on creation
}

export interface SurveyVar {
  name: string;
  title: string;
  type: "text" | "int" | "enum" | "secret";
  required: boolean;
  default?: string;
  description?: string;
  options?: string[];
}

export interface EnvSecret {
  name: string;
  type: "env" | "var";
  value?: string; // write-only
  hasValue?: boolean; // read-only
}

export interface ExternalSecretRef {
  name: string;
  backend: string;
  path: string;
  field?: string;
  asVar?: boolean;
}

export interface Environment {
  id: string;
  name: string;
  description: string;
  extraVars: Record<string, unknown>;
  envVars: Record<string, string>;
  secrets: EnvSecret[];
  externalSecrets?: ExternalSecretRef[];
  createdAt: string;
  updatedAt: string;
}

export interface SecretBackend {
  id: string;
  name: string;
  type: "vault" | "aws" | "azure" | "gcp" | "cyberark";
  address: string;
  mount?: string;
  namespace?: string;
  insecure?: boolean;
  region?: string; // aws
  tenantId?: string; // azure
  accessKeyId?: string; // aws key id / azure client id / gcp project id
  token?: string; // write-only secret credential
  hasToken?: boolean; // read-only
  createdAt: string;
  updatedAt: string;
}

export interface Schedule {
  id: string;
  templateId: string;
  workflowId?: string;
  name: string;
  cron: string;
  once: boolean;
  active: boolean;
  lastRunAt?: string | null;
  nextRunAt?: string | null;
  createdAt: string;
  updatedAt: string;
  templateName?: string;
  projectId?: string;
}

export type IntegrationAuth = "none" | "token" | "hmac" | "github" | "bitbucket" | "basic";

export interface Integration {
  id: string;
  templateId: string;
  workflowId?: string;
  name: string;
  token: string;
  active: boolean;
  authMethod: IntegrationAuth;
  authHeader?: string;
  passPayload: boolean;
  aliases?: string[];
  hasSecret: boolean;
  authSecret?: string; // write-only
  lastTriggeredAt?: string | null;
  createdAt: string;
  templateName?: string;
  projectId?: string;
}

export interface Activity {
  id: string;
  actor: string;
  action: string;
  target: string;
  detail: string;
  createdAt: string;
}

export interface Application {
  id: string;
  name: string;
  icon: string;
  bin: string;
  args: string[];
  priority: number;
  active: boolean;
  kind: "builtin" | "custom";
  createdAt: string;
  updatedAt: string;
}

export type NotifyType =
  | "telegram" | "slack" | "webhook" | "email"
  | "discord" | "teams" | "gotify" | "rocketchat" | "ntfy" | "googlechat" | "pushover" | "dingtalk" | "pagerduty" | "opsgenie";

export interface NotificationChannel {
  id: string;
  type: NotifyType;
  name: string;
  enabled: boolean;
  events: string[];
  config: Record<string, string>;
  projectId?: string | null;
  template?: string;
  createdAt: string;
  updatedAt: string;
}

export interface NotificationLog {
  id: string;
  channelId: string;
  channelName: string;
  channelType: NotifyType | string;
  runId: string;
  runName: string;
  projectId: string;
  event: string;
  ok: boolean;
  error?: string;
  createdAt: string;
}

export interface HostFacts {
  host: string;
  inventoryId?: string;
  facts?: Record<string, unknown>;
  gatheredAt: string;
  os?: string;
  distro?: string;
  ip?: string;
  kernel?: string;
}

export interface HostMonitor {
  host: string;
  inventoryId: string;
  status: "up" | "down" | "unknown";
  consecFails: number;
  threshold: number;
  lastError?: string;
  lastCheckedAt?: string | null;
  lastUpAt?: string | null;
  createdAt: string;
}

export type ProjectCap = "view" | "run" | "edit" | "manage";
export interface CustomRole {
  id: string;
  name: string;
  description?: string;
  permissions: ProjectCap[];
  createdAt: string;
  updatedAt: string;
}

export interface SecretAccessLog {
  id: string;
  actor: string;
  action: string; // run-secrets | pubkey | credential-use
  credentialId?: string;
  credentialName?: string;
  detail?: string;
  createdAt: string;
}

export interface InventoryHosts {
  groups: { name: string; hosts: string[]; children?: string[] }[];
  hosts: string[];
  total: number;
  unsupported?: boolean;
  message?: string;
}

export type InventoryType = "static" | "file" | "dynamic" | "cloud";

export interface Inventory {
  id: string;
  projectId: string;
  name: string;
  type: InventoryType;
  content: string;
  provider?: string; // cloud: aws_ec2 (…)
  credentialId?: string | null; // cloud
  region?: string; // cloud
  runnerTag?: string; // affinity: pin runs using this inventory to a runner with this tag
  connCredentialIds?: string[]; // non-cloud: SSH credentials used to connect to the hosts
  createdAt: string;
  updatedAt: string;
}

export interface Template {
  id: string;
  projectId: string;
  name: string;
  description: string;
  app: AppId;
  action?: string;
  playbook: string;
  inventoryId?: string | null;
  inventoryIds?: string[];
  maxConcurrent?: number;
  vaultCredentialId?: string | null;
  environmentId?: string | null;
  surveyVars: SurveyVar[];
  limit: string;
  tags: string;
  skipTags: string;
  extraVars: Record<string, unknown>;
  check: boolean;
  diff: boolean;
  verbosity: number;
  cliArgs?: string[];
  prompts?: TemplatePrompts;
  allowParallel?: boolean;
  autorunOnCommit?: boolean;
  suppressSuccessNotifications?: boolean;
  suppressAllNotifications?: boolean;
  workspace?: string;
  autoApprove?: boolean;
  vaults?: string[];
  type?: "task" | "build" | "deploy";
  buildTemplateId?: string | null;
  runnerTag?: string;
  requiresApproval?: boolean;
  artifactPaths?: string[];
  tfBackend?: string;
  lastBuildVersion?: string;
  projectName?: string; // hydrated by /api/templates/all for the cross-project workflow picker
  createdAt: string;
  updatedAt: string;
}

export interface Runner {
  id: string;
  name: string;
  url: string;
  tags: string[];
  platform?: string;
  version?: string;
  maxConcurrent?: number;
  builtin: boolean;
  status: "online" | "offline";
  lastSeenAt?: string | null;
  createdAt: string;
  updatedAt: string;
}

export interface TemplatePrompts {
  cliArgs?: boolean;
  branch?: boolean;
  inventory?: boolean;
  limit?: boolean;
  tags?: boolean;
  skipTags?: boolean;
  debug?: boolean;
  vaults?: boolean;
}

export interface TemplateView {
  id: string;
  name: string;
  app: string;
  search: string;
  position: number;
  createdAt: string;
  updatedAt: string;
}

export interface RunStats {
  hosts: number;
  ok: number;
  changed: number;
  unreachable: number;
  failed: number;
  skipped: number;
  rescued: number;
  ignored: number;
}

export interface Run {
  id: string;
  projectId: string;
  templateId?: string | null;
  environmentId?: string | null;
  name: string;
  triggeredBy?: string;
  app: AppId;
  action?: string;
  commit?: string;
  version?: string;
  runnerId?: string;
  runnerName?: string;
  playbook: string;
  status: RunStatus;
  exitCode?: number | null;
  limit: string;
  tags: string;
  skipTags: string;
  extraVars: Record<string, unknown>;
  check: boolean;
  diff: boolean;
  verbosity: number;
  cliArgs?: string[];
  workspace?: string;
  artifactPaths?: string[];
  autoApprove?: boolean;
  args: string[];
  stats: RunStats;
  createdAt: string;
  startedAt?: string | null;
  finishedAt?: string | null;
  projectName?: string;
}

export interface DashboardStats {
  projects: number;
  templates: number;
  runsTotal: number;
  runsByState: Record<string, number>;
  active: number;
  recentRuns: Run[] | null;
}

export interface FileNode {
  name: string;
  path: string;
  type: "dir" | "file";
  size?: number;
  children?: FileNode[];
}

export interface RunnerCapacity {
  id: string;
  name: string;
  online: boolean;
  builtin: boolean;
  tags: string[];
  active: number;
  maxConcurrent: number;
}

export interface ClusterInfo {
  leader: boolean;
  runners: RunnerCapacity[];
  queued: Run[];
  running: Run[];
  counts: { queued: number; running: number; awaiting: number; runnersOnline: number };
}

export interface SyslogConfig {
  enabled: boolean;
  address: string;
  protocol: string; // udp | tcp
  tag: string;
  httpEnabled: boolean;
  httpUrl: string;
  httpToken: string;
  httpFormat: string; // splunk | elasticsearch | generic
}

export interface SystemInfo {
  system: { version: string; goVersion: string; platform: string };
  ansible?: string;
  database: { dialect: string };
  auth: Record<string, boolean>;
  notifications: { type: string; configured: boolean }[];
  cluster: { highAvailability: boolean };
  runners: { remoteRunners: boolean; total: number; online: number };
  taskSettings: { maxDurationSec: number };
  featureFlags: Record<string, boolean>;
}

export type WorkflowCondition = "always" | "on_success" | "on_failure";

export interface WorkflowStep {
  name: string;
  templateId: string;
  condition: WorkflowCondition;
  when?: string;
  inventoryId?: string;
  environmentId?: string;
  vaultCredentialId?: string;
  parallel?: boolean;
}

export interface Workflow {
  id: string;
  projectId: string;
  name: string;
  description: string;
  steps: WorkflowStep[];
  variables?: Record<string, string>;
  createdAt: string;
  updatedAt: string;
}

export interface WorkflowVersion {
  id: string;
  workflowId: string;
  version: number;
  name: string;
  description: string;
  steps: WorkflowStep[];
  variables?: Record<string, string>;
  actor?: string;
  createdAt: string;
}

export interface WorkflowRunStep {
  name: string;
  templateId: string;
  condition: WorkflowCondition;
  when?: string;
  inventoryId?: string;
  environmentId?: string;
  vaultCredentialId?: string;
  parallel?: boolean;
  runId?: string;
  status: string; // pending | running | success | failed | skipped
}

export interface WorkflowRun {
  id: string;
  workflowId: string;
  projectId: string;
  name: string;
  triggeredBy?: string;
  status: string; // running | success | failed | canceled
  steps: WorkflowRunStep[];
  createdAt: string;
  startedAt?: string;
  finishedAt?: string;
  workflowName?: string;
  projectName?: string;
}

export interface DayBucket {
  date: string;
  success: number;
  failed: number;
  other: number;
  total: number;
}

export interface TemplateInsight {
  templateId: string;
  templateName: string;
  runs: number;
  successRate: number;
  avgDurationSec: number;
}
export interface Insights {
  days: number;
  perDay: DayBucket[];
  total: number;
  successRate: number;
  avgDurationSec: number;
  avgWaitSec: number;
  byTemplate: TemplateInsight[];
}

export interface RunArtifact {
  id: string;
  runId: string;
  name: string;
  size: number;
  createdAt: string;
}

export interface PushedBranch {
  id: string;
  projectId: string;
  branch: string;
  commit: string;
  prUrl?: string;
  files: string[];
  actor: string;
  createdAt: string;
}

export interface RequirementItem {
  kind: "collection" | "role";
  name: string;
  version?: string;
  source: "galaxy" | "git" | "local";
  url?: string; // external link (galaxy/git)
  path?: string; // project-relative file to open in the editor
}

export interface Requirements {
  files: string[];
  items: RequirementItem[];
}

/** The browser's IANA timezone, sent with every run so timestamps are local. */
export function browserTimezone(): string {
  try {
    return localStorage.getItem("aui.tz") || Intl.DateTimeFormat().resolvedOptions().timeZone || "";
  } catch {
    return "";
  }
}

export interface LaunchRequest {
  projectId: string;
  app?: AppId;
  action?: string;
  timezone?: string;
  playbook: string;
  inventoryId?: string;
  name?: string;
  limit?: string;
  tags?: string;
  skipTags?: string;
  extraVars?: Record<string, unknown>;
  check?: boolean;
  diff?: boolean;
  verbosity?: number;
  cliArgs?: string[];
  branch?: string;
  vaultCredentialId?: string;
  vaults?: string[];
  environmentId?: string;
}

/** The active project id (multi-tenant scope), sent with every request. */
export function currentProjectId(): string {
  try {
    return localStorage.getItem("aui.project") || "";
  } catch {
    return "";
  }
}

async function http<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = {};
  if (body !== undefined) headers["Content-Type"] = "application/json";
  const proj = currentProjectId();
  if (proj) headers["X-Project-Id"] = proj;
  const res = await fetch(path, {
    method,
    credentials: "same-origin",
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  if (res.status === 401 && !path.startsWith("/api/auth/")) {
    window.dispatchEvent(new Event("auth:unauthorized"));
  }
  if (!res.ok) {
    let message = `${res.status} ${res.statusText}`;
    try {
      const data = await res.json();
      if (data?.error) message = data.error;
    } catch {
      /* non-json error body */
    }
    throw new Error(message);
  }
  if (res.status === 204) return undefined as T;
  const ct = res.headers.get("content-type") || "";
  if (ct.includes("application/json")) return res.json() as Promise<T>;
  return (await res.text()) as unknown as T;
}

export const api = {
  stats: () => http<DashboardStats>("GET", "/api/stats"),

  // auth
  authStatus: () => http<AuthStatus>("GET", "/api/auth/status"),
  me: () => http<User>("GET", "/api/auth/me"),
  login: (username: string, password: string, code?: string) =>
    http<User | { twoFactorRequired: true } | { emailOtpRequired: true }>("POST", "/api/auth/login", { username, password, code }),
  logout: () => http<{ ok: boolean }>("POST", "/api/auth/logout", {}),
  twoFactorSetup: () => http<{ secret: string; otpauthUrl: string; qrDataUri?: string }>("POST", "/api/auth/2fa/setup", {}),
  twoFactorEnable: (code: string) => http<{ enabled: boolean }>("POST", "/api/auth/2fa/enable", { code }),
  twoFactorDisable: (code: string) => http<{ enabled: boolean }>("POST", "/api/auth/2fa/disable", { code }),
  resetUser2FA: (id: string) => http<{ enabled: boolean }>("POST", `/api/users/${id}/2fa/reset`, {}),
  enableEmailOTP: () => http<{ emailOtpEnabled: boolean }>("POST", "/api/auth/2fa/email/enable", {}),
  disableEmailOTP: () => http<{ emailOtpEnabled: boolean }>("POST", "/api/auth/2fa/email/disable", {}),
  getSMTP: () => http<SMTPConfig>("GET", "/api/settings/smtp"),
  setSMTP: (b: SMTPConfig) => http<SMTPConfig>("PUT", "/api/settings/smtp", b),
  getGalaxy: () => http<GalaxyConfig>("GET", "/api/settings/galaxy"),
  setGalaxy: (b: GalaxyConfig) => http<GalaxyConfig>("PUT", "/api/settings/galaxy", b),
  getNotifyProxy: () => http<{ proxyUrl: string }>("GET", "/api/settings/notify-proxy"),
  setNotifyProxy: (proxyUrl: string) => http<{ proxyUrl: string }>("PUT", "/api/settings/notify-proxy", { proxyUrl }),
  getAuthMapping: () => http<AuthMapping>("GET", "/api/settings/auth-mapping"),
  setAuthMapping: (b: AuthMapping) => http<AuthMapping>("PUT", "/api/settings/auth-mapping", b),
  setup: (b: { username: string; email?: string; password: string }) =>
    http<User>("POST", "/api/auth/setup", b),
  listUsers: () => http<User[]>("GET", "/api/users"),
  createUser: (b: { username: string; email?: string; password: string; role?: string }) =>
    http<User>("POST", "/api/users", b),
  deleteUser: (id: string) => http<void>("DELETE", `/api/users/${id}`),
  listTokens: () => http<APIToken[]>("GET", "/api/tokens"),
  createToken: (b: { name: string; expiresDays?: number }) => http<APIToken>("POST", "/api/tokens", b),
  deleteToken: (id: string) => http<void>("DELETE", `/api/tokens/${id}`),

  // key store
  listCredentials: () => http<Credential[]>("GET", "/api/credentials"),
  createCredential: (b: Record<string, unknown>) => http<Credential>("POST", "/api/credentials", b),
  updateCredential: (id: string, b: Record<string, unknown>) =>
    http<Credential>("PUT", `/api/credentials/${id}`, b),
  deleteCredential: (id: string) => http<void>("DELETE", `/api/credentials/${id}`),
  credentialPubKey: (id: string) => http<{ publicKey: string; type: string }>("GET", `/api/credentials/${id}/pubkey`),

  // repositories
  listRepositories: () => http<Repository[]>("GET", "/api/repositories"),
  getRepository: (id: string) => http<Repository>("GET", `/api/repositories/${id}`),
  createRepository: (b: { name: string; gitUrl: string; branch?: string; credentialId?: string | null; cacheEnabled?: boolean }) =>
    http<Repository>("POST", "/api/repositories", b),
  updateRepository: (id: string, b: Partial<Repository>) =>
    http<Repository>("PUT", `/api/repositories/${id}`, b),
  deleteRepository: (id: string) => http<void>("DELETE", `/api/repositories/${id}`),
  syncRepository: (id: string) =>
    http<{ repository: Repository; commit?: string; message?: string; error?: string }>(
      "POST",
      `/api/repositories/${id}/sync`,
      {},
    ),
  repoTree: (id: string) => http<FileNode[]>("GET", `/api/repositories/${id}/tree`),

  listProjects: () => http<Project[]>("GET", "/api/projects"),
  getProject: (id: string) => http<Project>("GET", `/api/projects/${id}`),
  createProject: (b: {
    name: string;
    description?: string;
    slug?: string;
    repositoryId?: string;
    subPath?: string;
  }) => http<Project>("POST", "/api/projects", b),
  updateProject: (id: string, b: { name?: string; description?: string; slug?: string }) =>
    http<Project>("PUT", `/api/projects/${id}`, b),
  deleteProject: (id: string) => http<void>("DELETE", `/api/projects/${id}`),
  exportProject: (id: string) => http<Record<string, unknown>>("GET", `/api/projects/${id}/export`),
  importProject: (bundle: unknown) => http<Project>("POST", "/api/projects/import", bundle),
  listMembers: (id: string) => http<ProjectMember[]>("GET", `/api/projects/${id}/members`),
  setMember: (id: string, userId: string, role: ProjectRole | string) =>
    http<{ ok: boolean }>("POST", `/api/projects/${id}/members`, { userId, role }),
  removeMember: (id: string, userId: string) =>
    http<{ ok: boolean }>("DELETE", `/api/projects/${id}/members/${userId}`),
  projectFiles: (id: string) => http<FileNode[]>("GET", `/api/projects/${id}/files`),
  projectRequirements: (id: string) => http<Requirements>("GET", `/api/projects/${id}/requirements`),
  readFile: (id: string, path: string) =>
    http<{ path: string; content: string; size: number }>(
      "GET",
      `/api/projects/${id}/file?path=${encodeURIComponent(path)}`,
    ),
  writeFile: (id: string, path: string, content: string) =>
    http<{ saved: boolean }>("PUT", `/api/projects/${id}/file`, { path, content }),
  gitCommit: (id: string, body: { files: { path: string; content: string }[]; message: string; branch: string }) =>
    http<{ branch: string; commit: string; pushed: boolean; prUrl: string }>("POST", `/api/projects/${id}/git-commit`, body),
  listBranches: (id: string) => http<PushedBranch[]>("GET", `/api/projects/${id}/branches`),
  deleteFile: (id: string, path: string) =>
    http<{ deleted: boolean }>("DELETE", `/api/projects/${id}/file?path=${encodeURIComponent(path)}`),
  renameFile: (id: string, from: string, to: string) =>
    http<{ path: string; renamed: boolean }>("POST", `/api/projects/${id}/rename`, { from, to }),
  // Upload a .tar.gz / .tgz / .tar / .zip archive into a local project (multipart).
  // For password-protected zips the server replies { encrypted: true }; the caller
  // re-invokes with the password. The thrown Error carries `.encrypted` so the UI
  // knows to prompt.
  uploadArchive: async (id: string, file: File, password?: string): Promise<{ extracted: number }> => {
    const fd = new FormData();
    fd.append("file", file);
    if (password) fd.append("password", password);
    const headers: Record<string, string> = {};
    const proj = currentProjectId();
    if (proj) headers["X-Project-Id"] = proj;
    const res = await fetch(`/api/projects/${id}/upload`, { method: "POST", credentials: "same-origin", headers, body: fd });
    if (!res.ok) {
      let message = `${res.status} ${res.statusText}`;
      let encrypted = false;
      try { const d = await res.json(); if (d?.error) message = d.error; encrypted = !!d?.encrypted; } catch { /* */ }
      throw Object.assign(new Error(message), { encrypted });
    }
    return res.json();
  },

  listInventories: (projectId: string) =>
    http<Inventory[]>("GET", `/api/projects/${projectId}/inventories`),
  createInventory: (projectId: string, b: { name: string; type?: string; content: string }) =>
    http<Inventory>("POST", `/api/projects/${projectId}/inventories`, b),
  updateInventory: (id: string, b: { name?: string; type?: string; content?: string }) =>
    http<Inventory>("PUT", `/api/inventories/${id}`, b),
  deleteInventory: (id: string) => http<void>("DELETE", `/api/inventories/${id}`),
  inventoryHosts: (id: string) => http<InventoryHosts>("GET", `/api/inventories/${id}/hosts`),
  gatherFacts: (id: string, pattern?: string) =>
    http<{ gathered: string[]; count: number }>("POST", `/api/inventories/${id}/facts/gather${pattern ? `?pattern=${encodeURIComponent(pattern)}` : ""}`, {}),
  enableMonitor: (id: string, threshold?: number) =>
    http<{ monitored: number }>("POST", `/api/inventories/${id}/monitor${threshold ? `?threshold=${threshold}` : ""}`, {}),
  disableMonitor: (id: string) => http<{ removed: number }>("DELETE", `/api/inventories/${id}/monitor`),
  listMonitors: () => http<HostMonitor[]>("GET", "/api/monitors"),
  checkMonitors: () => http<{ checked: boolean }>("POST", "/api/monitors/check", {}),
  listHosts: () => http<HostFacts[]>("GET", "/api/hosts"),
  getHost: (host: string) => http<HostFacts>("GET", `/api/hosts/${encodeURIComponent(host)}`),
  deleteHost: (host: string) => http<void>("DELETE", `/api/hosts/${encodeURIComponent(host)}`),

  // schedules
  listSchedules: (templateId?: string) =>
    http<Schedule[]>("GET", `/api/schedules${templateId ? `?templateId=${templateId}` : ""}`),
  createSchedule: (b: {
    templateId?: string;
    workflowId?: string;
    name?: string;
    cron?: string;
    once?: boolean;
    runAt?: string;
    active?: boolean;
  }) => http<Schedule>("POST", "/api/schedules", b),
  updateSchedule: (id: string, b: Partial<Schedule>) =>
    http<Schedule>("PUT", `/api/schedules/${id}`, b),
  deleteSchedule: (id: string) => http<void>("DELETE", `/api/schedules/${id}`),

  // integrations (webhooks)
  listIntegrations: (templateId?: string) =>
    http<Integration[]>("GET", `/api/integrations${templateId ? `?templateId=${templateId}` : ""}`),
  createIntegration: (b: {
    templateId?: string;
    workflowId?: string;
    name?: string;
    authMethod?: IntegrationAuth;
    authHeader?: string;
    authSecret?: string;
    passPayload?: boolean;
    aliases?: string[];
  }) => http<Integration>("POST", "/api/integrations", b),
  updateIntegration: (
    id: string,
    b: Partial<{
      name: string;
      active: boolean;
      authMethod: IntegrationAuth;
      authHeader: string;
      authSecret: string;
      passPayload: boolean;
      aliases: string[];
    }>,
  ) => http<Integration>("PUT", `/api/integrations/${id}`, b),
  deleteIntegration: (id: string) => http<void>("DELETE", `/api/integrations/${id}`),

  // activity feed
  listActivity: (limit = 100) => http<Activity[]>("GET", `/api/activity?limit=${limit}`),

  // applications (execution backends)
  listApplications: () => http<Application[]>("GET", "/api/applications"),
  createApplication: (b: Partial<Application>) => http<Application>("POST", "/api/applications", b),
  updateApplication: (id: string, b: Partial<Application>) => http<Application>("PUT", `/api/applications/${id}`, b),
  deleteApplication: (id: string) => http<void>("DELETE", `/api/applications/${id}`),

  // API Explorer
  apiDocs: () => http<ApiDocs>("GET", "/api/meta/endpoints"),
  setDocsEnabled: (enabled: boolean) => http<{ docsEnabled: boolean }>("PUT", "/api/meta/docs", { enabled }),

  // notification channels
  getInsights: (params?: { projectId?: string; days?: number }) =>
    http<Insights>(
      "GET",
      `/api/insights?days=${params?.days ?? 14}${params?.projectId ? `&projectId=${encodeURIComponent(params.projectId)}` : ""}`,
    ),
  getSystemInfo: () => http<SystemInfo>("GET", "/api/system"),
  getCluster: () => http<ClusterInfo>("GET", "/api/cluster"),
  getTaskSettings: () => http<{ maxDurationSec: number; longRunAlertSec: number; maxParallel: number }>("GET", "/api/settings/tasks"),
  setTaskSettings: (b: { maxDurationSec: number; longRunAlertSec: number; maxParallel: number }) =>
    http<{ maxDurationSec: number; longRunAlertSec: number; maxParallel: number }>("PUT", "/api/settings/tasks", b),
  getSyslog: () => http<SyslogConfig>("GET", "/api/settings/syslog"),
  setSyslog: (b: SyslogConfig) => http<SyslogConfig>("PUT", "/api/settings/syslog", b),
  getFlags: () => http<{ nonadminCreateProject: boolean }>("GET", "/api/settings/flags"),
  setFlags: (b: { nonadminCreateProject: boolean }) =>
    http<{ nonadminCreateProject: boolean }>("PUT", "/api/settings/flags", b),
  getRetention: () => http<{ runsDays: number; auditDays: number }>("GET", "/api/settings/retention"),
  setRetention: (b: { runsDays: number; auditDays: number }) =>
    http<{ runsDays: number; auditDays: number }>("PUT", "/api/settings/retention", b),
  auditExportURL: () => "/api/audit/export",
  complianceReportURL: () => "/api/compliance/report",
  runsExportURL: (projectId?: string) =>
    `/api/runs/export${projectId ? `?projectId=${encodeURIComponent(projectId)}` : ""}`,
  listNotifications: () => http<NotificationChannel[]>("GET", "/api/notifications"),
  createNotification: (b: Partial<NotificationChannel>) =>
    http<NotificationChannel>("POST", "/api/notifications", b),
  updateNotification: (id: string, b: Partial<NotificationChannel>) =>
    http<NotificationChannel>("PUT", `/api/notifications/${id}`, b),
  deleteNotification: (id: string) => http<void>("DELETE", `/api/notifications/${id}`),
  testNotification: (id: string) => http<{ sent: boolean }>("POST", `/api/notifications/${id}/test`),
  listNotificationLogs: () => http<NotificationLog[]>("GET", "/api/notification-logs"),
  listRoles: () => http<CustomRole[]>("GET", "/api/roles"),
  createRole: (b: { name: string; description?: string; permissions: string[] }) => http<CustomRole>("POST", "/api/roles", b),
  updateRole: (id: string, b: Partial<CustomRole>) => http<CustomRole>("PUT", `/api/roles/${id}`, b),
  deleteRole: (id: string) => http<void>("DELETE", `/api/roles/${id}`),
  listSecretAccessLogs: () => http<SecretAccessLog[]>("GET", "/api/secret-access-logs"),

  // environments
  listEnvironments: () => http<Environment[]>("GET", "/api/environments"),
  getEnvironment: (id: string) => http<Environment>("GET", `/api/environments/${id}`),
  createEnvironment: (b: Partial<Environment>) => http<Environment>("POST", "/api/environments", b),
  updateEnvironment: (id: string, b: Partial<Environment>) =>
    http<Environment>("PUT", `/api/environments/${id}`, b),
  deleteEnvironment: (id: string) => http<void>("DELETE", `/api/environments/${id}`),

  // distributed runners
  listRunners: () => http<Runner[]>("GET", "/api/runners"),
  deleteRunner: (id: string) => http<void>("DELETE", `/api/runners/${id}`),

  // external secret backends (Vault, …)
  listSecretBackends: () => http<SecretBackend[]>("GET", "/api/secret-backends"),
  createSecretBackend: (b: Partial<SecretBackend>) => http<SecretBackend>("POST", "/api/secret-backends", b),
  updateSecretBackend: (id: string, b: Partial<SecretBackend>) =>
    http<SecretBackend>("PUT", `/api/secret-backends/${id}`, b),
  deleteSecretBackend: (id: string) => http<void>("DELETE", `/api/secret-backends/${id}`),
  testSecretBackend: (b: Partial<SecretBackend>) =>
    http<{ ok: boolean; message: string }>("POST", "/api/secret-backends/test", b),

  listTemplates: (projectId?: string) =>
    http<Template[]>("GET", `/api/templates${projectId ? `?projectId=${projectId}` : ""}`),
  listAllTemplates: () => http<Template[]>("GET", "/api/templates/all"),
  getTemplate: (id: string) => http<Template>("GET", `/api/templates/${id}`),
  createTemplate: (b: Partial<Template>) => http<Template>("POST", "/api/templates", b),
  updateTemplate: (id: string, b: Partial<Template>) =>
    http<Template>("PUT", `/api/templates/${id}`, b),
  deleteTemplate: (id: string) => http<void>("DELETE", `/api/templates/${id}`),
  // template views (saved filter tabs)
  listViews: () => http<TemplateView[]>("GET", "/api/views"),
  createView: (b: Partial<TemplateView>) => http<TemplateView>("POST", "/api/views", b),
  updateView: (id: string, b: Partial<TemplateView>) => http<TemplateView>("PUT", `/api/views/${id}`, b),
  deleteView: (id: string) => http<void>("DELETE", `/api/views/${id}`),

  runTemplate: (id: string, overrides?: Partial<LaunchRequest>) =>
    http<Run>("POST", `/api/templates/${id}/run`, { timezone: browserTimezone(), ...(overrides ?? {}) }),

  listWorkflows: (projectId: string) => http<Workflow[]>("GET", `/api/projects/${projectId}/workflows`),
  listAllWorkflows: () => http<Workflow[]>("GET", "/api/workflows"),
  createWorkflow: (projectId: string, b: Partial<Workflow>) =>
    http<Workflow>("POST", `/api/projects/${projectId}/workflows`, b),
  getWorkflow: (id: string) => http<Workflow>("GET", `/api/workflows/${id}`),
  updateWorkflow: (id: string, b: Partial<Workflow>) => http<Workflow>("PUT", `/api/workflows/${id}`, b),
  listWorkflowVersions: (id: string) => http<WorkflowVersion[]>("GET", `/api/workflows/${id}/versions`),
  rollbackWorkflow: (id: string, versionId: string) => http<Workflow>("POST", `/api/workflows/${id}/rollback/${versionId}`, {}),
  deleteWorkflow: (id: string) => http<{ deleted: boolean }>("DELETE", `/api/workflows/${id}`),
  runWorkflow: (id: string, variables?: Record<string, unknown>) =>
    http<WorkflowRun>("POST", `/api/workflows/${id}/run`, variables ? { variables } : {}),
  listWorkflowRuns: (params?: { projectId?: string; workflowId?: string }) => {
    const q = new URLSearchParams();
    if (params?.projectId) q.set("projectId", params.projectId);
    if (params?.workflowId) q.set("workflowId", params.workflowId);
    const qs = q.toString();
    return http<WorkflowRun[]>("GET", `/api/workflow-runs${qs ? `?${qs}` : ""}`);
  },
  getWorkflowRun: (id: string) => http<WorkflowRun>("GET", `/api/workflow-runs/${id}`),

  listRuns: (params?: { projectId?: string; status?: string; limit?: number }) => {
    const q = new URLSearchParams();
    if (params?.projectId) q.set("projectId", params.projectId);
    if (params?.status) q.set("status", params.status);
    if (params?.limit) q.set("limit", String(params.limit));
    const qs = q.toString();
    return http<Run[]>("GET", `/api/runs${qs ? `?${qs}` : ""}`);
  },
  getRun: (id: string) => http<{ run: Run; active: boolean }>("GET", `/api/runs/${id}`),
  runSecrets: (id: string) => http<Record<string, unknown>>("GET", `/api/runs/${id}/secrets`),
  createRun: (b: LaunchRequest) => http<Run>("POST", "/api/runs", { timezone: browserTimezone(), ...b }),
  cancelRun: (id: string) => http<{ status: string }>("POST", `/api/runs/${id}/cancel`),
  approveRun: (id: string) => http<{ status: string }>("POST", `/api/runs/${id}/approve`),
  rejectRun: (id: string) => http<{ status: string }>("POST", `/api/runs/${id}/reject`),
  deleteRun: (id: string) => http<{ deleted: number }>("DELETE", `/api/runs/${id}`),
  clearRuns: (projectId?: string) =>
    http<{ deleted: number }>("DELETE", `/api/runs${projectId ? `?projectId=${encodeURIComponent(projectId)}` : ""}`),
  runOutputURL: (id: string) => `/api/runs/${id}/output`,
  runLog: (id: string) => http<{ output: string; times: number[] }>("GET", `/api/runs/${id}/log`),
  listArtifacts: (id: string) => http<RunArtifact[]>("GET", `/api/runs/${id}/artifacts`),
  artifactURL: (runId: string, artifactId: string) => `/api/runs/${runId}/artifacts/${artifactId}`,
};

export function wsURL(path: string): string {
  const proto = location.protocol === "https:" ? "wss:" : "ws:";
  return `${proto}//${location.host}${path}`;
}
