// The pristine demo dataset — rich, interlinked history across every feature, so a
// visitor can feel the whole product. Rebuilt verbatim by "Reset Demo".
import type { Run, RunStats } from "../api";
import type { DemoDB } from "./db";
import { LOGS } from "./runlogs";

const iso = (ms: number) => new Date(ms).toISOString();
const D = 86_400_000, H = 3_600_000, M = 60_000;
const ago = (ms: number) => iso(Date.now() - ms);

export function buildSeed(): DemoDB {
  const runLogs: Record<string, string> = {};
  const runs: Run[] = [];

  const zeroStats = { hosts: 0, ok: 0, changed: 0, unreachable: 0, failed: 0, skipped: 0, rescued: 0, ignored: 0 };
  const mkRun = (r: Omit<Partial<Run>, "stats"> & { id: string; name: string; log?: keyof typeof LOGS; stats?: Partial<RunStats> }): Run => {
    const created = r.createdAt ?? ago(2 * H);
    const run: Run = {
      id: r.id, projectId: r.projectId ?? "prj_prod", templateId: r.templateId ?? null,
      name: r.name, triggeredBy: r.triggeredBy ?? "demo", app: r.app ?? "ansible", action: r.action,
      commit: r.commit, version: r.version, runnerId: r.runnerId ?? "runner_builtin",
      runnerName: r.runnerName ?? "builtin", playbook: r.playbook ?? "deploy.yml",
      status: r.status ?? "success", exitCode: r.exitCode ?? (r.status === "failed" ? 2 : 0),
      limit: r.limit ?? "", tags: r.tags ?? "", skipTags: r.skipTags ?? "",
      extraVars: r.extraVars ?? {}, check: r.check ?? false, diff: r.diff ?? false,
      verbosity: r.verbosity ?? 0, cliArgs: r.cliArgs, args: r.args ?? [],
      stats: { ...zeroStats, ...(r.stats ?? {}) }, createdAt: created,
      startedAt: r.startedAt ?? created, finishedAt: r.finishedAt ?? (r.status === "running" || r.status === "awaiting" || r.status === "queued" ? null : iso(new Date(created).getTime() + 90 * 1000)),
      projectName: r.projectName,
    };
    if (r.log) runLogs[run.id] = LOGS[r.log]();
    runs.push(run);
    return run;
  };

  // ---- runs: spread across ~14 days, varied status / app / trigger ----
  mkRun({ id: "run_live1", name: "Deploy web tier", templateId: "tpl_deploy", playbook: "deploy.yml", status: "running", triggeredBy: "demo", commit: "a1b2c3d", version: "#482", runnerId: "runner_eu", runnerName: "eu-west-1", verbosity: 2, log: "deploy", createdAt: ago(40 * 1000), stats: { hosts: 2, ok: 4, changed: 2 } });
  mkRun({ id: "run_wait1", name: "DB migration (prod)", templateId: "tpl_migrate", playbook: "migrate.yml", status: "awaiting", triggeredBy: "alice", createdAt: ago(6 * M), stats: zeroStats });
  mkRun({ id: "run_queue1", name: "Nightly facts", templateId: "tpl_facts", playbook: "facts.yml", status: "queued", triggeredBy: "schedule", createdAt: ago(2 * M), stats: zeroStats });
  mkRun({ id: "run_d1", name: "Deploy web tier", templateId: "tpl_deploy", playbook: "deploy.yml", status: "success", triggeredBy: "demo", commit: "f00ba12", version: "#481", verbosity: 2, log: "deploy", createdAt: ago(3 * H), stats: { hosts: 2, ok: 8, changed: 3 } });
  mkRun({ id: "run_d2", name: "Deploy web tier", templateId: "tpl_deploy", playbook: "deploy.yml", status: "failed", triggeredBy: "webhook", commit: "bad1dea", projectId: "prj_staging", projectName: "Staging", log: "failed", createdAt: ago(7 * H), stats: { hosts: 1, ok: 2, changed: 1, failed: 1 } });
  mkRun({ id: "run_f1", name: "Gather & assert facts", templateId: "tpl_facts", playbook: "facts.yml", status: "success", triggeredBy: "schedule", log: "facts", createdAt: ago(1 * D), stats: { hosts: 3, ok: 5, changed: 1 } });
  mkRun({ id: "run_tf1", name: "Terraform apply (vpc)", templateId: "tpl_tf_apply", app: "terraform", action: "apply", playbook: "main.tf", status: "success", triggeredBy: "carol", log: "terraform", createdAt: ago(1 * D + 3 * H), stats: { hosts: 1, ok: 3, changed: 3 } });
  mkRun({ id: "run_bash1", name: "Fleet healthcheck", templateId: "tpl_health", app: "bash", action: "script", playbook: "healthcheck.sh", status: "success", triggeredBy: "demo", log: "bash", createdAt: ago(2 * D), stats: { hosts: 4, ok: 4 } });
  mkRun({ id: "run_cancel1", name: "Deploy web tier", templateId: "tpl_deploy", playbook: "deploy.yml", status: "canceled", triggeredBy: "bob", createdAt: ago(2 * D + 5 * H), stats: { hosts: 2, ok: 3 } });
  // a tail of older successful runs to fill the insights charts
  for (let i = 0; i < 16; i++) {
    const fail = i % 6 === 4;
    mkRun({
      id: `run_h${i}`, name: i % 2 ? "Deploy web tier" : "Gather & assert facts",
      templateId: i % 2 ? "tpl_deploy" : "tpl_facts", playbook: i % 2 ? "deploy.yml" : "facts.yml",
      status: fail ? "failed" : "success", triggeredBy: ["demo", "alice", "schedule", "webhook", "carol"][i % 5],
      log: fail ? "failed" : i % 2 ? "deploy" : "facts", verbosity: i % 2 ? 2 : 0,
      createdAt: ago((3 + i) * D + (i % 5) * H),
      stats: fail ? { hosts: 2, ok: 4, changed: 1, failed: 1 } : { hosts: 2, ok: 8, changed: i % 3 },
    });
  }

  return {
    users: [
      { id: "usr_demo", username: "demo", email: "demo@ansible-ui.dev", role: "admin", twoFactorEnabled: true, createdAt: ago(60 * D) },
      { id: "usr_alice", username: "alice", email: "alice@acme.io", role: "user", twoFactorEnabled: true, createdAt: ago(45 * D) },
      { id: "usr_bob", username: "bob", email: "bob@acme.io", role: "user", createdAt: ago(30 * D) },
      { id: "usr_carol", username: "carol", email: "carol@acme.io", role: "admin", createdAt: ago(50 * D) },
    ],
    tokens: [
      { id: "tok_ci", userId: "usr_demo", name: "ci-pipeline", prefix: "aui_7f3a", lastUsedAt: ago(2 * H), expiresAt: iso(Date.now() + 60 * D), createdAt: ago(20 * D) },
      { id: "tok_bot", userId: "usr_demo", name: "release-bot", prefix: "aui_b21c", lastUsedAt: ago(3 * D), createdAt: ago(40 * D) },
    ],
    projects: [
      { id: "prj_prod", slug: "production", name: "Production", description: "Customer-facing web tier + database.", path: "/data/projects/production", sourceType: "git", gitUrl: "https://github.com/acme/infra.git", gitBranch: "main", repositoryId: "repo_app", repositoryName: "acme/infra", subPath: "ansible", createdAt: ago(58 * D), updatedAt: ago(3 * H), myRole: "admin" },
      { id: "prj_staging", slug: "staging", name: "Staging", description: "Pre-prod mirror, auto-deployed on every push.", path: "/data/projects/staging", sourceType: "git", gitUrl: "https://github.com/acme/infra.git", gitBranch: "develop", repositoryId: "repo_app", repositoryName: "acme/infra", subPath: "ansible", createdAt: ago(40 * D), updatedAt: ago(7 * H), myRole: "admin" },
      { id: "prj_demo", slug: "demo", name: "Demo · Ansible Examples", description: "Starter playbooks showcasing loops, roles, blocks & full-stack.", path: "/data/projects/demo", sourceType: "local", createdAt: ago(60 * D), updatedAt: ago(1 * D), myRole: "admin" },
    ],
    repositories: [
      { id: "repo_app", name: "acme/infra", gitUrl: "https://github.com/acme/infra.git", branch: "main", credentialId: "cred_ssh", status: "ready", lastCommit: "a1b2c3d", lastSyncedAt: ago(3 * H), cacheEnabled: true, createdAt: ago(58 * D), updatedAt: ago(3 * H) },
      { id: "repo_roles", name: "acme/shared-roles", gitUrl: "git@gitlab.com:acme/shared-roles.git", branch: "main", credentialId: "cred_ssh", status: "ready", lastCommit: "9d8c7b6", lastSyncedAt: ago(1 * D), cacheEnabled: false, createdAt: ago(50 * D), updatedAt: ago(1 * D) },
    ],
    credentials: [
      { id: "cred_ssh", name: "deploy-key (ed25519)", type: "ssh", hasSecret: true, ownerUserId: null, createdAt: ago(58 * D), updatedAt: ago(58 * D) },
      { id: "cred_login", name: "rhel-sudo", type: "login_password", login: "ansible", hasSecret: true, createdAt: ago(40 * D), updatedAt: ago(40 * D) },
      { id: "cred_vault", name: "ansible-vault-pass", type: "vault", hasSecret: true, createdAt: ago(40 * D), updatedAt: ago(40 * D) },
      { id: "cred_gcp", name: "gcp-sa (compute)", type: "gcp", hasSecret: true, createdAt: ago(30 * D), updatedAt: ago(30 * D) },
      { id: "cred_azure", name: "azure-sp", type: "azure", login: "deployer", hasSecret: true, createdAt: ago(25 * D), updatedAt: ago(25 * D) },
    ],
    inventories: [
      { id: "inv_prod", projectId: "prj_prod", name: "production.ini", type: "static", content: "[web]\nweb-01.prod\nweb-02.prod\n\n[db]\ndb-01.prod\n\n[prod:children]\nweb\ndb\n", connCredentialIds: ["cred_ssh", "cred_login"], runnerTag: "eu", createdAt: ago(58 * D), updatedAt: ago(10 * D) },
      { id: "inv_aws", projectId: "prj_prod", name: "aws-ec2 (dynamic)", type: "cloud", provider: "aws_ec2", credentialId: "cred_gcp", region: "eu-west-1", content: "plugin: amazon.aws.aws_ec2\nregions: [eu-west-1]\nkeyed_groups:\n  - key: tags.Role\n", createdAt: ago(30 * D), updatedAt: ago(5 * D) },
      { id: "inv_staging", projectId: "prj_staging", name: "staging.ini", type: "static", content: "[web]\nweb-03.staging\n", connCredentialIds: ["cred_ssh"], createdAt: ago(40 * D), updatedAt: ago(8 * D) },
      { id: "inv_demo", projectId: "prj_demo", name: "localhost", type: "static", content: "localhost ansible_connection=local\n", createdAt: ago(60 * D), updatedAt: ago(60 * D) },
    ],
    templates: [
      { id: "tpl_deploy", projectId: "prj_prod", name: "Deploy web tier", description: "Rolling deploy with a health gate.", app: "ansible", playbook: "deploy.yml", inventoryId: "inv_prod", environmentId: "env_prod", surveyVars: [{ name: "version", title: "Release version", type: "text", required: true, default: "1.4.2" }, { name: "canary", title: "Canary %", type: "enum", required: false, default: "10", options: ["10", "25", "50", "100"] }], limit: "", tags: "deploy", skipTags: "", extraVars: { rolling: 1 }, check: false, diff: true, verbosity: 2, type: "deploy", runnerTag: "eu", prompts: { limit: true, tags: true, branch: true }, createdAt: ago(58 * D), updatedAt: ago(3 * H) },
      { id: "tpl_migrate", projectId: "prj_prod", name: "DB migration (prod)", description: "Schema migration — requires approval.", app: "ansible", playbook: "migrate.yml", inventoryId: "inv_prod", vaultCredentialId: "cred_vault", surveyVars: [], limit: "db", tags: "", skipTags: "", extraVars: {}, check: false, diff: false, verbosity: 0, requiresApproval: true, createdAt: ago(50 * D), updatedAt: ago(6 * M) },
      { id: "tpl_facts", projectId: "prj_prod", name: "Gather & assert facts", description: "Collect facts + assert baseline.", app: "ansible", playbook: "facts.yml", inventoryId: "inv_prod", surveyVars: [], limit: "", tags: "", skipTags: "", extraVars: {}, check: false, diff: false, verbosity: 0, createdAt: ago(55 * D), updatedAt: ago(1 * D) },
      { id: "tpl_roles", projectId: "prj_demo", name: "Compose roles", description: "common + webserver + monitoring roles.", app: "ansible", playbook: "playbooks/06-roles.yml", inventoryId: "inv_demo", surveyVars: [], limit: "", tags: "", skipTags: "", extraVars: {}, check: false, diff: true, verbosity: 2, createdAt: ago(60 * D), updatedAt: ago(1 * D) },
      { id: "tpl_tf_plan", projectId: "prj_prod", name: "Terraform plan (vpc)", description: "Plan the network module.", app: "terraform", action: "plan", playbook: "infra/vpc", inventoryId: null, surveyVars: [], limit: "", tags: "", skipTags: "", extraVars: {}, check: false, diff: false, verbosity: 0, workspace: "prod", tfBackend: "https://state.acme.io/tf", createdAt: ago(35 * D), updatedAt: ago(2 * D) },
      { id: "tpl_tf_apply", projectId: "prj_prod", name: "Terraform apply (vpc)", description: "Apply after a successful plan.", app: "terraform", action: "apply", playbook: "infra/vpc", inventoryId: null, surveyVars: [], limit: "", tags: "", skipTags: "", extraVars: {}, check: false, diff: false, verbosity: 0, workspace: "prod", autoApprove: true, type: "deploy", buildTemplateId: "tpl_tf_plan", createdAt: ago(35 * D), updatedAt: ago(1 * D) },
      { id: "tpl_health", projectId: "prj_prod", name: "Fleet healthcheck", description: "Bash probe across the fleet.", app: "bash", action: "script", playbook: "scripts/healthcheck.sh", inventoryId: "inv_prod", surveyVars: [], limit: "", tags: "", skipTags: "", extraVars: {}, check: false, diff: false, verbosity: 0, createdAt: ago(20 * D), updatedAt: ago(2 * D) },
      { id: "tpl_py", projectId: "prj_prod", name: "Reconcile DNS (python)", description: "Sync Route53 records.", app: "python", action: "script", playbook: "scripts/dns_sync.py", inventoryId: null, surveyVars: [], limit: "", tags: "", skipTags: "", extraVars: {}, check: false, diff: false, verbosity: 0, createdAt: ago(18 * D), updatedAt: ago(4 * D) },
      { id: "tpl_ps", projectId: "prj_prod", name: "Windows patch (pwsh)", description: "PowerShell patch run.", app: "powershell", action: "script", playbook: "scripts/patch.ps1", inventoryId: null, surveyVars: [], limit: "", tags: "", skipTags: "", extraVars: {}, check: false, diff: false, verbosity: 0, createdAt: ago(15 * D), updatedAt: ago(6 * D) },
      { id: "tpl_build", projectId: "prj_prod", name: "Build artifact", description: "Build step feeding the deploy.", app: "ansible", playbook: "build.yml", inventoryId: "inv_demo", surveyVars: [], limit: "", tags: "", skipTags: "", extraVars: {}, check: false, diff: false, verbosity: 0, type: "build", lastBuildVersion: "#482", createdAt: ago(40 * D), updatedAt: ago(3 * H) },
    ],
    runs,
    runLogs,
    schedules: [
      { id: "sch_nightly", templateId: "tpl_facts", name: "Nightly facts", cron: "0 2 * * *", once: false, active: true, lastRunAt: ago(20 * H), nextRunAt: iso(Date.now() + 4 * H), templateName: "Gather & assert facts", projectId: "prj_prod", createdAt: ago(55 * D), updatedAt: ago(20 * H) },
      { id: "sch_deploy", templateId: "tpl_deploy", name: "Weekday deploy window", cron: "0 9 * * 1-5", once: false, active: true, lastRunAt: ago(1 * D), nextRunAt: iso(Date.now() + 18 * H), templateName: "Deploy web tier", projectId: "prj_prod", createdAt: ago(30 * D), updatedAt: ago(1 * D) },
      { id: "sch_once", templateId: "tpl_migrate", name: "Maintenance migration", cron: "", once: true, active: true, nextRunAt: iso(Date.now() + 2 * D), templateName: "DB migration (prod)", projectId: "prj_prod", createdAt: ago(2 * D), updatedAt: ago(2 * D) },
    ],
    integrations: [
      { id: "int_gh", templateId: "tpl_deploy", name: "GitHub push → staging deploy", token: "whk_3f8a21", active: true, authMethod: "github", passPayload: true, hasSecret: true, aliases: ["deploy"], lastTriggeredAt: ago(7 * H), templateName: "Deploy web tier", projectId: "prj_prod", createdAt: ago(30 * D) },
      { id: "int_tok", templateId: "tpl_facts", name: "CI token webhook", token: "whk_b1c9d4", active: true, authMethod: "token", authHeader: "X-CI-Token", passPayload: false, hasSecret: true, lastTriggeredAt: ago(2 * D), templateName: "Gather & assert facts", projectId: "prj_prod", createdAt: ago(25 * D) },
      { id: "int_hmac", templateId: "tpl_health", name: "Alertmanager → healthcheck", token: "whk_77ee10", active: false, authMethod: "hmac", passPayload: true, hasSecret: true, templateName: "Fleet healthcheck", projectId: "prj_prod", createdAt: ago(10 * D) },
    ],
    environments: [
      { id: "env_prod", name: "production", description: "Prod extra-vars + secrets.", extraVars: { env: "prod", replicas: 3, feature_flags: { new_ui: true } }, envVars: { ANSIBLE_FORCE_COLOR: "1", TZ: "UTC" }, secrets: [{ name: "API_KEY", type: "env", hasValue: true }, { name: "db_password", type: "var", hasValue: true }], externalSecrets: [{ name: "STRIPE_KEY", backend: "Vault (prod)", path: "secret/data/stripe", field: "key", asVar: false }], createdAt: ago(50 * D), updatedAt: ago(5 * D) },
      { id: "env_staging", name: "staging", description: "Staging overrides.", extraVars: { env: "staging", replicas: 1 }, envVars: { TZ: "UTC" }, secrets: [{ name: "API_KEY", type: "env", hasValue: true }], createdAt: ago(40 * D), updatedAt: ago(8 * D) },
    ],
    workflows: [
      { id: "wf_release", projectId: "prj_prod", name: "Release pipeline", description: "Build → (Test ∥ Lint) → Deploy → Smoke.", variables: { version: "1.4.2", channel: "stable" }, steps: [
        { name: "Build", templateId: "tpl_build", condition: "always" },
        { name: "Test", templateId: "tpl_facts", condition: "on_success", parallel: true },
        { name: "Lint", templateId: "tpl_roles", condition: "on_success", parallel: true },
        { name: "Deploy", templateId: "tpl_deploy", condition: "on_success", inventoryId: "inv_prod", environmentId: "env_prod" },
        { name: "Smoke test", templateId: "tpl_health", condition: "on_success" },
      ], createdAt: ago(30 * D), updatedAt: ago(1 * D) },
    ],
    workflowVersions: [
      { id: "wfv_3", workflowId: "wf_release", version: 3, name: "Release pipeline", description: "Build → (Test ∥ Lint) → Deploy → Smoke.", steps: [], actor: "demo", createdAt: ago(1 * D) },
      { id: "wfv_2", workflowId: "wf_release", version: 2, name: "Release pipeline", description: "added smoke test", steps: [], actor: "alice", createdAt: ago(10 * D) },
      { id: "wfv_1", workflowId: "wf_release", version: 1, name: "Release pipeline", description: "initial", steps: [], actor: "carol", createdAt: ago(30 * D) },
    ],
    workflowRuns: [
      { id: "wfr_live", workflowId: "wf_release", projectId: "prj_prod", name: "Release pipeline", triggeredBy: "demo", status: "running", workflowName: "Release pipeline", projectName: "Production", createdAt: ago(3 * M), startedAt: ago(3 * M), steps: [
        { name: "Build", templateId: "tpl_build", condition: "always", status: "success", runId: "run_h1" },
        { name: "Test", templateId: "tpl_facts", condition: "on_success", parallel: true, status: "success", runId: "run_f1" },
        { name: "Lint", templateId: "tpl_roles", condition: "on_success", parallel: true, status: "running", runId: "run_live1" },
        { name: "Deploy", templateId: "tpl_deploy", condition: "on_success", status: "pending" },
        { name: "Smoke test", templateId: "tpl_health", condition: "on_success", status: "pending" },
      ] },
      { id: "wfr_ok", workflowId: "wf_release", projectId: "prj_prod", name: "Release pipeline", triggeredBy: "schedule", status: "success", workflowName: "Release pipeline", projectName: "Production", createdAt: ago(1 * D), startedAt: ago(1 * D), finishedAt: ago(1 * D - 5 * M), steps: [
        { name: "Build", templateId: "tpl_build", condition: "always", status: "success" },
        { name: "Test", templateId: "tpl_facts", condition: "on_success", parallel: true, status: "success" },
        { name: "Lint", templateId: "tpl_roles", condition: "on_success", parallel: true, status: "success" },
        { name: "Deploy", templateId: "tpl_deploy", condition: "on_success", status: "success" },
        { name: "Smoke test", templateId: "tpl_health", condition: "on_success", status: "success" },
      ] },
    ],
    runners: [
      { id: "runner_builtin", name: "builtin", url: "ws://localhost:8081", tags: [], builtin: true, status: "online", platform: "linux/amd64", version: "1.0.0", maxConcurrent: 4, lastSeenAt: ago(10 * 1000), createdAt: ago(60 * D), updatedAt: ago(10 * 1000) },
      { id: "runner_eu", name: "eu-west-1", url: "wss://runner-eu.acme.io", tags: ["eu", "prod"], builtin: false, status: "online", platform: "linux/amd64", version: "1.0.0", maxConcurrent: 8, lastSeenAt: ago(15 * 1000), createdAt: ago(20 * D), updatedAt: ago(15 * 1000) },
      { id: "runner_us", name: "us-east-1", url: "wss://runner-us.acme.io", tags: ["us"], builtin: false, status: "offline", platform: "linux/arm64", version: "1.0.0", maxConcurrent: 8, lastSeenAt: ago(2 * H), createdAt: ago(18 * D), updatedAt: ago(2 * H) },
    ],
    applications: [
      { id: "app_ansible", name: "Ansible", icon: "ansible", bin: "ansible-playbook", args: [], priority: 1, active: true, kind: "builtin", createdAt: ago(60 * D), updatedAt: ago(60 * D) },
      { id: "app_tf", name: "Terraform", icon: "terraform", bin: "terraform", args: [], priority: 2, active: true, kind: "builtin", createdAt: ago(60 * D), updatedAt: ago(60 * D) },
      { id: "app_tofu", name: "OpenTofu", icon: "opentofu", bin: "tofu", args: [], priority: 3, active: true, kind: "builtin", createdAt: ago(60 * D), updatedAt: ago(60 * D) },
      { id: "app_pulumi", name: "Pulumi", icon: "pulumi", bin: "pulumi", args: [], priority: 4, active: true, kind: "builtin", createdAt: ago(60 * D), updatedAt: ago(60 * D) },
      { id: "app_bash", name: "Bash", icon: "bash", bin: "bash", args: [], priority: 5, active: true, kind: "builtin", createdAt: ago(60 * D), updatedAt: ago(60 * D) },
      { id: "app_pwsh", name: "PowerShell", icon: "powershell", bin: "pwsh", args: [], priority: 6, active: true, kind: "builtin", createdAt: ago(60 * D), updatedAt: ago(60 * D) },
      { id: "app_py", name: "Python", icon: "python", bin: "python3", args: [], priority: 7, active: true, kind: "builtin", createdAt: ago(60 * D), updatedAt: ago(60 * D) },
      { id: "app_helm", name: "Helmfile", icon: "helm", bin: "helmfile", args: ["apply"], priority: 8, active: true, kind: "custom", createdAt: ago(12 * D), updatedAt: ago(12 * D) },
    ],
    activity: [
      { id: "act1", actor: "demo", action: "run.launch", target: "Deploy web tier", detail: "version=1.4.2", createdAt: ago(40 * 1000) },
      { id: "act2", actor: "alice", action: "run.launch", target: "DB migration (prod)", detail: "awaiting approval", createdAt: ago(6 * M) },
      { id: "act3", actor: "demo", action: "approval.request", target: "DB migration (prod)", detail: "", createdAt: ago(6 * M) },
      { id: "act4", actor: "webhook", action: "run.launch", target: "Deploy web tier", detail: "github push acme/infra@bad1dea", createdAt: ago(7 * H) },
      { id: "act5", actor: "carol", action: "template.update", target: "Terraform apply (vpc)", detail: "autoApprove=true", createdAt: ago(1 * D) },
      { id: "act6", actor: "demo", action: "credential.create", target: "azure-sp", detail: "type=azure", createdAt: ago(25 * D) },
      { id: "act7", actor: "alice", action: "login", target: "alice", detail: "via OIDC (okta)", createdAt: ago(2 * H) },
      { id: "act8", actor: "schedule", action: "run.launch", target: "Gather & assert facts", detail: "cron 0 2 * * *", createdAt: ago(20 * H) },
      { id: "act9", actor: "demo", action: "workflow.run", target: "Release pipeline", detail: "", createdAt: ago(3 * M) },
      { id: "act10", actor: "bob", action: "run.cancel", target: "Deploy web tier", detail: "", createdAt: ago(2 * D + 5 * H) },
      { id: "act11", actor: "carol", action: "user.create", target: "bob", detail: "role=user", createdAt: ago(30 * D) },
      { id: "act12", actor: "demo", action: "secret-backend.create", target: "Vault (prod)", detail: "type=vault", createdAt: ago(22 * D) },
    ],
    notifications: [
      { id: "ntf_slack", type: "slack", name: "#deploys", enabled: true, events: ["run.failed", "run.success"], config: { webhookUrl: "https://hooks.slack.com/services/…" }, createdAt: ago(40 * D), updatedAt: ago(2 * D) },
      { id: "ntf_tg", type: "telegram", name: "Ops bot", enabled: true, events: ["run.failed"], config: { chatId: "-100123" }, createdAt: ago(35 * D), updatedAt: ago(35 * D) },
      { id: "ntf_pd", type: "pagerduty", name: "On-call", enabled: true, events: ["run.failed"], config: { routingKey: "R0…" }, projectId: "prj_prod", createdAt: ago(20 * D), updatedAt: ago(20 * D) },
      { id: "ntf_email", type: "email", name: "Release digest", enabled: false, events: ["run.success", "run.failed"], config: { to: "ops@acme.io" }, createdAt: ago(15 * D), updatedAt: ago(15 * D) },
      { id: "ntf_discord", type: "discord", name: "#ci", enabled: true, events: ["run.failed"], config: { webhookUrl: "https://discord.com/api/webhooks/…" }, createdAt: ago(10 * D), updatedAt: ago(10 * D) },
    ],
    notificationLogs: [
      { id: "nl1", channelId: "ntf_slack", channelName: "#deploys", channelType: "slack", runId: "run_d2", runName: "Deploy web tier", projectId: "prj_staging", event: "run.failed", ok: true, createdAt: ago(7 * H) },
      { id: "nl2", channelId: "ntf_pd", channelName: "On-call", channelType: "pagerduty", runId: "run_d2", runName: "Deploy web tier", projectId: "prj_staging", event: "run.failed", ok: true, createdAt: ago(7 * H) },
      { id: "nl3", channelId: "ntf_tg", channelName: "Ops bot", channelType: "telegram", runId: "run_d2", runName: "Deploy web tier", projectId: "prj_staging", event: "run.failed", ok: false, error: "429 Too Many Requests", createdAt: ago(7 * H) },
      { id: "nl4", channelId: "ntf_slack", channelName: "#deploys", channelType: "slack", runId: "run_d1", runName: "Deploy web tier", projectId: "prj_prod", event: "run.success", ok: true, createdAt: ago(3 * H) },
    ],
    roles: [
      { id: "role_op", name: "Operator", description: "Run templates, no edit.", permissions: ["view", "run"], createdAt: ago(50 * D), updatedAt: ago(50 * D) },
      { id: "role_dev", name: "Developer", description: "Run + edit, no member management.", permissions: ["view", "run", "edit"], createdAt: ago(50 * D), updatedAt: ago(50 * D) },
    ],
    secretBackends: [
      { id: "sb_vault", name: "Vault (prod)", type: "vault", address: "https://vault.acme.io", mount: "secret", namespace: "platform", hasToken: true, createdAt: ago(22 * D), updatedAt: ago(22 * D) },
      { id: "sb_aws", name: "AWS Secrets Manager", type: "aws", address: "secretsmanager.eu-west-1.amazonaws.com", region: "eu-west-1", accessKeyId: "AKIA…", hasToken: true, createdAt: ago(18 * D), updatedAt: ago(18 * D) },
      { id: "sb_az", name: "Azure Key Vault", type: "azure", address: "https://acme-kv.vault.azure.net", tenantId: "72f9…", accessKeyId: "client-…", hasToken: true, createdAt: ago(12 * D), updatedAt: ago(12 * D) },
    ],
    secretAccessLogs: [
      { id: "sal1", actor: "demo", action: "run-secrets", credentialId: "cred_vault", credentialName: "ansible-vault-pass", detail: "run run_d1", createdAt: ago(3 * H) },
      { id: "sal2", actor: "eu-west-1", action: "pubkey", credentialId: "cred_ssh", credentialName: "deploy-key (ed25519)", detail: "fetched for git sync", createdAt: ago(3 * H) },
      { id: "sal3", actor: "alice", action: "credential-use", credentialId: "cred_login", credentialName: "rhel-sudo", detail: "become_password", createdAt: ago(1 * D) },
      { id: "sal4", actor: "demo", action: "run-secrets", credentialId: "cred_gcp", credentialName: "gcp-sa (compute)", detail: "aws_ec2 inventory", createdAt: ago(5 * D) },
    ],
    hosts: [
      { host: "web-01.prod", inventoryId: "inv_prod", os: "Linux", distro: "Ubuntu 22.04", ip: "10.0.1.11", kernel: "5.15.0-91", gatheredAt: ago(1 * D), facts: { ansible_processor_vcpus: 4, ansible_memtotal_mb: 7976, ansible_service_mgr: "systemd" } },
      { host: "web-02.prod", inventoryId: "inv_prod", os: "Linux", distro: "Ubuntu 22.04", ip: "10.0.1.12", kernel: "5.15.0-91", gatheredAt: ago(1 * D), facts: { ansible_processor_vcpus: 4, ansible_memtotal_mb: 7976 } },
      { host: "db-01.prod", inventoryId: "inv_prod", os: "Linux", distro: "Rocky 9.3", ip: "10.0.2.21", kernel: "5.14.0-362", gatheredAt: ago(1 * D), facts: { ansible_processor_vcpus: 8, ansible_memtotal_mb: 32072, postgres_version: "16.1" } },
      { host: "web-03.staging", inventoryId: "inv_staging", os: "Linux", distro: "Ubuntu 22.04", ip: "10.1.1.11", kernel: "5.15.0-89", gatheredAt: ago(2 * D), facts: { ansible_processor_vcpus: 2, ansible_memtotal_mb: 3936 } },
    ],
    monitors: [
      { host: "web-01.prod", inventoryId: "inv_prod", status: "up", consecFails: 0, threshold: 3, lastCheckedAt: ago(2 * M), lastUpAt: ago(2 * M), createdAt: ago(10 * D) },
      { host: "web-02.prod", inventoryId: "inv_prod", status: "up", consecFails: 0, threshold: 3, lastCheckedAt: ago(2 * M), lastUpAt: ago(2 * M), createdAt: ago(10 * D) },
      { host: "db-01.prod", inventoryId: "inv_prod", status: "down", consecFails: 2, threshold: 3, lastError: "ssh: connect timed out", lastCheckedAt: ago(2 * M), lastUpAt: ago(3 * H), createdAt: ago(10 * D) },
    ],
    views: [
      { id: "view_ans", name: "Ansible", app: "ansible", search: "", position: 0, createdAt: ago(20 * D), updatedAt: ago(20 * D) },
      { id: "view_infra", name: "Infra", app: "terraform", search: "", position: 1, createdAt: ago(20 * D), updatedAt: ago(20 * D) },
      { id: "view_deploy", name: "Deploys", app: "", search: "deploy", position: 2, createdAt: ago(15 * D), updatedAt: ago(15 * D) },
    ],
    members: [
      { projectId: "prj_prod", userId: "usr_demo", username: "demo", email: "demo@ansible-ui.dev", role: "admin", createdAt: ago(58 * D) },
      { projectId: "prj_prod", userId: "usr_alice", username: "alice", email: "alice@acme.io", role: "editor", createdAt: ago(45 * D) },
      { projectId: "prj_prod", userId: "usr_bob", username: "bob", email: "bob@acme.io", role: "viewer", createdAt: ago(30 * D) },
      { projectId: "prj_staging", userId: "usr_alice", username: "alice", email: "alice@acme.io", role: "admin", createdAt: ago(40 * D) },
    ],
    branches: [
      { id: "br1", projectId: "prj_prod", branch: "aui/edit-deploy-yml-1a2b", commit: "c0ffee1", prUrl: "https://github.com/acme/infra/pull/142", files: ["ansible/deploy.yml"], actor: "demo", createdAt: ago(2 * D) },
      { id: "br2", projectId: "prj_prod", branch: "aui/edit-vars-9f8e", commit: "dec0de2", prUrl: "https://github.com/acme/infra/pull/139", files: ["ansible/group_vars/all.yml"], actor: "alice", createdAt: ago(6 * D) },
    ],
    settings: {
      smtp: { host: "smtp.acme.io", port: "587", from: "ansible-ui@acme.io", username: "ansible-ui", password: "" },
      galaxy: { serverUrl: "https://galaxy.acme.io/api/", token: "", cliArgs: "" },
      notifyProxy: "",
      authMapping: { rules: [{ group: "platform-admins", role: "admin" }, { group: "developers", role: "user" }], ldapGroupAttr: "memberOf", oidcGroupsClaim: "groups", ldapDebug: false, oidcAutoLogin: false },
      syslog: { enabled: true, address: "splunk.acme.io:514", protocol: "tcp", tag: "ansible-ui", httpEnabled: true, httpUrl: "https://splunk.acme.io:8088/services/collector", httpToken: "", httpFormat: "splunk" },
      flags: { nonadminCreateProject: false },
      retention: { runsDays: 90, auditDays: 365 },
      tasks: { maxDurationSec: 3600, longRunAlertSec: 600, maxParallel: 8 },
      docsEnabled: true,
    },
  };
}
