import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowDown, ArrowUp, GitMerge, History, Pencil, Play, Plus, RotateCcw, Trash2, X } from "lucide-react";
import { Page, PageHeader } from "../components/PageHeader";
import { Select } from "../components/Select";
import { Checkbox, EmptyState, ErrorText, Field, Modal, Spinner } from "../components/ui";
import { useConfirm, useToast } from "../components/feedback";
import { api, type Template, type Workflow, type WorkflowCondition, type WorkflowRun, type WorkflowStep } from "../lib/api";
import { usePrefs } from "../lib/prefs";
import { fmtRelative } from "../lib/format";

const STEP_DOT: Record<string, string> = {
  success: "bg-success",
  failed: "bg-danger",
  running: "bg-info animate-pulse",
  pending: "bg-ink-faint/40",
  skipped: "bg-ink-faint/30",
  canceled: "bg-ink-faint/50",
};

export function Workflows() {
  const { t } = usePrefs();
  const qc = useQueryClient();
  const toast = useToast();
  const projects = useQuery({ queryKey: ["projects"], queryFn: api.listProjects });
  const [projectId, setProjectId] = useState("");
  useEffect(() => {
    if (!projectId && projects.data?.length) setProjectId(projects.data[0].id);
  }, [projects.data, projectId]);

  const workflows = useQuery({
    queryKey: ["workflows", projectId],
    queryFn: () => api.listWorkflows(projectId),
    enabled: !!projectId,
  });
  const templates = useQuery({
    queryKey: ["templates", projectId],
    queryFn: () => api.listTemplates(projectId),
    enabled: !!projectId,
  });
  const [editing, setEditing] = useState<Workflow | "new" | null>(null);

  const run = useMutation({
    mutationFn: (id: string) => api.runWorkflow(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["workflowRuns"] });
      toast.success(t("wf.started"));
    },
    onError: (e) => toast.error((e as Error).message),
  });

  return (
    <>
      <PageHeader
        title={t("wf.title")}
        subtitle={t("wf.subtitle")}
        actions={
          <button className="btn-primary" disabled={!projectId} onClick={() => setEditing("new")}>
            <Plus size={15} /> {t("wf.new")}
          </button>
        }
      />
      <Page>
        <div className="mb-4 w-72">
          <Select
            value={projectId}
            onChange={setProjectId}
            placeholder={t("wf.selectProject")}
            options={(projects.data ?? []).map((p) => ({ label: p.name, value: p.id }))}
          />
        </div>

        {workflows.isLoading ? (
          <div className="flex justify-center py-16 text-ink-faint"><Spinner /></div>
        ) : workflows.data?.length ? (
          <div className="grid gap-4">
            {workflows.data.map((wf) => (
              <WorkflowCard
                key={wf.id}
                wf={wf}
                templates={templates.data ?? []}
                busy={run.isPending}
                onRun={() => run.mutate(wf.id)}
                onEdit={() => setEditing(wf)}
              />
            ))}
          </div>
        ) : (
          <EmptyState
            icon={<GitMerge size={28} />}
            title={t("wf.none")}
            description={t("wf.noneHint")}
            action={
              projectId ? (
                <button className="btn-primary" onClick={() => setEditing("new")}>
                  <Plus size={15} /> {t("wf.new")}
                </button>
              ) : undefined
            }
          />
        )}

        {projectId && <WorkflowRunsPanel projectId={projectId} templates={templates.data ?? []} />}
      </Page>

      {editing && (
        <WorkflowForm
          projectId={projectId}
          workflow={editing === "new" ? null : editing}
          templates={templates.data ?? []}
          onClose={() => setEditing(null)}
        />
      )}
    </>
  );
}

function condLabel(t: (k: string) => string, c: WorkflowCondition): string {
  return c === "always" ? t("wf.condAlways") : c === "on_failure" ? t("wf.condFailure") : t("wf.condSuccess");
}

const condMeta: Record<string, { icon: string; cls: string }> = {
  on_success: { icon: "✓", cls: "text-success border-success/40" },
  on_failure: { icon: "⚠", cls: "text-danger border-danger/40" },
  always: { icon: "∗", cls: "text-warn border-warn/40" },
};

// Group steps into sequential "waves" (an anchor step + the parallel steps that
// follow it run concurrently) — mirrors the engine's advanceFrom wave logic.
function toWaves(steps: WorkflowStep[]): WorkflowStep[][] {
  const waves: WorkflowStep[][] = [];
  for (const s of steps) {
    if (s.parallel && waves.length) waves[waves.length - 1].push(s);
    else waves.push([s]);
  }
  return waves;
}

// WorkflowGraph renders the pipeline as a visual DAG: waves as columns of node
// cards (parallel steps stacked in a lane), connected left→right by arrows.
function WorkflowGraph({ steps, tName }: { steps: WorkflowStep[]; tName: (id: string) => string }) {
  const { t } = usePrefs();
  const waves = toWaves(steps);
  if (steps.length === 0) return <div className="py-4 text-center text-xs text-ink-faint">{t("wf.noSteps")}</div>;
  return (
    <div className="flex items-center gap-1 overflow-x-auto py-1">
      <span className="shrink-0 rounded-full bg-accent/15 px-2 py-1 text-[10px] font-semibold uppercase text-accent">{t("wf.start")}</span>
      {waves.map((wave, wi) => (
        <div key={wi} className="flex shrink-0 items-center gap-1">
          <span className="text-ink-faint">→</span>
          <div className="flex flex-col gap-1.5">
            {wave.map((s, si) => {
              const m = condMeta[s.condition] ?? condMeta.on_success;
              return (
                <div key={si} className={`rounded-md border bg-surface px-2.5 py-1.5 ${m.cls}`} title={condLabel(t, s.condition)}>
                  <div className="flex items-center gap-1.5">
                    <span className="text-xs font-medium text-ink">{s.name || tName(s.templateId)}</span>
                    <span className="text-[11px]">{m.icon}</span>
                    {wave.length > 1 && <span className="text-[10px] text-info" title={t("wf.parallel")}>∥</span>}
                  </div>
                  <div className="truncate font-mono text-[10px] text-ink-faint">{tName(s.templateId)}</div>
                  {(s.when || s.inventoryId || s.environmentId || s.vaultCredentialId) && (
                    <div className="mt-0.5 flex flex-wrap gap-1">
                      {s.when && <span className="chip text-[9px]">when: {s.when}</span>}
                      {s.inventoryId && <span className="chip text-[9px]">inv</span>}
                      {s.environmentId && <span className="chip text-[9px]">env</span>}
                      {s.vaultCredentialId && <span className="chip text-[9px]">vault</span>}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        </div>
      ))}
      <span className="text-ink-faint">→</span>
      <span className="shrink-0 rounded-full bg-ink-faint/15 px-2 py-1 text-[10px] font-semibold uppercase text-ink-faint">{t("wf.end")}</span>
    </div>
  );
}

function WorkflowVersionsModal({ wf, onClose }: { wf: Workflow; onClose: () => void }) {
  const { t, lang } = usePrefs();
  const qc = useQueryClient();
  const toast = useToast();
  const confirm = useConfirm();
  const versions = useQuery({ queryKey: ["wf-versions", wf.id], queryFn: () => api.listWorkflowVersions(wf.id) });
  const rollback = useMutation({
    mutationFn: (vid: string) => api.rollbackWorkflow(wf.id, vid),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["workflows"] });
      qc.invalidateQueries({ queryKey: ["wf-versions", wf.id] });
      toast.success(t("wf.rolledBack"));
    },
    onError: (e) => toast.error((e as Error).message),
  });
  return (
    <Modal open onClose={onClose} title={`${t("wf.versions")} · ${wf.name}`}>
      {versions.isLoading ? (
        <div className="flex justify-center py-10 text-ink-faint"><Spinner /></div>
      ) : (versions.data?.length ?? 0) === 0 ? (
        <div className="py-6 text-center text-sm text-ink-faint">{t("wf.noVersions")}</div>
      ) : (
        <div className="divide-y divide-border">
          {versions.data!.map((v) => (
            <div key={v.id} className="flex items-center gap-3 py-2.5">
              <span className="chip text-[10px]">v{v.version}</span>
              <div className="min-w-0 flex-1">
                <div className="truncate text-sm text-ink">{v.name} · {v.steps.length} {t("wf.stepsLabel")}</div>
                <div className="truncate text-xs text-ink-faint">{v.actor || t("common.system")} · {fmtRelative(v.createdAt, lang)}</div>
              </div>
              <button className="btn-outline px-2.5 py-1.5 text-sm" disabled={rollback.isPending}
                onClick={() => confirm({ title: t("wf.rollbackConfirm"), body: `v${v.version}` }).then((ok) => ok && rollback.mutate(v.id))}>
                <RotateCcw size={14} /> {t("wf.rollback")}
              </button>
            </div>
          ))}
        </div>
      )}
    </Modal>
  );
}

function WorkflowCard({
  wf,
  templates,
  busy,
  onRun,
  onEdit,
}: {
  wf: Workflow;
  templates: Template[];
  busy: boolean;
  onRun: () => void;
  onEdit: () => void;
}) {
  const { t } = usePrefs();
  const qc = useQueryClient();
  const confirm = useConfirm();
  const [showVersions, setShowVersions] = useState(false);
  const tName = (id: string) => templates.find((x) => x.id === id)?.name ?? id;
  const del = useMutation({
    mutationFn: () => api.deleteWorkflow(wf.id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["workflows"] }),
  });
  return (
    <div className="card p-4">
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="truncate text-sm font-semibold text-ink">{wf.name}</div>
          <div className="truncate text-xs text-ink-faint">{wf.description || t("wf.noDesc")}</div>
        </div>
        <div className="flex shrink-0 items-center gap-1">
          <button className="btn-ghost p-1.5" title={t("wf.versions")} onClick={() => setShowVersions(true)}><History size={14} /></button>
          <button className="btn-ghost p-1.5" title={t("common.edit")} onClick={onEdit}><Pencil size={14} /></button>
          <button
            className="btn-ghost p-1.5 hover:text-danger"
            title={t("common.delete")}
            onClick={() => confirm({ title: t("wf.deleteConfirm"), body: wf.name }).then((ok) => ok && del.mutate())}
          >
            <Trash2 size={14} />
          </button>
        </div>
      </div>
      {showVersions && <WorkflowVersionsModal wf={wf} onClose={() => setShowVersions(false)} />}
      <div className="mt-3">
        <WorkflowGraph steps={wf.steps} tName={tName} />
      </div>
      <div className="mt-4 flex items-center justify-between border-t border-border pt-3">
        <span className="text-xs text-ink-faint">{wf.steps.length} {t("wf.steps")}</span>
        <button className="btn-primary px-3 py-1.5" onClick={onRun} disabled={busy || !wf.steps.length}>
          <Play size={14} /> {t("wf.run")}
        </button>
      </div>
    </div>
  );
}

function WorkflowForm({
  projectId,
  workflow,
  templates,
  onClose,
}: {
  projectId: string;
  workflow: Workflow | null;
  templates: Template[];
  onClose: () => void;
}) {
  const { t } = usePrefs();
  const qc = useQueryClient();
  const inventories = useQuery({
    queryKey: ["inventories", projectId],
    queryFn: () => api.listInventories(projectId),
    enabled: !!projectId,
  });
  const environments = useQuery({ queryKey: ["environments"], queryFn: api.listEnvironments });
  const credentials = useQuery({ queryKey: ["credentials"], queryFn: api.listCredentials });
  const vaultCreds = (credentials.data ?? []).filter((c) => c.type === "vault");
  // Templates across every project the user can run in — lets a step target a
  // template in ANOTHER project (cross-project workflow). Falls back to this
  // project's templates until it loads.
  const allTemplates = useQuery({ queryKey: ["allTemplates"], queryFn: api.listAllTemplates });
  const stepTemplates = allTemplates.data ?? templates;
  const tmplOptions = stepTemplates.map((tpl) => ({
    label: tpl.projectName && tpl.projectId !== projectId ? `${tpl.projectName} / ${tpl.name}` : tpl.name,
    value: tpl.id,
  }));
  const [name, setName] = useState(workflow?.name ?? "");
  const [description, setDescription] = useState(workflow?.description ?? "");
  const [steps, setSteps] = useState<WorkflowStep[]>(
    workflow?.steps ?? [{ name: "", templateId: templates[0]?.id ?? "", condition: "on_success" }],
  );
  const [vars, setVars] = useState<{ k: string; v: string }[]>(
    Object.entries(workflow?.variables ?? {}).map(([k, v]) => ({ k, v })),
  );
  const [error, setError] = useState("");

  const setStep = (i: number, patch: Partial<WorkflowStep>) =>
    setSteps((cur) => cur.map((s, j) => (j === i ? { ...s, ...patch } : s)));
  const move = (i: number, d: -1 | 1) =>
    setSteps((cur) => {
      const j = i + d;
      if (j < 0 || j >= cur.length) return cur;
      const copy = [...cur];
      [copy[i], copy[j]] = [copy[j], copy[i]];
      return copy;
    });

  const save = useMutation({
    mutationFn: () => {
      const variables: Record<string, string> = {};
      for (const { k, v } of vars) if (k.trim()) variables[k.trim()] = v;
      const body = {
        name,
        description,
        variables,
        steps: steps
          .filter((s) => s.templateId)
          .map((s) => ({ ...s, name: s.name || stepTemplates.find((x) => x.id === s.templateId)?.name || "step" })),
      };
      return workflow ? api.updateWorkflow(workflow.id, body) : api.createWorkflow(projectId, body);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["workflows"] });
      onClose();
    },
    onError: (e) => setError((e as Error).message),
  });

  const condOpts = [
    { label: t("wf.condSuccess"), value: "on_success" },
    { label: t("wf.condFailure"), value: "on_failure" },
    { label: t("wf.condAlways"), value: "always" },
  ];

  return (
    <Modal
      open
      wide
      onClose={onClose}
      title={workflow ? t("wf.edit") : t("wf.new")}
      subtitle={t("wf.formSub")}
      footer={
        <>
          <button className="btn-outline" onClick={onClose}>{t("common.cancel")}</button>
          <button
            className="btn-primary"
            onClick={() => { setError(""); save.mutate(); }}
            disabled={!name || !steps.some((s) => s.templateId) || save.isPending}
          >
            {save.isPending ? <Spinner /> : null} {t("common.save")}
          </button>
        </>
      }
    >
      <div className="space-y-4">
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label={t("common.name")}>
            <input className="input" value={name} onChange={(e) => setName(e.target.value)} placeholder="build → deploy" autoFocus />
          </Field>
          <Field label={t("common.description")}>
            <input className="input" value={description} onChange={(e) => setDescription(e.target.value)} placeholder={t("common.optional")} />
          </Field>
        </div>

        <div>
          <div className="label mb-1">{t("wf.stepsLabel")}</div>
          <p className="mb-2 text-xs text-ink-faint">{t("wf.stepsHint")}</p>
          <div className="space-y-2">
            {steps.map((s, i) => (
              <div key={i} className="rounded-lg border border-border bg-surface p-2">
                <div className="flex items-center gap-2">
                  <span className="w-5 shrink-0 text-center text-xs text-ink-faint">{i + 1}</span>
                  <input
                    className="input w-32 text-xs"
                    value={s.name}
                    onChange={(e) => setStep(i, { name: e.target.value })}
                    placeholder={t("wf.stepName")}
                  />
                  <div className="min-w-0 flex-1">
                    <Select
                      value={s.templateId}
                      onChange={(v) => setStep(i, { templateId: v })}
                      placeholder={t("wf.pickTemplate")}
                      options={tmplOptions}
                    />
                  </div>
                  <div className="w-36 shrink-0">
                    <Select value={s.condition} onChange={(v) => setStep(i, { condition: v as WorkflowCondition })} options={condOpts} />
                  </div>
                  <div className="flex shrink-0 items-center">
                    <button className="btn-ghost p-1 disabled:opacity-30" disabled={i === 0} onClick={() => move(i, -1)} title="↑"><ArrowUp size={13} /></button>
                    <button className="btn-ghost p-1 disabled:opacity-30" disabled={i === steps.length - 1} onClick={() => move(i, 1)} title="↓"><ArrowDown size={13} /></button>
                    <button className="btn-ghost p-1 hover:text-danger" onClick={() => setSteps((cur) => cur.filter((_, j) => j !== i))} title={t("common.delete")}><X size={13} /></button>
                  </div>
                </div>
                <div className="mt-1.5 flex items-center gap-2 pl-7">
                  {i > 0 && (
                    <span title={t("wf.parallelHint")}>
                      <Checkbox checked={!!s.parallel} onChange={(v) => setStep(i, { parallel: v })} label={t("wf.parallel")} />
                    </span>
                  )}
                  <span className="text-[11px] text-ink-faint">when</span>
                  <input
                    className="input min-w-0 flex-1 font-mono text-xs"
                    value={s.when ?? ""}
                    onChange={(e) => setStep(i, { when: e.target.value })}
                    placeholder={t("wf.whenPlaceholder")}
                  />
                  <div className="w-40 shrink-0">
                    <Select
                      value={s.inventoryId ?? ""}
                      onChange={(v) => setStep(i, { inventoryId: v })}
                      placeholder={t("wf.invDefault")}
                      options={[{ label: t("wf.invDefault"), value: "" }, ...(inventories.data ?? []).map((inv) => ({ label: inv.name, value: inv.id }))]}
                    />
                  </div>
                  <div className="w-40 shrink-0">
                    <Select
                      value={s.environmentId ?? ""}
                      onChange={(v) => setStep(i, { environmentId: v })}
                      placeholder={t("wf.envDefault")}
                      options={[{ label: t("wf.envDefault"), value: "" }, ...(environments.data ?? []).map((en) => ({ label: en.name, value: en.id }))]}
                    />
                  </div>
                  <div className="w-40 shrink-0">
                    <Select
                      value={s.vaultCredentialId ?? ""}
                      onChange={(v) => setStep(i, { vaultCredentialId: v })}
                      placeholder={t("wf.vaultDefault")}
                      options={[{ label: t("wf.vaultDefault"), value: "" }, ...vaultCreds.map((c) => ({ label: c.name, value: c.id }))]}
                    />
                  </div>
                </div>
              </div>
            ))}
          </div>
          <button
            className="btn-ghost mt-2 text-sm"
            onClick={() => setSteps((cur) => [...cur, { name: "", templateId: templates[0]?.id ?? "", condition: "on_success" }])}
          >
            <Plus size={14} /> {t("wf.addStep")}
          </button>
        </div>

        <div>
          <div className="label mb-1">{t("wf.varsLabel")}</div>
          <p className="mb-2 text-xs text-ink-faint">{t("wf.varsHint")}</p>
          <div className="space-y-2">
            {vars.map((row, i) => (
              <div key={i} className="flex items-center gap-2">
                <input
                  className="input w-44 font-mono text-xs"
                  value={row.k}
                  onChange={(e) => setVars((cur) => cur.map((r, j) => (j === i ? { ...r, k: e.target.value } : r)))}
                  placeholder="version"
                />
                <span className="text-ink-faint">=</span>
                <input
                  className="input flex-1 font-mono text-xs"
                  value={row.v}
                  onChange={(e) => setVars((cur) => cur.map((r, j) => (j === i ? { ...r, v: e.target.value } : r)))}
                  placeholder="1.2.3"
                />
                <button className="btn-ghost p-1 hover:text-danger" onClick={() => setVars((cur) => cur.filter((_, j) => j !== i))} title={t("common.delete")}>
                  <X size={13} />
                </button>
              </div>
            ))}
          </div>
          <button className="btn-ghost mt-2 text-sm" onClick={() => setVars((cur) => [...cur, { k: "", v: "" }])}>
            <Plus size={14} /> {t("wf.addVar")}
          </button>
        </div>
        {error && <ErrorText>{error}</ErrorText>}
      </div>
    </Modal>
  );
}

function WorkflowRunsPanel({ projectId, templates }: { projectId: string; templates: Template[] }) {
  const { t } = usePrefs();
  const runs = useQuery({
    queryKey: ["workflowRuns", projectId],
    queryFn: () => api.listWorkflowRuns({ projectId }),
    enabled: !!projectId,
    refetchInterval: (q) => (q.state.data?.some((r) => r.status === "running") ? 2000 : false),
  });
  const tName = (id: string) => templates.find((x) => x.id === id)?.name ?? id;
  if (!runs.data?.length) return null;
  return (
    <div className="mt-8">
      <h2 className="mb-3 text-sm font-semibold text-ink">{t("wf.recentRuns")}</h2>
      <div className="space-y-2">
        {runs.data.map((wr) => (
          <WorkflowRunRow key={wr.id} wr={wr} tName={tName} />
        ))}
      </div>
    </div>
  );
}

function WorkflowRunRow({ wr, tName }: { wr: WorkflowRun; tName: (id: string) => string }) {
  const { t, lang } = usePrefs();
  const tone =
    wr.status === "success" ? "text-success" : wr.status === "failed" ? "text-danger" : wr.status === "running" ? "text-info" : "text-ink-dim";
  return (
    <div className="card p-3">
      <div className="flex items-center justify-between gap-2">
        <div className="flex min-w-0 items-center gap-2">
          <span className={`inline-block h-2.5 w-2.5 shrink-0 rounded-full ${STEP_DOT[wr.status] ?? "bg-ink-faint"}`} />
          <span className="truncate text-sm text-ink">{wr.name || wr.workflowName}</span>
          <span className={`chip text-[10px] uppercase ${tone}`}>{t("status." + wr.status) || wr.status}</span>
        </div>
        <span className="shrink-0 text-xs text-ink-faint">{wr.triggeredBy} · {fmtRelative(wr.createdAt, lang)}</span>
      </div>
      <div className="mt-2 flex flex-wrap items-center gap-1.5">
        {wr.steps.map((st, i) => {
          const inner = (
            <span className="chip inline-flex items-center gap-1 text-xs">
              <span className={`inline-block h-2 w-2 rounded-full ${STEP_DOT[st.status] ?? "bg-ink-faint"}`} />
              {st.name || tName(st.templateId)}
            </span>
          );
          return (
            <span key={i} className="flex items-center gap-1.5">
              {i > 0 && <span className="text-ink-faint">{st.parallel ? "∥" : "→"}</span>}
              {st.runId ? (
                <a href={`/runs/${st.runId}`} className="no-underline transition hover:brightness-125">{inner}</a>
              ) : (
                inner
              )}
            </span>
          );
        })}
      </div>
    </div>
  );
}
