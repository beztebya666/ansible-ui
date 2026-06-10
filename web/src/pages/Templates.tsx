import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ClipboardList, ListChecks, Package, Pencil, Play, Plus, Rocket, Search, Trash2, X } from "lucide-react";
import { Page, PageHeader } from "../components/PageHeader";
import { Checkbox, EmptyState, ErrorText, Field, Modal, Segmented, Spinner, Toggle } from "../components/ui";
import { useConfirm, usePrompt } from "../components/feedback";
import { Select } from "../components/Select";
import { api, type AppId, type LaunchRequest, type SurveyVar, type Template } from "../lib/api";
import { APP_LIST, AppBadge, appMeta, appText, actionsFor, defaultActionFor } from "../lib/apps";
import { extraVarsToText, parseExtraVars } from "../lib/extravars";
import { usePrefs } from "../lib/prefs";

export function Templates() {
  const { t } = usePrefs();
  const confirm = useConfirm();
  const prompt = usePrompt();
  const qc = useQueryClient();
  const templates = useQuery({ queryKey: ["templates"], queryFn: () => api.listTemplates() });
  const projects = useQuery({ queryKey: ["projects"], queryFn: api.listProjects });
  const views = useQuery({ queryKey: ["views"], queryFn: api.listViews });
  const [editing, setEditing] = useState<Template | null>(null);
  const [creating, setCreating] = useState(false);
  const [activeView, setActiveView] = useState("");
  const [appFilter, setAppFilter] = useState("");
  const [typeFilter, setTypeFilter] = useState("");
  const [search, setSearch] = useState("");

  const projectName = (id: string) => projects.data?.find((p) => p.id === id)?.name ?? "project";

  const filtered = (templates.data ?? []).filter((t) => {
    if (appFilter && (t.app || "ansible") !== appFilter) return false;
    if (typeFilter && (t.type || "task") !== typeFilter) return false;
    if (search) {
      const q = search.toLowerCase();
      if (!t.name.toLowerCase().includes(q) && !t.playbook.toLowerCase().includes(q)) return false;
    }
    return true;
  });

  const applyView = (v?: { id: string; app: string; search: string }) => {
    setActiveView(v?.id ?? "");
    setAppFilter(v?.app ?? "");
    setSearch(v?.search ?? "");
  };
  const saveView = async () => {
    const name = await prompt({ title: t("tpl.saveView"), placeholder: "production" });
    if (!name) return;
    await api.createView({ name, app: appFilter, search });
    qc.invalidateQueries({ queryKey: ["views"] });
  };
  const delView = async (id: string) => {
    if (!(await confirm(t("tpl.deleteView")))) return;
    await api.deleteView(id);
    if (activeView === id) applyView();
    qc.invalidateQueries({ queryKey: ["views"] });
  };

  return (
    <>
      <PageHeader
        title={t("tpl.title")}
        subtitle={t("tpl.subtitle")}
        actions={
          <button className="btn-primary" onClick={() => setCreating(true)}>
            <Plus size={15} /> {t("tpl.new")}
          </button>
        }
      />
      <Page>
        {/* View tabs */}
        <div className="mb-3 flex flex-wrap items-center gap-1.5 border-b border-border pb-3">
          <ViewTab label={t("common.all")} active={!activeView} onClick={() => applyView()} />
          {views.data?.map((v) => (
            <ViewTab
              key={v.id}
              label={v.name}
              active={activeView === v.id}
              onClick={() => applyView(v)}
              onDelete={() => delView(v.id)}
            />
          ))}
        </div>

        {/* Filter toolbar */}
        <div className="mb-4 flex flex-wrap items-center gap-2">
          <div className="relative">
            <Search size={14} className="pointer-events-none absolute left-2.5 top-2.5 text-ink-faint" />
            <input
              className="input w-56 pl-8"
              placeholder={t("common.search")}
              value={search}
              onChange={(e) => { setSearch(e.target.value); setActiveView(""); }}
            />
          </div>
          <Select
            className="w-40"
            value={appFilter}
            onChange={(v) => { setAppFilter(v); setActiveView(""); }}
            options={[{ label: t("runs.allApps"), value: "" }, ...APP_LIST.map((a) => ({ label: a.label, value: a.id }))]}
          />
          <Select
            className="w-36"
            value={typeFilter}
            onChange={setTypeFilter}
            options={[
              { label: t("tpl.allTypes"), value: "" },
              { label: t("tpl.typeTask"), value: "task" },
              { label: t("tpl.typeBuild"), value: "build" },
              { label: t("tpl.typeDeploy"), value: "deploy" },
            ]}
          />
          {(appFilter || search) && (
            <button className="btn-ghost text-xs" onClick={saveView}>
              <Plus size={13} /> {t("tpl.saveView")}
            </button>
          )}
          <span className="ml-auto text-xs text-ink-faint">{filtered.length} {t("tpl.count")}</span>
        </div>

        {templates.isLoading ? (
          <div className="flex justify-center py-20 text-ink-faint">
            <Spinner />
          </div>
        ) : filtered.length ? (
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {filtered.map((t) => (
              <TemplateCard key={t.id} template={t} projectName={projectName(t.projectId)} onEdit={() => setEditing(t)} />
            ))}
          </div>
        ) : (
          <EmptyState
            icon={<ListChecks size={28} />}
            title={templates.data?.length ? t("tpl.noneMatch") : t("tpl.none")}
            description={t("tpl.noneHint")}
            action={
              <button className="btn-primary" onClick={() => setCreating(true)}>
                <Plus size={15} /> {t("tpl.new")}
              </button>
            }
          />
        )}
      </Page>

      {(creating || editing) && (
        <TemplateForm template={editing} onClose={() => { setCreating(false); setEditing(null); }} />
      )}
    </>
  );
}

function ViewTab({
  label,
  active,
  onClick,
  onDelete,
}: {
  label: string;
  active: boolean;
  onClick: () => void;
  onDelete?: () => void;
}) {
  const { t } = usePrefs();
  return (
    <span
      className={
        "group inline-flex items-center gap-1 rounded-lg border px-2.5 py-1 text-xs font-medium transition-colors " +
        (active ? "border-accent bg-accent/10 text-accent" : "border-border text-ink-dim hover:border-border-strong")
      }
    >
      <button onClick={onClick}>{label}</button>
      {onDelete && (
        <button onClick={onDelete} className="opacity-0 transition-opacity group-hover:opacity-100" title={t("tpl.deleteView")}>
          <X size={11} />
        </button>
      )}
    </span>
  );
}

function TemplateCard({
  template,
  projectName,
  onEdit,
}: {
  template: Template;
  projectName: string;
  onEdit: () => void;
}) {
  const { t } = usePrefs();
  const confirm = useConfirm();
  const qc = useQueryClient();
  const navigate = useNavigate();
  const [busy, setBusy] = useState(false);
  const [dialog, setDialog] = useState(false);

  const del = useMutation({
    mutationFn: () => api.deleteTemplate(template.id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["templates"] }),
  });

  const launch = async (overrides?: Partial<LaunchRequest>) => {
    setBusy(true);
    try {
      const r = await api.runTemplate(template.id, overrides);
      qc.invalidateQueries({ queryKey: ["runs"] });
      navigate(`/runs/${r.id}`);
    } finally {
      setBusy(false);
    }
  };

  // Ask the operator before launching when the template has survey variables or
  // any "prompt at launch" field enabled; otherwise run straight away.
  const anyPrompt = !!template.prompts && Object.values(template.prompts).some(Boolean);
  const run = () => {
    if (template.surveyVars?.length || anyPrompt) setDialog(true);
    else launch();
  };

  return (
    <div className="card group flex flex-col p-4 transition-colors hover:border-border-strong">
      <div className="flex items-start justify-between">
        <div className="min-w-0">
          <h3 className="truncate font-medium text-ink">{template.name}</h3>
          <p className="mt-1 line-clamp-2 min-h-[2.5rem] text-sm text-ink-dim">
            {template.description || <span className="text-ink-faint/60">{t("tpl.noDescription")}</span>}
          </p>
        </div>
        <div className="flex shrink-0 gap-0.5 opacity-0 group-hover:opacity-100">
          <button onClick={onEdit} className="btn-ghost p-1.5" title={t("common.edit")}>
            <Pencil size={14} />
          </button>
          <button
            onClick={() => confirm({ title: t("tpl.deleteConfirm"), body: template.name }).then((ok) => ok && del.mutate())}
            className="btn-ghost p-1.5"
            title={t("common.delete")}
          >
            <Trash2 size={14} />
          </button>
        </div>
      </div>

      <div className="mt-3 flex flex-wrap items-center gap-1.5 text-xs">
        <AppBadge app={template.app} action={template.action} />
        {template.type === "build" && <span className="chip text-accent">{t("tpl.typeBuild")}</span>}
        {template.type === "deploy" && <span className="chip text-info">{t("tpl.typeDeploy")}</span>}
        {template.lastBuildVersion && (
          <span
            className="chip inline-flex items-center gap-1 text-success"
            title={template.type === "deploy" ? t("tpl.deploysVersion") : t("tpl.lastBuild")}
          >
            <Package size={11} /> {template.lastBuildVersion}
          </span>
        )}
        <span className="chip font-mono">{template.playbook}</span>
        {template.app === "ansible" && template.tags && <span className="chip">tags: {template.tags}</span>}
        {template.app === "ansible" && template.check && <span className="chip text-warn">check</span>}
        {template.surveyVars?.length ? <span className="chip text-info">survey · {template.surveyVars.length}</span> : null}
      </div>

      <div className="mt-4 flex items-center justify-between border-t border-border pt-3">
        <span className="text-xs text-ink-faint">{projectName}</span>
        <button className="btn-primary px-3 py-1.5" onClick={run} disabled={busy}>
          {busy ? <Spinner /> : <Play size={14} />} {t("common.run")}
        </button>
      </div>
      {dialog && (
        <RunTemplateDialog template={template} busy={busy} onClose={() => setDialog(false)} onSubmit={launch} />
      )}
    </div>
  );
}

function TemplateForm({ template, onClose }: { template: Template | null; onClose: () => void }) {
  const qc = useQueryClient();
  const projects = useQuery({ queryKey: ["projects"], queryFn: api.listProjects });

  const [projectId, setProjectId] = useState(template?.projectId ?? "");
  const [name, setName] = useState(template?.name ?? "");
  const [description, setDescription] = useState(template?.description ?? "");
  const [app, setApp] = useState<AppId>(template?.app ?? "ansible");
  const [action, setAction] = useState(template?.action ?? "plan");
  const [playbook, setPlaybook] = useState(template?.playbook ?? "");
  const [inventoryId, setInventoryId] = useState(template?.inventoryId ?? "");
  const [inventoryIds, setInventoryIds] = useState<string[]>(template?.inventoryIds ?? []);
  const [limit, setLimit] = useState(template?.limit ?? "");
  const [tags, setTags] = useState(template?.tags ?? "");
  const [skipTags, setSkipTags] = useState(template?.skipTags ?? "");
  const [extraVars, setExtraVars] = useState(extraVarsToText(template?.extraVars));
  const [check, setCheck] = useState(template?.check ?? false);
  const [diff, setDiff] = useState(template?.diff ?? true);
  const [verbosity, setVerbosity] = useState(template?.verbosity ?? 0);
  const [environmentId, setEnvironmentId] = useState(template?.environmentId ?? "");
  const [vaultCredentialId, setVaultCredentialId] = useState(template?.vaultCredentialId ?? "");
  const [surveyVars, setSurveyVars] = useState<SurveyVar[]>(template?.surveyVars ?? []);
  const [cliArgs, setCliArgs] = useState((template?.cliArgs ?? []).join(" "));
  const [allowParallel, setAllowParallel] = useState(template?.allowParallel ?? false);
  const [maxConcurrent, setMaxConcurrent] = useState(String(template?.maxConcurrent ?? 0));
  const [autorunOnCommit, setAutorunOnCommit] = useState(template?.autorunOnCommit ?? false);
  const [suppressSuccess, setSuppressSuccess] = useState(template?.suppressSuccessNotifications ?? false);
  const [suppressAll, setSuppressAll] = useState(template?.suppressAllNotifications ?? false);
  const [requiresApproval, setRequiresApproval] = useState(template?.requiresApproval ?? false);
  const [artifactPaths, setArtifactPaths] = useState((template?.artifactPaths ?? []).join("\n"));
  const [workspace, setWorkspace] = useState(template?.workspace ?? "");
  const [autoApprove, setAutoApprove] = useState(template?.autoApprove ?? false);
  const [tfBackend, setTfBackend] = useState(template?.tfBackend ?? "");
  const [prompts, setPrompts] = useState<NonNullable<Template["prompts"]>>(template?.prompts ?? {});
  const [type, setType] = useState<"task" | "build" | "deploy">(template?.type ?? "task");
  const [buildTemplateId, setBuildTemplateId] = useState(template?.buildTemplateId ?? "");
  const [runnerTag, setRunnerTag] = useState(template?.runnerTag ?? "");
  const [error, setError] = useState("");

  const environments = useQuery({ queryKey: ["environments"], queryFn: api.listEnvironments });
  const credentials = useQuery({ queryKey: ["credentials"], queryFn: api.listCredentials });

  useEffect(() => {
    if (!projectId && projects.data?.length) setProjectId(projects.data[0].id);
  }, [projects.data, projectId]);

  const project = useQuery({
    queryKey: ["project", projectId],
    queryFn: () => api.getProject(projectId),
    enabled: !!projectId,
  });
  const inventories = useQuery({
    queryKey: ["inventories", projectId],
    queryFn: () => api.listInventories(projectId),
    enabled: !!projectId,
  });
  const projTemplates = useQuery({
    queryKey: ["templates", projectId],
    queryFn: () => api.listTemplates(projectId),
    enabled: !!projectId && type === "deploy",
  });
  const buildTemplates = (projTemplates.data ?? []).filter((tt) => tt.type === "build" && tt.id !== template?.id);
  const runners = useQuery({ queryKey: ["runners"], queryFn: api.listRunners });
  const runnerTags = Array.from(new Set((runners.data ?? []).flatMap((rn) => rn.tags || []))).sort();
  const { t } = usePrefs();
  const meta = appMeta(app);
  const at = appText(meta, t);
  const playbooks = project.data?.playbooks ?? [];
  useEffect(() => {
    if (app === "ansible" && !template && playbooks.length && !playbooks.includes(playbook)) {
      setPlaybook(playbooks[0]);
    }
  }, [app, playbooks, playbook, template]);
  // Keep the action valid for the selected app (Pulumi uses preview/up/destroy).
  useEffect(() => {
    if (meta.kind === "infra" && !actionsFor(app).includes(action)) {
      setAction(defaultActionFor(app));
    }
  }, [app]); // eslint-disable-line react-hooks/exhaustive-deps

  const save = useMutation({
    mutationFn: () => {
      const parsed = parseExtraVars(extraVars);
      if (parsed.error) throw new Error(parsed.error);
      const isInfra = meta.kind === "infra";
      const isAnsible = app === "ansible";
      const entry = isInfra ? (playbook.trim() || ".") : playbook.trim();
      const body: Partial<Template> = {
        projectId,
        name,
        description,
        app,
        action: isInfra ? action : "",
        playbook: entry,
        inventoryId: isAnsible ? inventoryId || null : null,
        inventoryIds: isAnsible ? inventoryIds : [],
        vaultCredentialId: isAnsible ? vaultCredentialId || null : null,
        environmentId: environmentId || null,
        surveyVars,
        limit: isAnsible ? limit : "",
        tags: isAnsible ? tags : "",
        skipTags: isAnsible ? skipTags : "",
        extraVars: parsed.vars,
        check: isAnsible ? check : false,
        diff: isAnsible ? diff : false,
        verbosity: isAnsible ? verbosity : 0,
        cliArgs: cliArgs.trim() ? cliArgs.trim().split(/\s+/) : [],
        prompts,
        allowParallel,
        maxConcurrent: allowParallel ? Math.max(0, parseInt(maxConcurrent) || 0) : 0,
        autorunOnCommit,
        suppressSuccessNotifications: suppressSuccess,
        suppressAllNotifications: suppressAll,
        requiresApproval,
        artifactPaths: artifactPaths.split("\n").map((s) => s.trim()).filter(Boolean),
        workspace: isInfra ? workspace : "",
        autoApprove: isInfra ? autoApprove : false,
        tfBackend: isInfra ? tfBackend.trim() : "",
        type,
        buildTemplateId: type === "deploy" ? buildTemplateId || null : null,
        runnerTag,
      };
      return template ? api.updateTemplate(template.id, body) : api.createTemplate(body);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["templates"] });
      onClose();
    },
    onError: (e) => setError((e as Error).message),
  });

  return (
    <Modal
      open
      onClose={onClose}
      wide
      title={template ? t("tpl.edit") : t("tpl.new")}
      subtitle={t("tpl.formSub")}
      footer={
        <>
          <button className="btn-outline" onClick={onClose}>{t("common.cancel")}</button>
          <button
            className="btn-primary"
            onClick={() => { setError(""); save.mutate(); }}
            disabled={!projectId || !name || (meta.kind !== "infra" && !playbook) || save.isPending}
          >
            {save.isPending ? <Spinner /> : null} {t("common.save")}
          </button>
        </>
      }
    >
      <div className="grid gap-4 sm:grid-cols-2">
        {/* Template type — Semaphore-style TASK / BUILD / DEPLOY tabs, up top. */}
        <div className="sm:col-span-2">
          <span className="label">{t("tpl.type")}</span>
          <div className="grid grid-cols-3 gap-1.5">
            {([
              { id: "task", label: t("tpl.typeTask"), desc: t("tpl.typeTaskDesc"), Icon: ClipboardList },
              { id: "build", label: t("tpl.typeBuild"), desc: t("tpl.typeBuildDesc"), Icon: Package },
              { id: "deploy", label: t("tpl.typeDeploy"), desc: t("tpl.typeDeployDesc"), Icon: Rocket },
            ] as const).map(({ id, label, desc, Icon }) => {
              const active = type === id;
              return (
                <button
                  key={id}
                  type="button"
                  onClick={() => setType(id)}
                  className={
                    "flex flex-col gap-1 rounded-lg border px-3 py-2.5 text-left transition-colors " +
                    (active ? "border-accent bg-accent/10" : "border-border hover:border-border-strong")
                  }
                >
                  <span className="flex items-center gap-1.5 text-sm font-medium text-ink">
                    <Icon size={15} className={active ? "text-accent" : "text-ink-dim"} /> {label}
                  </span>
                  <span className="text-[11px] leading-tight text-ink-faint">{desc}</span>
                </button>
              );
            })}
          </div>
        </div>
        {type === "deploy" && (
          <div className="sm:col-span-2">
            <Field label={t("tpl.buildTemplate")} hint={t("tpl.buildTemplateHint")}>
              <Select
                value={buildTemplateId}
                onChange={setBuildTemplateId}
                placeholder={buildTemplates.length ? t("tpl.selectBuild") : t("tpl.noBuilds")}
                options={buildTemplates.map((b) => ({ label: b.name, value: b.id }))}
              />
            </Field>
          </div>
        )}

        <div className="sm:col-span-2">
          <span className="label">{t("launch.app")}</span>
          <div className="flex flex-wrap gap-1.5">
            {APP_LIST.map((a) => {
              const Icon = a.icon;
              const active = a.id === app;
              return (
                <button
                  key={a.id}
                  type="button"
                  onClick={() => setApp(a.id)}
                  className={
                    "flex items-center gap-1.5 rounded-lg border px-2.5 py-1.5 text-xs font-medium transition-colors " +
                    (active
                      ? "border-accent bg-accent/10 text-ink"
                      : "border-border text-ink-dim hover:border-border-strong")
                  }
                >
                  <Icon size={14} className={active ? "text-accent" : ""} />
                  {a.label}
                </button>
              );
            })}
          </div>
        </div>

        <Field label={t("run.project")}>
          <Select
            value={projectId}
            onChange={setProjectId}
            placeholder={t("launch.selectProject")}
            options={(projects.data ?? []).map((p) => ({ label: p.name, value: p.id }))}
          />
        </Field>
        <Field label={t("common.name")}>
          <input className="input" value={name} onChange={(e) => setName(e.target.value)} placeholder="Deploy production" />
        </Field>
        <div className="sm:col-span-2">
          <Field label={t("common.description")}>
            <input className="input" value={description} onChange={(e) => setDescription(e.target.value)} placeholder={t("common.optional")} />
          </Field>
        </div>

        <div className="grid gap-4 sm:col-span-2 sm:grid-cols-2">
          <Field label={t("tpl.runner")} hint={t("tpl.runnerHint")}>
            <Select
              value={runnerTag === "default" ? "" : runnerTag}
              onChange={setRunnerTag}
              options={[
                { label: t("tpl.runnerDefault"), value: "" },
                ...runnerTags.filter((tg) => tg !== "default").map((tg) => ({ label: tg, value: tg })),
              ]}
            />
          </Field>
        </div>

        {/* Entrypoint — differs per app */}
        {app === "ansible" ? (
          <Field label={at.entryLabel} hint={at.entryHint}>
            <Select
              value={playbook}
              onChange={setPlaybook}
              disabled={!playbooks.length}
              placeholder={playbook || t("launch.noPlaybooks")}
              options={playbooks.map((p) => ({ label: p, value: p }))}
            />
          </Field>
        ) : (
          <Field label={at.entryLabel} hint={at.entryHint}>
            <input className="input font-mono text-xs" value={playbook} onChange={(e) => setPlaybook(e.target.value)} placeholder={meta.entryPlaceholder} />
          </Field>
        )}

        {meta.kind === "infra" && (
          <Field label={t("run.action")} hint={t("launch.actionHint")}>
            <Select value={action} onChange={setAction} options={actionsFor(app).map((a) => ({ label: a, value: a }))} />
          </Field>
        )}

        {app === "ansible" && (
          <>
            <Field label={t("pd.inventories")}>
              <Select
                value={inventoryId ?? ""}
                onChange={setInventoryId}
                options={[{ label: t("launch.projectDefault"), value: "" }, ...(inventories.data ?? []).map((inv) => ({ label: inv.name, value: inv.id }))]}
              />
            </Field>
            {(inventories.data ?? []).length > 0 && (
              <Field label={t("tpl.altInventories")} hint={t("tpl.altInventoriesHint")}>
                <div className="flex flex-wrap gap-3">
                  {(inventories.data ?? []).filter((inv) => inv.id !== inventoryId).map((inv) => (
                    <Checkbox
                      key={inv.id}
                      checked={inventoryIds.includes(inv.id)}
                      onChange={(v) => setInventoryIds((cur) => (v ? [...cur, inv.id] : cur.filter((x) => x !== inv.id)))}
                      label={inv.name}
                    />
                  ))}
                </div>
              </Field>
            )}
            <Field label={t("tpl.vault")} hint={t("tpl.vaultHint")}>
              <Select
                value={vaultCredentialId ?? ""}
                onChange={setVaultCredentialId}
                options={[{ label: t("common.none"), value: "" }, ...(credentials.data?.filter((c) => c.type === "vault").map((c) => ({ label: c.name, value: c.id })) ?? [])]}
              />
            </Field>
            <Field label={t("run.limit")}><input className="input" value={limit} onChange={(e) => setLimit(e.target.value)} /></Field>
            <Field label={t("run.tags")}><input className="input" value={tags} onChange={(e) => setTags(e.target.value)} /></Field>
            <Field label={t("run.skipTags")}><input className="input" value={skipTags} onChange={(e) => setSkipTags(e.target.value)} /></Field>
          </>
        )}

        <Field label={t("launch.environment")} hint={t("launch.envHint")}>
          <Select
            value={environmentId ?? ""}
            onChange={setEnvironmentId}
            options={[{ label: t("common.none"), value: "" }, ...(environments.data ?? []).map((en) => ({ label: en.name, value: en.id }))]}
          />
        </Field>

        <div className="sm:col-span-2">
          <Field label={at.varsLabel} hint={at.varsHint}>
            <textarea className="input min-h-[64px] font-mono text-xs" value={extraVars} onChange={(e) => setExtraVars(e.target.value)} />
          </Field>
        </div>
        <div className="space-y-3 border-t border-border pt-3 sm:col-span-2">
          <div className="label mb-0">{t("tpl.advanced")}</div>
          <Field label={t("tpl.cliArgs")} hint={t("tpl.cliArgsHint")}>
            <input className="input font-mono text-xs" value={cliArgs} onChange={(e) => setCliArgs(e.target.value)} placeholder="--forks 10 --timeout 60" />
          </Field>
          <Field label={t("tpl.artifactPaths")} hint={t("tpl.artifactPathsHint")}>
            <textarea
              className="input min-h-[68px] font-mono text-xs"
              spellCheck={false}
              value={artifactPaths}
              onChange={(e) => setArtifactPaths(e.target.value)}
              placeholder={"reports/*.html\noutput.json"}
            />
          </Field>
          {meta.kind === "infra" && (
            <>
              <div className="grid items-end gap-4 sm:grid-cols-2">
                <Field label={t("tpl.workspace")} hint={t("tpl.workspaceHint")}>
                  <input className="input" value={workspace} onChange={(e) => setWorkspace(e.target.value)} placeholder="prod" />
                </Field>
                <div className="pb-2"><Toggle checked={autoApprove} onChange={setAutoApprove} label={t("tpl.autoApprove")} /></div>
              </div>
              {(app === "terraform" || app === "tofu") && (
                <Field label={t("tpl.tfBackend")} hint={t("tpl.tfBackendHint")}>
                  <input className="input font-mono text-xs" value={tfBackend} onChange={(e) => setTfBackend(e.target.value)}
                    placeholder="https://state.example.com/tf/myproject" />
                </Field>
              )}
            </>
          )}
          {allowParallel && (
            <Field label={t("tpl.maxConcurrent")} hint={t("tpl.maxConcurrentHint")}>
              <input type="number" min={0} className="input w-32" value={maxConcurrent} onChange={(e) => setMaxConcurrent(e.target.value)} placeholder="0" />
            </Field>
          )}
          <div className="flex flex-wrap gap-x-6 gap-y-2">
            <Toggle checked={allowParallel} onChange={setAllowParallel} label={t("tpl.allowParallel")} />
            <Toggle checked={autorunOnCommit} onChange={setAutorunOnCommit} label={t("tpl.autorun")} />
            <Toggle checked={suppressSuccess} onChange={setSuppressSuccess} label={t("tpl.suppress")} />
            <Toggle checked={suppressAll} onChange={setSuppressAll} label={t("tpl.suppressAll")} />
            <Toggle checked={requiresApproval} onChange={setRequiresApproval} label={t("tpl.requiresApproval")} />
          </div>
          <div>
            <div className="label">{t("tpl.promptsTitle")}</div>
            <div className="flex flex-wrap gap-x-4 gap-y-2 text-sm text-ink-dim">
              {([
                ["cliArgs", t("tpl.cliArgs")],
                ["branch", t("tpl.branch")],
                ["inventory", t("pd.inventories")],
                ["limit", t("run.limit")],
                ["tags", t("run.tags")],
                ["skipTags", t("run.skipTags")],
                ["debug", t("tpl.debug")],
                ["vaults", t("tpl.vault")],
              ] as const).map(([k, label]) => (
                <Checkbox
                  key={k}
                  checked={!!prompts[k]}
                  onChange={(v) => setPrompts((p) => ({ ...p, [k]: v }))}
                  label={label}
                />
              ))}
            </div>
          </div>
        </div>

        <div className="sm:col-span-2">
          <SurveyEditor value={surveyVars} onChange={setSurveyVars} />
        </div>

        {app === "ansible" && (
          <div className="flex flex-wrap items-center gap-x-6 gap-y-3 sm:col-span-2">
            <Toggle checked={check} onChange={setCheck} label={`${t("run.check")} (dry-run)`} />
            <Toggle checked={diff} onChange={setDiff} label={t("run.diff")} />
            <div className="ml-auto flex items-center gap-2">
              <span className="label mb-0">{t("run.verbosity")}</span>
              <Segmented
                value={verbosity}
                onChange={setVerbosity}
                options={[
                  { label: "0", value: 0 },
                  { label: "v", value: 1 },
                  { label: "vv", value: 2 },
                  { label: "vvv", value: 3 },
                ]}
              />
            </div>
          </div>
        )}
        {error && <div className="sm:col-span-2"><ErrorText>{error}</ErrorText></div>}
      </div>
    </Modal>
  );
}

function SurveyEditor({ value, onChange }: { value: SurveyVar[]; onChange: (v: SurveyVar[]) => void }) {
  const { t } = usePrefs();
  const update = (i: number, patch: Partial<SurveyVar>) =>
    onChange(value.map((v, idx) => (idx === i ? { ...v, ...patch } : v)));
  const remove = (i: number) => onChange(value.filter((_, idx) => idx !== i));
  const add = () => onChange([...value, { name: "", title: "", type: "text", required: false }]);

  return (
    <div>
      <div className="mb-1.5 flex items-center justify-between">
        <span className="label mb-0 flex items-center gap-1.5"><ClipboardList size={13} /> {t("tpl.surveyTitle")}</span>
        <button type="button" className="btn-ghost px-2 py-1 text-xs" onClick={add}>
          <Plus size={13} /> {t("tpl.addPrompt")}
        </button>
      </div>
      {value.length === 0 && (
        <p className="text-xs text-ink-faint">{t("tpl.surveyHint")}</p>
      )}
      <div className="space-y-2">
        {value.map((v, i) => (
          <div key={i} className="rounded-lg border border-border bg-surface p-2.5">
            <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
              <input className="input" placeholder={t("tpl.surveyName")} value={v.name} onChange={(e) => update(i, { name: e.target.value })} />
              <input className="input" placeholder={t("tpl.surveyVarTitle")} value={v.title} onChange={(e) => update(i, { title: e.target.value })} />
              <Select
                value={v.type}
                onChange={(val) => update(i, { type: val as SurveyVar["type"] })}
                options={["text", "int", "enum", "secret"].map((x) => ({ label: x, value: x }))}
              />
              <input className="input" placeholder={t("tpl.surveyDefault")} value={v.default ?? ""} onChange={(e) => update(i, { default: e.target.value })} />
            </div>
            <div className="mt-2 flex items-center gap-3">
              {v.type === "enum" && (
                <input
                  className="input flex-1"
                  placeholder={t("tpl.surveyOptions")}
                  value={(v.options ?? []).join(",")}
                  onChange={(e) => update(i, { options: e.target.value.split(",").map((s) => s.trim()).filter(Boolean) })}
                />
              )}
              <Checkbox checked={v.required} onChange={(val) => update(i, { required: val })} label={t("tpl.required")} />
              <button type="button" className="btn-ghost ml-auto p-1.5" onClick={() => remove(i)} title={t("common.remove")}>
                <X size={14} />
              </button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

// RunTemplateDialog asks the operator for inputs before launching a template:
// its survey variables AND any built-in field the template marks as a launch
// prompt (inventory / branch / limit / tags / skip-tags / CLI args / verbosity).
// Each is pre-filled with the template's saved value; the operator tweaks then runs.
function RunTemplateDialog({
  template,
  busy,
  onClose,
  onSubmit,
}: {
  template: Template;
  busy: boolean;
  onClose: () => void;
  onSubmit: (overrides: Partial<LaunchRequest>) => void;
}) {
  const { t } = usePrefs();
  const p = template.prompts ?? {};
  const isAnsible = !template.app || template.app === "ansible";
  const surveyVars = template.surveyVars ?? [];
  const inventories = useQuery({
    queryKey: ["inventories", template.projectId],
    queryFn: () => api.listInventories(template.projectId),
    enabled: !!p.inventory,
  });
  const creds = useQuery({ queryKey: ["credentials"], queryFn: api.listCredentials, enabled: !!p.vaults });
  const vaultCreds = (creds.data ?? []).filter((c) => c.type === "vault");

  const [answers, setAnswers] = useState<Record<string, string>>(() =>
    Object.fromEntries(surveyVars.map((v) => [v.name, v.default ?? ""])),
  );
  const [inventoryId, setInventoryId] = useState(template.inventoryId ?? "");
  const [branch, setBranch] = useState("");
  const [limit, setLimit] = useState(template.limit ?? "");
  const [tags, setTags] = useState(template.tags ?? "");
  const [skipTags, setSkipTags] = useState(template.skipTags ?? "");
  const [cliArgs, setCliArgs] = useState((template.cliArgs ?? []).join(" "));
  const [verbosity, setVerbosity] = useState(template.verbosity ?? 0);
  const [vaults, setVaults] = useState<string[]>(() => {
    const base = [...(template.vaults ?? [])];
    if (template.vaultCredentialId) base.push(template.vaultCredentialId);
    return Array.from(new Set(base));
  });
  const toggleVault = (id: string) => setVaults((v) => (v.includes(id) ? v.filter((x) => x !== id) : [...v, id]));
  const [error, setError] = useState("");

  const submit = () => {
    setError("");
    for (const v of surveyVars) {
      if (v.required && !answers[v.name]) {
        setError(`${v.title || v.name} is required`);
        return;
      }
    }
    const ov: Partial<LaunchRequest> = {};
    if (surveyVars.length) {
      const extra: Record<string, unknown> = {};
      for (const v of surveyVars) {
        const raw = answers[v.name] ?? "";
        extra[v.name] = v.type === "int" ? Number(raw) : raw;
      }
      ov.extraVars = extra;
    }
    if (p.inventory && isAnsible) ov.inventoryId = inventoryId || undefined;
    if (p.branch && branch.trim()) ov.branch = branch.trim();
    if (p.limit && isAnsible) ov.limit = limit;
    if (p.tags && isAnsible) ov.tags = tags;
    if (p.skipTags && isAnsible) ov.skipTags = skipTags;
    if (p.cliArgs) ov.cliArgs = cliArgs.trim() ? cliArgs.trim().split(/\s+/) : [];
    if (p.debug && isAnsible) ov.verbosity = verbosity;
    if (p.vaults && isAnsible) ov.vaults = vaults;
    onSubmit(ov);
  };

  return (
    <Modal
      open
      onClose={onClose}
      title={`${t("common.run")} · ${template.name}`}
      subtitle={t("launch.adjustInputs")}
      footer={
        <>
          <button className="btn-outline" onClick={onClose}>{t("common.cancel")}</button>
          <button className="btn-primary" onClick={submit} disabled={busy}>
            {busy ? <Spinner /> : <Play size={14} />} {t("launch.go")}
          </button>
        </>
      }
    >
      <div className="space-y-4">
        {surveyVars.map((v) => (
          <Field key={v.name} label={(v.title || v.name) + (v.required ? " *" : "")} hint={v.description}>
            {v.type === "enum" ? (
              <Select
                value={answers[v.name] ?? ""}
                onChange={(val) => setAnswers((a) => ({ ...a, [v.name]: val }))}
                options={(v.options ?? []).map((o) => ({ label: o, value: o }))}
              />
            ) : (
              <input
                className="input"
                type={v.type === "secret" ? "password" : v.type === "int" ? "number" : "text"}
                value={answers[v.name] ?? ""}
                onChange={(e) => setAnswers((a) => ({ ...a, [v.name]: e.target.value }))}
              />
            )}
          </Field>
        ))}

        {p.inventory && isAnsible && (
          <Field label={t("pd.inventories")} hint={(template.inventoryIds?.length ?? 0) > 0 ? t("tpl.curatedInvHint") : undefined}>
            <Select
              value={inventoryId}
              onChange={setInventoryId}
              options={[
                { label: t("launch.projectDefault"), value: "" },
                ...(inventories.data ?? [])
                  .filter((inv) => !(template.inventoryIds?.length) || template.inventoryIds.includes(inv.id) || inv.id === template.inventoryId)
                  .map((inv) => ({ label: inv.name, value: inv.id })),
              ]}
            />
          </Field>
        )}
        {p.branch && (
          <Field label={t("tpl.branch")} hint={t("tpl.branchHint")}>
            <input className="input font-mono text-xs" value={branch} onChange={(e) => setBranch(e.target.value)} placeholder="(repo default)" />
          </Field>
        )}
        {p.limit && isAnsible && (
          <Field label={t("run.limit")} hint={t("launch.limitHint")}>
            <input className="input" value={limit} onChange={(e) => setLimit(e.target.value)} placeholder="all" />
          </Field>
        )}
        {p.tags && isAnsible && (
          <Field label={t("run.tags")} hint={t("launch.tagsHint")}>
            <input className="input" value={tags} onChange={(e) => setTags(e.target.value)} />
          </Field>
        )}
        {p.skipTags && isAnsible && (
          <Field label={t("run.skipTags")} hint={t("launch.skipTagsHint")}>
            <input className="input" value={skipTags} onChange={(e) => setSkipTags(e.target.value)} />
          </Field>
        )}
        {p.cliArgs && (
          <Field label={t("tpl.cliArgs")} hint={t("tpl.cliArgsHint")}>
            <input className="input font-mono text-xs" value={cliArgs} onChange={(e) => setCliArgs(e.target.value)} placeholder="--forks 10" />
          </Field>
        )}
        {p.debug && isAnsible && (
          <Field label={t("run.verbosity")}>
            <Segmented
              value={verbosity}
              onChange={setVerbosity}
              options={[
                { label: "0", value: 0 },
                { label: "v", value: 1 },
                { label: "vv", value: 2 },
                { label: "vvv", value: 3 },
              ]}
            />
          </Field>
        )}
        {p.vaults && isAnsible && (
          <Field label={t("launch.vaults")} hint={t("launch.vaultsHint")}>
            {vaultCreds.length ? (
              <div className="flex flex-col gap-2 rounded-lg border border-border bg-surface-2/40 p-2.5">
                {vaultCreds.map((c) => (
                  <Checkbox key={c.id} checked={vaults.includes(c.id)} onChange={() => toggleVault(c.id)} label={c.name} />
                ))}
              </div>
            ) : (
              <p className="text-xs text-ink-faint">{t("launch.noVaults")}</p>
            )}
          </Field>
        )}
        {error && <ErrorText>{error}</ErrorText>}
      </div>
    </Modal>
  );
}
