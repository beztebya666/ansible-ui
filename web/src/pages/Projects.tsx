import { useState } from "react";
import { Link } from "react-router-dom";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Download, FolderGit2, Pencil, Plus, Trash2, Upload } from "lucide-react";
import { Page, PageHeader } from "../components/PageHeader";
import { ConfirmDialog, EmptyState, ErrorText, Field, Modal, Spinner } from "../components/ui";
import { useToast } from "../components/feedback";
import { Select } from "../components/Select";
import { LaunchButton } from "../components/Launcher";
import { api, type Project } from "../lib/api";
import { fmtRelative } from "../lib/format";
import { usePrefs } from "../lib/prefs";

export function Projects() {
  const { t } = usePrefs();
  const qc = useQueryClient();
  const toast = useToast();
  const projects = useQuery({ queryKey: ["projects"], queryFn: api.listProjects });
  const [creating, setCreating] = useState(false);

  const pickImport = () => {
    const inp = document.createElement("input");
    inp.type = "file";
    inp.accept = "application/json,.json";
    inp.onchange = async () => {
      const f = inp.files?.[0];
      if (!f) return;
      try {
        const bundle = JSON.parse(await f.text());
        const p = await api.importProject(bundle);
        qc.invalidateQueries({ queryKey: ["projects"] });
        toast.success(t("proj.imported").replace("{name}", p.name));
      } catch (e) {
        toast.error((e as Error).message);
      }
    };
    inp.click();
  };

  return (
    <>
      <PageHeader
        title={t("proj.title")}
        subtitle={t("proj.subtitle")}
        actions={
          <div className="flex gap-2">
            <button className="btn-outline" onClick={pickImport}>
              <Upload size={15} /> {t("proj.import")}
            </button>
            <button className="btn-outline" onClick={() => setCreating(true)}>
              <Plus size={15} /> {t("proj.new")}
            </button>
          </div>
        }
      />
      <Page>
        {projects.isLoading ? (
          <Loading />
        ) : projects.data?.length ? (
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {projects.data.map((p) => (
              <ProjectCard key={p.id} project={p} />
            ))}
          </div>
        ) : (
          <EmptyState
            icon={<FolderGit2 size={28} />}
            title={t("proj.none")}
            description={t("proj.noneHint")}
            action={
              <button className="btn-primary" onClick={() => setCreating(true)}>
                <Plus size={15} /> {t("proj.new")}
              </button>
            }
          />
        )}
      </Page>
      {creating && <CreateProject onClose={() => setCreating(false)} />}
    </>
  );
}

function ProjectCard({ project }: { project: Project }) {
  const { t, lang } = usePrefs();
  const qc = useQueryClient();
  const toast = useToast();
  const [editing, setEditing] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const del = useMutation({
    mutationFn: () => api.deleteProject(project.id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["projects"] });
      setConfirming(false);
    },
  });
  const exportProject = async () => {
    try {
      const bundle = await api.exportProject(project.id);
      const url = URL.createObjectURL(new Blob([JSON.stringify(bundle, null, 2)], { type: "application/json" }));
      const a = document.createElement("a");
      a.href = url;
      a.download = `project-${project.slug}.json`;
      a.click();
      URL.revokeObjectURL(url);
    } catch (e) {
      toast.error((e as Error).message);
    }
  };

  return (
    <div className="card group flex flex-col p-4 transition-colors hover:border-border-strong">
      <div className="flex items-start justify-between gap-2">
        <Link to={`/projects/${project.id}`} className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <FolderGit2 size={16} className="shrink-0 text-accent" />
            <h3 className="truncate font-medium text-ink">{project.name}</h3>
          </div>
          <p className="mt-1.5 line-clamp-2 min-h-[2.5rem] text-sm text-ink-dim">
            {project.description || <span className="text-ink-faint/60">{t("proj.noDescription")}</span>}
          </p>
        </Link>
        <div className="-mr-1.5 -mt-1 flex shrink-0 gap-0.5 opacity-0 transition-opacity group-hover:opacity-100">
          <button
            onClick={exportProject}
            className="btn-ghost p-1.5 text-ink-faint hover:text-ink"
            title={t("proj.export")}
          >
            <Download size={15} />
          </button>
          <button
            onClick={() => setEditing(true)}
            className="btn-ghost p-1.5 text-ink-faint hover:text-ink"
            title={t("common.edit")}
          >
            <Pencil size={15} />
          </button>
          <button
            onClick={() => setConfirming(true)}
            className="btn-ghost p-1.5 text-danger hover:bg-danger/10 hover:text-danger"
            title={t("common.delete")}
          >
            <Trash2 size={15} />
          </button>
        </div>
      </div>
      <div className="mt-4 flex items-center justify-between border-t border-border pt-3 text-xs text-ink-faint">
        <span className="chip">{project.slug}</span>
        <span>{t("proj.updated")} {fmtRelative(project.updatedAt, lang)}</span>
      </div>
      <div className="mt-3 flex gap-2">
        <Link to={`/projects/${project.id}`} className="btn-outline flex-1">
          {t("proj.open")}
        </Link>
        <LaunchButton prefill={{ projectId: project.id }} label={t("common.run")} />
      </div>

      {editing && <EditProject project={project} onClose={() => setEditing(false)} />}
      <ConfirmDialog
        open={confirming}
        title={t("proj.deleteTitle")}
        body={t("proj.deleteConfirm").replace("{name}", project.name)}
        confirmLabel={t("common.delete")}
        cancelLabel={t("common.cancel")}
        busy={del.isPending}
        onConfirm={() => del.mutate()}
        onClose={() => setConfirming(false)}
      />
    </div>
  );
}

function EditProject({ project, onClose }: { project: Project; onClose: () => void }) {
  const { t } = usePrefs();
  const qc = useQueryClient();
  const [name, setName] = useState(project.name);
  const [description, setDescription] = useState(project.description ?? "");
  const [slug, setSlug] = useState(project.slug);
  const [error, setError] = useState("");

  const save = useMutation({
    mutationFn: () => api.updateProject(project.id, { name, description, slug }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["projects"] });
      qc.invalidateQueries({ queryKey: ["project", project.id] });
      onClose();
    },
    onError: (e) => setError((e as Error).message),
  });

  return (
    <Modal
      open
      onClose={onClose}
      title={t("proj.editTitle")}
      footer={
        <>
          <button className="btn-outline" onClick={onClose}>{t("common.cancel")}</button>
          <button className="btn-primary" onClick={() => { setError(""); save.mutate(); }} disabled={!name.trim() || save.isPending}>
            {save.isPending ? <Spinner /> : null} {t("common.save")}
          </button>
        </>
      }
    >
      <div className="space-y-4">
        <Field label={t("proj.name")}>
          <input className="input" value={name} onChange={(e) => setName(e.target.value)} autoFocus />
        </Field>
        <Field label={t("proj.tag")} hint={t("proj.tagHint")}>
          <input className="input font-mono" value={slug} onChange={(e) => setSlug(e.target.value)} placeholder="my-project" />
        </Field>
        <Field label={t("proj.description")}>
          <input className="input" value={description} onChange={(e) => setDescription(e.target.value)} placeholder={t("common.optional")} />
        </Field>
        {error && <ErrorText>{error}</ErrorText>}
      </div>
    </Modal>
  );
}

function flattenDirs(nodes: { type: string; path: string; children?: unknown[] }[] | undefined, out: string[] = []): string[] {
  for (const n of nodes ?? []) {
    if (n.type === "dir") {
      out.push(n.path);
      flattenDirs(n.children as typeof nodes, out);
    }
  }
  return out;
}

function CreateProject({ onClose }: { onClose: () => void }) {
  const { t } = usePrefs();
  const qc = useQueryClient();
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [source, setSource] = useState<"local" | "repo">("local");
  const [repositoryId, setRepositoryId] = useState("");
  const [subPath, setSubPath] = useState("");
  const [error, setError] = useState("");

  const repos = useQuery({ queryKey: ["repositories"], queryFn: api.listRepositories });
  const tree = useQuery({
    queryKey: ["repoTree", repositoryId],
    queryFn: () => api.repoTree(repositoryId),
    enabled: source === "repo" && !!repositoryId,
  });
  const dirs = flattenDirs(tree.data as never);

  const create = useMutation({
    mutationFn: () =>
      source === "repo"
        ? api.createProject({ name, description, repositoryId, subPath })
        : api.createProject({ name, description }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["projects"] });
      onClose();
    },
    onError: (e) => setError((e as Error).message),
  });

  const repoReady = repos.data?.find((r) => r.id === repositoryId)?.status === "ready";

  return (
    <Modal
      open
      onClose={onClose}
      title={t("proj.new")}
      subtitle={t("proj.createSubtitle")}
      footer={
        <>
          <button className="btn-outline" onClick={onClose}>{t("common.cancel")}</button>
          <button
            className="btn-primary"
            onClick={() => { setError(""); create.mutate(); }}
            disabled={!name || (source === "repo" && !repositoryId) || create.isPending}
          >
            {create.isPending ? <Spinner /> : <Plus size={15} />} {t("common.create")}
          </button>
        </>
      }
    >
      <div className="space-y-4">
        <Field label={t("proj.name")}>
          <input className="input" value={name} onChange={(e) => setName(e.target.value)} placeholder="My infrastructure" autoFocus />
        </Field>
        <Field label={t("proj.description")}>
          <input className="input" value={description} onChange={(e) => setDescription(e.target.value)} placeholder={t("common.optional")} />
        </Field>

        <Field label={t("proj.source")}>
          <Select
            value={source}
            onChange={(v) => setSource(v as "local" | "repo")}
            options={[
              { label: t("proj.sourceLocal"), value: "local" },
              { label: t("proj.sourceRepo"), value: "repo" },
            ]}
          />
        </Field>

        {source === "repo" && (
          <>
            <Field label={t("proj.repository")}>
              <Select
                value={repositoryId}
                onChange={setRepositoryId}
                placeholder={t("pd.selectRepo")}
                options={(repos.data ?? []).map((r) => ({
                  label: `${r.name} (${r.branch})${r.status !== "ready" ? t("proj.notSynced") : ""}`,
                  value: r.id,
                }))}
              />
            </Field>
            <Field label={t("proj.folder")} hint={repoReady ? "the ansible project folder within the repo" : "sync the repository to browse folders"}>
              {dirs.length > 0 ? (
                <Select
                  value={subPath}
                  onChange={setSubPath}
                  options={[{ label: "(repository root)", value: "" }, ...dirs.map((d) => ({ label: d, value: d }))]}
                />
              ) : (
                <input className="input font-mono text-xs" value={subPath} onChange={(e) => setSubPath(e.target.value)} placeholder="path/to/project" />
              )}
            </Field>
          </>
        )}
        {error && <ErrorText>{error}</ErrorText>}
      </div>
    </Modal>
  );
}

function Loading() {
  return (
    <div className="flex items-center justify-center py-20 text-ink-faint">
      <Spinner />
    </div>
  );
}
