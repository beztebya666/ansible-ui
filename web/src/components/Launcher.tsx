import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from "react";
import { useNavigate } from "react-router-dom";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Play, Rocket } from "lucide-react";
import { Field, Modal, Segmented, Toggle, ErrorText, Spinner } from "./ui";
import { Select } from "./Select";
import { api, type AppId, type LaunchRequest } from "../lib/api";
import { APP_LIST, appMeta, appText, actionsFor, defaultActionFor } from "../lib/apps";
import { parseExtraVars } from "../lib/extravars";
import { usePrefs } from "../lib/prefs";

export interface Prefill {
  projectId?: string;
  playbook?: string;
  inventoryId?: string;
}

interface LauncherCtx {
  open: (prefill?: Prefill) => void;
}
const Ctx = createContext<LauncherCtx>({ open: () => {} });
export const useLauncher = () => useContext(Ctx);

export function LauncherProvider({ children }: { children: ReactNode }) {
  const [isOpen, setOpen] = useState(false);
  const [prefill, setPrefill] = useState<Prefill>({});

  const open = useCallback((p?: Prefill) => {
    setPrefill(p ?? {});
    setOpen(true);
  }, []);

  return (
    <Ctx.Provider value={{ open }}>
      {children}
      {isOpen && <LauncherDialog prefill={prefill} onClose={() => setOpen(false)} />}
    </Ctx.Provider>
  );
}

function LauncherDialog({ prefill, onClose }: { prefill: Prefill; onClose: () => void }) {
  const { t } = usePrefs();
  const navigate = useNavigate();
  const qc = useQueryClient();

  const projects = useQuery({ queryKey: ["projects"], queryFn: api.listProjects });
  const [projectId, setProjectId] = useState(prefill.projectId ?? "");
  const [app, setApp] = useState<AppId>("ansible");
  const [action, setAction] = useState("plan");
  const [playbook, setPlaybook] = useState(prefill.playbook ?? "");
  const [inventoryId, setInventoryId] = useState(prefill.inventoryId ?? "");
  const [environmentId, setEnvironmentId] = useState("");
  const [limit, setLimit] = useState("");
  const [tags, setTags] = useState("");
  const [skipTags, setSkipTags] = useState("");
  const [extraVars, setExtraVars] = useState("");
  const [check, setCheck] = useState(false);
  const [diff, setDiff] = useState(true);
  const [verbosity, setVerbosity] = useState(0);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  // Default to the first project once loaded.
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
  const environments = useQuery({ queryKey: ["environments"], queryFn: api.listEnvironments });

  const meta = appMeta(app);
  const at = appText(meta, t);
  const playbooks = project.data?.playbooks ?? [];
  useEffect(() => {
    if (app === "ansible" && playbooks.length && !playbooks.includes(playbook)) setPlaybook(playbooks[0]);
  }, [app, playbooks, playbook]);
  // Keep the action valid for the selected app (Pulumi uses preview/up/destroy).
  useEffect(() => {
    if (meta.kind === "infra" && !actionsFor(app).includes(action)) setAction(defaultActionFor(app));
  }, [app]); // eslint-disable-line react-hooks/exhaustive-deps

  const isAnsible = app === "ansible";
  const isInfra = meta.kind === "infra";

  const launch = async () => {
    setError("");
    const parsed = parseExtraVars(extraVars);
    if (parsed.error) {
      setError(parsed.error);
      return;
    }
    const entry = isInfra ? (playbook.trim() || ".") : playbook.trim();
    if (!projectId || (!isInfra && !entry)) {
      setError(t("launch.pickEntry").replace("{x}", at.entryLabel.toLowerCase()));
      return;
    }
    const body: LaunchRequest = {
      projectId,
      app,
      action: isInfra ? action : undefined,
      playbook: entry,
      inventoryId: isAnsible ? inventoryId || undefined : undefined,
      environmentId: environmentId || undefined,
      limit: isAnsible ? limit || undefined : undefined,
      tags: isAnsible ? tags || undefined : undefined,
      skipTags: isAnsible ? skipTags || undefined : undefined,
      extraVars: parsed.vars,
      check: isAnsible ? check : undefined,
      diff: isAnsible ? diff : undefined,
      verbosity: isAnsible ? verbosity : undefined,
    };
    setBusy(true);
    try {
      const run = await api.createRun(body);
      qc.invalidateQueries({ queryKey: ["runs"] });
      onClose();
      navigate(`/runs/${run.id}`);
    } catch (e) {
      setError((e as Error).message);
      setBusy(false);
    }
  };

  return (
    <Modal
      open
      onClose={onClose}
      wide
      title={t("launch.title")}
      subtitle={t("launch.subtitle")}
      footer={
        <>
          <button className="btn-outline" onClick={onClose} disabled={busy}>
            {t("common.cancel")}
          </button>
          <button className="btn-primary" onClick={launch} disabled={busy || !projectId || (!isInfra && !playbook)}>
            {busy ? <Spinner /> : <Rocket size={15} />}
            {t("launch.go")}
          </button>
        </>
      }
    >
      <div className="grid gap-4 sm:grid-cols-2">
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
            placeholder={projects.isLoading ? t("common.loading") : t("launch.selectProject")}
            options={(projects.data ?? []).map((p) => ({ label: p.name, value: p.id }))}
          />
        </Field>

        {isAnsible ? (
          <Field label={at.entryLabel} hint={at.entryHint}>
            <Select
              value={playbook}
              onChange={setPlaybook}
              disabled={!playbooks.length}
              placeholder={project.isLoading ? t("common.loading") : t("launch.noPlaybooks")}
              options={playbooks.map((p) => ({ label: p, value: p }))}
            />
          </Field>
        ) : (
          <Field label={at.entryLabel} hint={at.entryHint}>
            <input
              className="input font-mono text-xs"
              value={playbook}
              onChange={(e) => setPlaybook(e.target.value)}
              placeholder={meta.entryPlaceholder}
            />
          </Field>
        )}

        {isInfra && (
          <Field label={t("run.action")} hint={t("launch.actionHint")}>
            <Select value={action} onChange={setAction} options={actionsFor(app).map((a) => ({ label: a, value: a }))} />
          </Field>
        )}

        {isAnsible && (
          <Field label={t("pd.inventories")} hint={t("launch.inventoryHint")}>
            <Select
              value={inventoryId}
              onChange={setInventoryId}
              options={[{ label: t("launch.projectDefault"), value: "" }, ...(inventories.data ?? []).map((inv) => ({ label: inv.name, value: inv.id }))]}
            />
          </Field>
        )}

        <Field label={t("launch.environment")} hint={t("launch.envHint")}>
          <Select
            value={environmentId}
            onChange={setEnvironmentId}
            options={[{ label: t("common.none"), value: "" }, ...(environments.data ?? []).map((en) => ({ label: en.name, value: en.id }))]}
          />
        </Field>

        {isAnsible && (
          <>
            <Field label={t("run.limit")} hint={t("launch.limitHint")}>
              <input className="input" value={limit} onChange={(e) => setLimit(e.target.value)} placeholder="all" />
            </Field>
            <Field label={t("run.tags")} hint={t("launch.tagsHint")}>
              <input className="input" value={tags} onChange={(e) => setTags(e.target.value)} placeholder="" />
            </Field>
            <Field label={t("run.skipTags")} hint={t("launch.skipTagsHint")}>
              <input className="input" value={skipTags} onChange={(e) => setSkipTags(e.target.value)} placeholder="" />
            </Field>
          </>
        )}

        <div className="sm:col-span-2">
          <Field label={at.varsLabel} hint={at.varsHint}>
            <textarea
              className="input min-h-[72px] font-mono text-xs"
              value={extraVars}
              onChange={(e) => setExtraVars(e.target.value)}
              placeholder={'app_version: 2.0.0\nor: {"app_version": "2.0.0"}'}
            />
          </Field>
        </div>

        {isAnsible && (
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

        {error && (
          <div className="sm:col-span-2">
            <ErrorText>{error}</ErrorText>
          </div>
        )}
      </div>
    </Modal>
  );
}

export function LaunchButton({ prefill, label }: { prefill?: Prefill; label?: string }) {
  const { t } = usePrefs();
  const { open } = useLauncher();
  return (
    <button className="btn-primary" onClick={() => open(prefill)}>
      <Play size={15} />
      {label ?? t("common.newRun")}
    </button>
  );
}
