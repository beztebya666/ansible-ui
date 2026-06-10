// In-browser database for the demo build. All state lives in the visitor's own
// localStorage — every browser is its own private sandbox, so no one can affect
// anyone else, and "Reset Demo" just rebuilds it from the seed.
import type {
  User, APIToken, Project, Repository, Credential, Inventory, Template, Run,
  Schedule, Integration, Environment, Workflow, WorkflowRun, WorkflowVersion,
  Runner, Application, Activity, NotificationChannel, NotificationLog, CustomRole,
  SecretBackend, SecretAccessLog, HostFacts, HostMonitor, TemplateView,
  ProjectMember, PushedBranch, SMTPConfig, GalaxyConfig, AuthMapping, SyslogConfig,
} from "../api";
import { buildSeed } from "./seed";

export interface DemoSettings {
  smtp: SMTPConfig;
  galaxy: GalaxyConfig;
  notifyProxy: string;
  authMapping: AuthMapping;
  syslog: SyslogConfig;
  flags: { nonadminCreateProject: boolean };
  retention: { runsDays: number; auditDays: number };
  tasks: { maxDurationSec: number; longRunAlertSec: number; maxParallel: number };
  docsEnabled: boolean;
}

export interface DemoDB {
  users: User[];
  tokens: APIToken[];
  projects: Project[];
  repositories: Repository[];
  credentials: Credential[];
  inventories: Inventory[];
  templates: Template[];
  runs: Run[];
  schedules: Schedule[];
  integrations: Integration[];
  environments: Environment[];
  workflows: Workflow[];
  workflowRuns: WorkflowRun[];
  workflowVersions: WorkflowVersion[];
  runners: Runner[];
  applications: Application[];
  activity: Activity[];
  notifications: NotificationChannel[];
  notificationLogs: NotificationLog[];
  roles: CustomRole[];
  secretBackends: SecretBackend[];
  secretAccessLogs: SecretAccessLog[];
  hosts: HostFacts[];
  monitors: HostMonitor[];
  views: TemplateView[];
  members: ProjectMember[];
  branches: PushedBranch[];
  runLogs: Record<string, string>; // runId -> full stored output
  settings: DemoSettings;
}

const KEY = "aui.demo.db.v2";
let db: DemoDB | null = null;

function load(): DemoDB {
  try {
    const raw = localStorage.getItem(KEY);
    if (raw) return JSON.parse(raw) as DemoDB;
  } catch { /* corrupt / unavailable */ }
  const fresh = buildSeed();
  try { localStorage.setItem(KEY, JSON.stringify(fresh)); } catch { /* ignore */ }
  return fresh;
}

export function getDB(): DemoDB {
  if (!db) db = load();
  return db;
}

export function saveDB() {
  try { localStorage.setItem(KEY, JSON.stringify(db)); } catch { /* quota / unavailable */ }
}

export function resetDemo() {
  db = buildSeed();
  saveDB();
}

// ---- tiny event bus driving the fake /ws/events socket ----
type Listener = (type: string) => void;
const listeners = new Set<Listener>();
export function onDemoEvent(l: Listener): () => void {
  listeners.add(l);
  return () => { listeners.delete(l); };
}
export function emitDemo(type: string) {
  for (const l of Array.from(listeners)) {
    try { l(type); } catch { /* ignore listener errors */ }
  }
}

// ---- helpers ----
let seq = Date.now();
export const genId = (p: string) => `${p}_${(seq++).toString(16)}${Math.random().toString(16).slice(2, 6)}`;
export const nowISO = () => new Date().toISOString();
export const minsAgo = (m: number) => new Date(Date.now() - m * 60_000).toISOString();
export const daysAgo = (d: number) => new Date(Date.now() - d * 86_400_000).toISOString();
