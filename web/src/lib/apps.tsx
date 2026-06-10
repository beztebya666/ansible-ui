// Central registry of execution backends ("apps"). One source of truth for the
// template form, launcher, cards and run views so a new app is added in one place.
import {
  Boxes,
  Box,
  Cloud,
  Code2,
  Layers,
  Server,
  SquareTerminal,
  TerminalSquare,
  type LucideIcon,
} from "lucide-react";
import type { AppId } from "./api";
import type { CodeLang } from "../components/CodeEditor";

export type AppKind = "ansible" | "infra" | "script";

export interface AppMeta {
  id: AppId;
  label: string; // full name
  short: string; // badge text
  icon: LucideIcon;
  /** tailwind classes for the badge (text + subtle bg/border) */
  badge: string;
  kind: AppKind;
  entryLabel: string; // label for the entrypoint field
  entryHint: string;
  entryPlaceholder: string;
  varsLabel: string; // how extra-vars are surfaced for this app
  varsHint: string;
  lang: CodeLang; // editor language for this app's primary files
}

export const APPS: Record<AppId, AppMeta> = {
  ansible: {
    id: "ansible",
    label: "Ansible",
    short: "Ansible",
    icon: Server,
    badge: "text-rose-300 bg-rose-500/10 border-rose-500/30",
    kind: "ansible",
    entryLabel: "Playbook",
    entryHint: "playbook in the project",
    entryPlaceholder: "playbooks/site.yml",
    varsLabel: "Extra variables",
    varsHint: "JSON object, or one key=value per line (--extra-vars)",
    lang: "yaml",
  },
  terraform: {
    id: "terraform",
    label: "Terraform",
    short: "Terraform",
    icon: Boxes,
    badge: "text-purple-300 bg-purple-500/10 border-purple-500/30",
    kind: "infra",
    entryLabel: "Working directory",
    entryHint: "dir with .tf files (relative to project, '.' = root)",
    entryPlaceholder: "terraform",
    varsLabel: "Variables",
    varsHint: "passed as TF_VAR_* — JSON object or key=value per line",
    lang: "hcl",
  },
  tofu: {
    id: "tofu",
    label: "OpenTofu",
    short: "OpenTofu",
    icon: Box,
    badge: "text-amber-300 bg-amber-500/10 border-amber-500/30",
    kind: "infra",
    entryLabel: "Working directory",
    entryHint: "dir with .tf files (relative to project, '.' = root)",
    entryPlaceholder: "infra",
    varsLabel: "Variables",
    varsHint: "passed as TF_VAR_* — JSON object or key=value per line",
    lang: "hcl",
  },
  terragrunt: {
    id: "terragrunt",
    label: "Terragrunt",
    short: "Terragrunt",
    icon: Layers,
    badge: "text-cyan-300 bg-cyan-500/10 border-cyan-500/30",
    kind: "infra",
    entryLabel: "Working directory",
    entryHint: "dir with terragrunt.hcl (relative to project)",
    entryPlaceholder: "live/prod",
    varsLabel: "Variables",
    varsHint: "passed as TF_VAR_* — JSON object or key=value per line",
    lang: "hcl",
  },
  pulumi: {
    id: "pulumi",
    label: "Pulumi",
    short: "Pulumi",
    icon: Cloud,
    badge: "text-indigo-300 bg-indigo-500/10 border-indigo-500/30",
    kind: "infra",
    entryLabel: "Working directory",
    entryHint: "dir with Pulumi.yaml (relative to project, '.' = root)",
    entryPlaceholder: "pulumi",
    varsLabel: "Variables",
    varsHint: "exported as environment variables — JSON object or key=value per line",
    lang: "yaml",
  },
  bash: {
    id: "bash",
    label: "Bash",
    short: "Bash",
    icon: SquareTerminal,
    badge: "text-emerald-300 bg-emerald-500/10 border-emerald-500/30",
    kind: "script",
    entryLabel: "Script",
    entryHint: "shell script in the project",
    entryPlaceholder: "scripts/deploy.sh",
    varsLabel: "Variables",
    varsHint: "exported as environment variables — JSON object or key=value per line",
    lang: "shell",
  },
  powershell: {
    id: "powershell",
    label: "PowerShell",
    short: "PowerShell",
    icon: TerminalSquare,
    badge: "text-blue-300 bg-blue-500/10 border-blue-500/30",
    kind: "script",
    entryLabel: "Script",
    entryHint: "PowerShell script in the project",
    entryPlaceholder: "scripts/Deploy.ps1",
    varsLabel: "Variables",
    varsHint: "exported as environment variables — JSON object or key=value per line",
    lang: "powershell",
  },
  python: {
    id: "python",
    label: "Python",
    short: "Python",
    icon: Code2,
    badge: "text-sky-300 bg-sky-500/10 border-sky-500/30",
    kind: "script",
    entryLabel: "Script",
    entryHint: "Python script in the project",
    entryPlaceholder: "scripts/report.py",
    varsLabel: "Variables",
    varsHint: "exported as environment variables — JSON object or key=value per line",
    lang: "python",
  },
};

export const APP_LIST: AppMeta[] = Object.values(APPS);

export function appMeta(id?: string | null): AppMeta {
  return (id && APPS[id as AppId]) || APPS.ansible;
}

// i18n keys for the per-app field labels/hints, derived from kind/id so we don't
// duplicate them per entry. Resolve via appText() at the call site (needs t).
const entryLabelKey: Record<AppKind, string> = {
  ansible: "apps.entryPlaybook",
  infra: "apps.entryWorkdir",
  script: "apps.entryScript",
};
const varsLabelKey: Record<AppKind, string> = {
  ansible: "apps.varsExtra",
  infra: "apps.vars",
  script: "apps.vars",
};
const varsHintKey: Record<AppKind, string> = {
  ansible: "apps.varsHintAnsible",
  infra: "apps.varsHintTf",
  script: "apps.varsHintScript",
};
const entryHintKey: Record<AppId, string> = {
  ansible: "apps.hintAnsible",
  terraform: "apps.hintTf",
  tofu: "apps.hintTf",
  terragrunt: "apps.hintTg",
  pulumi: "apps.hintPulumi",
  bash: "apps.hintBash",
  powershell: "apps.hintPwsh",
  python: "apps.hintPython",
};

/** Localized field labels/hints for an app (placeholder stays literal). */
export function appText(meta: AppMeta, t: (k: string) => string) {
  return {
    entryLabel: t(entryLabelKey[meta.kind]),
    entryHint: t(entryHintKey[meta.id]),
    varsLabel: t(varsLabelKey[meta.kind]),
    varsHint: t(varsHintKey[meta.kind]),
  };
}

export const INFRA_ACTIONS = ["plan", "apply", "destroy"] as const;
export type InfraAction = (typeof INFRA_ACTIONS)[number];

// Pulumi has its own verbs; everything else Terraform-family uses plan/apply/destroy.
export const PULUMI_ACTIONS = ["preview", "up", "destroy"] as const;
export function actionsFor(app?: string | null): readonly string[] {
  return app === "pulumi" ? PULUMI_ACTIONS : INFRA_ACTIONS;
}
export function defaultActionFor(app?: string | null): string {
  return app === "pulumi" ? "preview" : "plan";
}

/** Small app badge used on cards and run rows. */
export function AppBadge({ app, action }: { app?: string | null; action?: string }) {
  const m = appMeta(app);
  const Icon = m.icon;
  return (
    <span
      className={`inline-flex items-center gap-1 rounded-md border px-1.5 py-0.5 text-[11px] font-medium ${m.badge}`}
    >
      <Icon size={12} />
      {m.short}
      {action ? <span className="opacity-70">· {action}</span> : null}
    </span>
  );
}
