import { useCallback, useEffect, useMemo, useState } from "react";
import { Link, useParams, useSearchParams } from "react-router-dom";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import clsx from "clsx";
import {
  ArrowLeft,
  ChevronRight,
  ExternalLink,
  File as FileIcon,
  FileCode2,
  Folder,
  GitBranch,
  HardDrive,
  Maximize2,
  Minimize2,
  Network,
  Pencil,
  Play,
  Plus,
  Save,
  Server,
  Shield,
  Trash2,
  Upload,
  Users,
} from "lucide-react";
import { PageHeader } from "../components/PageHeader";
import { RunRow } from "../components/RunRow";
import { Checkbox, EmptyState, Field, Modal, SearchBox, Spinner, ErrorText } from "../components/ui";
import { useConfirm, usePrompt, useToast } from "../components/feedback";
import { CodeEditor, langFromPath } from "../components/CodeEditor";
import { Select } from "../components/Select";
import { LaunchButton, useLauncher } from "../components/Launcher";
import { api, type FileNode, type Inventory } from "../lib/api";
import { usePrefs } from "../lib/prefs";
import { useAuth } from "../lib/auth";
import { fmtRelative } from "../lib/format";

type Tab = "playbooks" | "files" | "inventories" | "runs" | "members";

const TABS: Tab[] = ["playbooks", "files", "inventories", "runs", "members"];

export function ProjectDetail() {
  const { t } = usePrefs();
  const { id = "" } = useParams();
  // Deep-link support: /projects/:id?tab=files&file=build.yml opens that file
  // in the editor (used by the "Playbook" link on the run-detail page).
  const [searchParams] = useSearchParams();
  const paramTab = searchParams.get("tab") as Tab | null;
  const [tab, setTab] = useState<Tab>(paramTab && TABS.includes(paramTab) ? paramTab : "playbooks");
  // Set when a playbook row is clicked (or via ?file=): jump to Files and open it.
  const [openPath, setOpenPath] = useState<string | null>(searchParams.get("file"));
  const clearOpenPath = useCallback(() => setOpenPath(null), []);
  const openInEditor = useCallback((path: string) => {
    setOpenPath(path);
    setTab("files");
  }, []);
  const project = useQuery({ queryKey: ["project", id], queryFn: () => api.getProject(id) });

  if (project.isLoading || !project.data) {
    return (
      <div className="flex h-64 items-center justify-center text-ink-faint">
        <Spinner />
      </div>
    );
  }
  const p = project.data;

  return (
    <>
      <PageHeader
        title={p.name}
        subtitle={p.description || p.slug}
        actions={
          <>
            <Link to="/projects" className="btn-ghost">
              <ArrowLeft size={15} /> {t("pd.allProjects")}
            </Link>
            <LaunchButton prefill={{ projectId: id }} />
          </>
        }
      />
      <div className="mx-auto w-full max-w-6xl px-6">
        <div className="flex gap-1 border-b border-border">
          {(["playbooks", "files", "inventories", "runs", "members"] as Tab[]).map((tb) => (
            <button
              key={tb}
              onClick={() => setTab(tb)}
              className={clsx(
                "relative px-3 py-2.5 text-sm font-medium transition-colors",
                tab === tb ? "text-ink" : "text-ink-faint hover:text-ink-dim",
              )}
            >
              {t(`pd.${tb}`)}
              {tab === tb && <span className="absolute inset-x-2 -bottom-px h-0.5 rounded-full bg-accent" />}
            </button>
          ))}
        </div>
        <div className="py-5">
          {tab === "playbooks" && <Playbooks projectId={id} playbooks={p.playbooks ?? []} onOpen={openInEditor} />}
          {tab === "files" && <Files projectId={id} openPath={openPath} onOpened={clearOpenPath} />}
          {tab === "inventories" && <Inventories projectId={id} />}
          {tab === "runs" && <ProjectRuns projectId={id} />}
          {tab === "members" && <Members projectId={id} myRole={p.myRole} />}
        </div>
      </div>
    </>
  );
}

function Playbooks({
  projectId,
  playbooks,
  onOpen,
}: {
  projectId: string;
  playbooks: string[];
  onOpen: (path: string) => void;
}) {
  const { t } = usePrefs();
  const { open } = useLauncher();
  const [q, setQ] = useState("");
  if (!playbooks.length) {
    return (
      <EmptyState
        icon={<FileCode2 size={28} />}
        title={t("pd.noPlaybooks")}
        description={t("pd.noPlaybooksHint")}
      />
    );
  }
  const ql = q.trim().toLowerCase();
  const shown = ql ? playbooks.filter((p) => p.toLowerCase().includes(ql)) : playbooks;
  return (
    <div className="space-y-3">
      {playbooks.length > 6 && (
        <SearchBox value={q} onChange={setQ} placeholder={t("pd.searchPlaybooks")} />
      )}
      {shown.length === 0 ? (
        <div className="py-8 text-center text-sm text-ink-faint">{t("pd.noMatch")}</div>
      ) : (
        <div className="card divide-y divide-border">
          {shown.map((pb) => (
            <div key={pb} className="group flex items-center gap-3 px-4 py-3 transition-colors hover:bg-white/[0.03]">
              {/* Click the name → open it in the file tree / editor. */}
              <button
                type="button"
                onClick={() => onOpen(pb)}
                className="flex min-w-0 flex-1 items-center gap-3 text-left"
                title={t("pd.openInEditor")}
              >
                <FileCode2 size={16} className="shrink-0 text-accent" />
                <span className="truncate font-mono text-sm text-ink transition-colors group-hover:text-accent">{pb}</span>
              </button>
              <button className="btn-outline px-3 py-1.5" onClick={() => open({ projectId, playbook: pb })} title={t("pd.configureLaunch")}>
                <Play size={14} /> {t("common.run")}
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function Files({
  projectId,
  openPath,
  onOpened,
}: {
  projectId: string;
  openPath?: string | null;
  onOpened?: () => void;
}) {
  const { t } = usePrefs();
  const qc = useQueryClient();
  const tree = useQuery({ queryKey: ["files", projectId], queryFn: () => api.projectFiles(projectId) });
  const project = useQuery({ queryKey: ["project", projectId], queryFn: () => api.getProject(projectId) });
  const isLocal = (project.data?.sourceType ?? "local") !== "git";
  const [selected, setSelected] = useState<string | null>(null);
  const [content, setContent] = useState("");
  const [original, setOriginal] = useState("");
  const [fullscreen, setFullscreen] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [fileQ, setFileQ] = useState("");
  const [commitOpen, setCommitOpen] = useState(false);
  const [staged, setStaged] = useState<StagedFile[]>([]);
  const [commitFiles, setCommitFiles] = useState<StagedFile[]>([]);
  const toast = useToast();
  const prompt = usePrompt();
  const confirm = useConfirm();
  const refreshFiles = () => {
    qc.invalidateQueries({ queryKey: ["files", projectId] });
    qc.invalidateQueries({ queryKey: ["playbooks", projectId] });
    qc.invalidateQueries({ queryKey: ["project", projectId] });
  };
  const doRename = async (path: string, name: string) => {
    const next = (await prompt({ title: t("pd.rename"), defaultValue: name, placeholder: name }))?.trim();
    if (!next || next === name) return;
    const dir = path.includes("/") ? path.slice(0, path.lastIndexOf("/") + 1) : "";
    const to = dir + next;
    try {
      await api.renameFile(projectId, path, to);
      if (selected === path) setSelected(to);
      refreshFiles();
    } catch (e) {
      toast.error((e as Error).message);
    }
  };
  const doDelete = async (path: string, isDir: boolean) => {
    if (!(await confirm({ title: isDir ? t("pd.deleteFolder") : t("pd.deleteFile"), body: path }))) return;
    try {
      await api.deleteFile(projectId, path);
      if (selected === path || (isDir && selected?.startsWith(path + "/"))) setSelected(null);
      refreshFiles();
    } catch (e) {
      toast.error((e as Error).message);
    }
  };
  const upload = async (file: File, password?: string) => {
    setUploading(true);
    try {
      const r = await api.uploadArchive(projectId, file, password);
      // Live update — no manual refresh: refetch the tree, playbooks and project.
      qc.invalidateQueries({ queryKey: ["files", projectId] });
      qc.invalidateQueries({ queryKey: ["project", projectId] });
      qc.invalidateQueries({ queryKey: ["playbooks", projectId] });
      toast.success(t("pd.uploaded").replace("{n}", String(r.extracted)));
      setUploading(false);
    } catch (e) {
      const err = e as Error & { encrypted?: boolean };
      setUploading(false);
      // Password-protected zip → ask for the password and retry (Enterprise: handle it).
      if (err.encrypted) {
        const pw = await prompt({ title: t("pd.archivePassword"), placeholder: t("pd.archivePasswordPh"), confirmLabel: t("pd.upload") });
        if (pw) upload(file, pw);
        return;
      }
      toast.error(err.message);
    }
  };
  // Esc exits the fullscreen editor (mirrors the run console).
  useEffect(() => {
    if (!fullscreen) return;
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && setFullscreen(false);
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [fullscreen]);
  // Dirty = content differs from what's on disk. Computed (not a sticky flag) so
  // undoing edits back to the original clears the unsaved ● marker.
  const dirty = content !== original;

  // Git staging cart: the current edit plus anything already staged, so several
  // files can be committed together onto one branch.
  const stagedHasCurrent = !!selected && staged.some((f) => f.path === selected);
  const pendingFiles: StagedFile[] = [
    ...staged,
    ...(dirty && selected && !stagedHasCurrent ? [{ path: selected, content, original }] : []),
  ];
  const stage = () => {
    if (!dirty || !selected) return;
    setStaged((s) => [...s.filter((f) => f.path !== selected), { path: selected, content, original }]);
    setOriginal(content); // staged → the editor is no longer "dirty"
  };
  const openCommit = () => {
    setCommitFiles(pendingFiles);
    setCommitOpen(true);
  };

  // A playbook row asked to open a specific file: select it, then clear the request.
  useEffect(() => {
    if (openPath) {
      setSelected(openPath);
      onOpened?.();
    }
  }, [openPath, onOpened]);

  const file = useQuery({
    queryKey: ["file", projectId, selected],
    queryFn: () => api.readFile(projectId, selected!),
    enabled: !!selected,
  });
  useEffect(() => {
    if (file.data) {
      setContent(file.data.content);
      setOriginal(file.data.content);
    }
  }, [file.data]);

  const save = useMutation({
    mutationFn: () => api.writeFile(projectId, selected!, content),
    onSuccess: () => {
      setOriginal(content); // now matches disk → no longer dirty
      qc.invalidateQueries({ queryKey: ["files", projectId] });
      qc.invalidateQueries({ queryKey: ["project", projectId] });
    },
  });

  return (
    <div className="grid gap-4 lg:grid-cols-[280px_1fr]">
      <div className="card flex max-h-[60vh] flex-col p-2">
        <div className="mb-1.5 flex items-center gap-1.5">
          <div className="min-w-0 flex-1">
            <SearchBox value={fileQ} onChange={setFileQ} placeholder={t("pd.searchFiles")} />
          </div>
          {isLocal && (
            <label className="btn-ghost shrink-0 cursor-pointer px-2 py-1.5 text-xs" title={t("pd.uploadHint")}>
              {uploading ? <Spinner /> : <Upload size={13} />}
              <input
                type="file"
                accept=".zip,.tar,.tgz,.gz,.tar.gz"
                className="hidden"
                onChange={(e) => {
                  const f = e.target.files?.[0];
                  if (f) upload(f);
                  e.target.value = "";
                }}
              />
            </label>
          )}
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto">
          {tree.isLoading ? (
            <div className="flex justify-center py-8 text-ink-faint"><Spinner /></div>
          ) : (
            (() => {
              const filtered = filterTree(tree.data ?? [], fileQ.trim().toLowerCase());
              if (fileQ.trim() && filtered.length === 0)
                return <div className="py-6 text-center text-sm text-ink-faint">{t("pd.noMatch")}</div>;
              return <Tree nodes={filtered} selected={selected} onSelect={setSelected} onRename={doRename} onDelete={doDelete} forceOpen={!!fileQ.trim()} />;
            })()
          )}
        </div>
      </div>
      <div className={`card flex flex-col overflow-hidden ${fullscreen ? "fixed inset-0 z-50 rounded-none" : "min-h-[60vh]"}`}>
        {selected ? (
          <>
            <div className="flex items-center justify-between border-b border-border px-4 py-2.5">
              <span className="truncate font-mono text-xs text-ink-dim">{selected}{dirty && <span className="ml-1 text-warn">●</span>}</span>
              <div className="flex items-center gap-1.5">
                <button
                  className="btn-ghost p-1.5"
                  title={fullscreen ? t("run.exitFullscreen") : t("run.fullscreen")}
                  onClick={() => setFullscreen((f) => !f)}
                >
                  {fullscreen ? <Minimize2 size={15} /> : <Maximize2 size={15} />}
                </button>
                {isLocal ? (
                  <button className="btn-primary px-3 py-1.5" onClick={() => save.mutate()} disabled={!dirty || save.isPending}>
                    {save.isPending ? <Spinner /> : <Save size={14} />} {t("common.save")}
                  </button>
                ) : (
                  <>
                    <button
                      className="btn-ghost px-2 py-1.5 text-xs"
                      onClick={stage}
                      disabled={!dirty}
                      title={t("pd.stageHint")}
                    >
                      <Plus size={13} /> {t("pd.stage")}{staged.length > 0 ? ` · ${staged.length}` : ""}
                    </button>
                    <button
                      className="btn-primary px-3 py-1.5"
                      onClick={openCommit}
                      disabled={pendingFiles.length === 0}
                      title={t("pd.commitHint")}
                    >
                      <GitBranch size={14} /> {t("pd.commitPush")}{pendingFiles.length > 1 ? ` (${pendingFiles.length})` : ""}
                    </button>
                  </>
                )}
              </div>
            </div>
            {file.isLoading ? (
              <div className="flex flex-1 items-center justify-center text-ink-faint"><Spinner /></div>
            ) : (
              <div className="min-h-0 flex-1 overflow-hidden bg-bg px-1">
                <CodeEditor
                  value={content}
                  language={langFromPath(selected)}
                  onChange={setContent}
                  className="h-full"
                />
              </div>
            )}
          </>
        ) : (
          <div className="flex flex-1 items-center justify-center">
            <EmptyState icon={<FileIcon size={28} />} title={t("pd.selectFile")} description={t("pd.selectFileHint")} />
          </div>
        )}
      </div>
      {!isLocal && <BranchesPanel projectId={projectId} />}
      {commitOpen && commitFiles.length > 0 && (
        <GitCommitDialog
          projectId={projectId}
          files={commitFiles}
          onClose={() => setCommitOpen(false)}
          onCommitted={() => {
            setStaged([]);
            setOriginal(content);
            qc.invalidateQueries({ queryKey: ["branches", projectId] });
            refreshFiles();
          }}
        />
      )}
    </div>
  );
}

// BranchesPanel lists the branches the UI committed + pushed for a git-backed
// project, each with a one-click PR/MR link.
function BranchesPanel({ projectId }: { projectId: string }) {
  const { t, lang } = usePrefs();
  const q = useQuery({ queryKey: ["branches", projectId], queryFn: () => api.listBranches(projectId) });
  const branches = q.data ?? [];
  if (!branches.length) return null;
  return (
    <div className="card p-3.5 lg:col-span-2">
      <div className="mb-2 flex items-center gap-1.5 text-sm font-medium text-ink">
        <GitBranch size={14} /> {t("pd.pushedBranches")}
      </div>
      <div className="divide-y divide-border">
        {branches.map((b) => (
          <div key={b.id} className="flex items-center gap-2 py-2 text-xs">
            <span className="chip shrink-0 font-mono">{b.branch}</span>
            <span className="truncate font-mono text-ink-faint">{b.files.join(", ")}</span>
            <span className="ml-auto shrink-0 text-ink-faint">{b.actor} · {fmtRelative(b.createdAt, lang)}</span>
            {b.prUrl && (
              <a href={b.prUrl} target="_blank" rel="noreferrer" className="btn-ghost shrink-0 p-1" title={t("pd.openPr")}>
                <ExternalLink size={13} />
              </a>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}

type StagedFile = { path: string; content: string; original: string };

// lineDiff is a small LCS line diff (old → new) for the pre-commit review.
function lineDiff(a: string, b: string): { type: "add" | "del" | "ctx"; text: string }[] {
  const aL = a.split("\n"), bL = b.split("\n");
  const m = aL.length, n = bL.length;
  const dp: number[][] = Array.from({ length: m + 1 }, () => new Array(n + 1).fill(0));
  for (let i = m - 1; i >= 0; i--)
    for (let j = n - 1; j >= 0; j--)
      dp[i][j] = aL[i] === bL[j] ? dp[i + 1][j + 1] + 1 : Math.max(dp[i + 1][j], dp[i][j + 1]);
  const out: { type: "add" | "del" | "ctx"; text: string }[] = [];
  let i = 0, j = 0;
  while (i < m && j < n) {
    if (aL[i] === bL[j]) { out.push({ type: "ctx", text: aL[i] }); i++; j++; }
    else if (dp[i + 1][j] >= dp[i][j + 1]) { out.push({ type: "del", text: aL[i] }); i++; }
    else { out.push({ type: "add", text: bL[j] }); j++; }
  }
  while (i < m) out.push({ type: "del", text: aL[i++] });
  while (j < n) out.push({ type: "add", text: bL[j++] });
  return out;
}

function DiffView({ file }: { file: StagedFile }) {
  const { t } = usePrefs();
  const lines = lineDiff(file.original, file.content);
  if (!lines.some((l) => l.type !== "ctx")) {
    return <p className="px-2 py-1 text-xs text-ink-faint">{t("pd.noChanges")}</p>;
  }
  return (
    <pre className="max-h-64 overflow-auto rounded-lg border border-border bg-surface p-2 font-mono text-[11px] leading-snug">
      {lines.map((l, i) => (
        <div
          key={i}
          className={clsx(
            "whitespace-pre-wrap",
            l.type === "add" && "bg-success/15 text-success",
            l.type === "del" && "bg-danger/15 text-danger",
            l.type === "ctx" && "text-ink-faint",
          )}
        >
          {l.type === "add" ? "+ " : l.type === "del" ? "- " : "  "}
          {l.text}
        </div>
      ))}
    </pre>
  );
}

// GitCommitDialog commits one or more edited files from a git-backed project onto
// a NEW branch and pushes it — master is usually protected, so we never touch the
// base branch. It shows a per-file diff before committing, then surfaces the branch
// + a one-click "open a PR/MR" link.
function GitCommitDialog({
  projectId,
  files,
  onClose,
  onCommitted,
}: {
  projectId: string;
  files: StagedFile[];
  onClose: () => void;
  onCommitted: () => void;
}) {
  const { t } = usePrefs();
  const first = files[0]?.path ?? "file";
  const base = (first.split("/").pop() || "file").replace(/\.[^.]+$/, "");
  const [message, setMessage] = useState(
    files.length === 1 ? `Update ${first.split("/").pop()}` : `Update ${files.length} files`,
  );
  const [branch, setBranch] = useState(`aui/edit-${base}-${Date.now().toString(36)}`);
  const [expanded, setExpanded] = useState<string | null>(files.length === 1 ? first : null);
  const [result, setResult] = useState<{ branch: string; prUrl: string; commit: string } | null>(null);
  const commit = useMutation({
    mutationFn: () =>
      api.gitCommit(projectId, {
        files: files.map((f) => ({ path: f.path, content: f.content })),
        message,
        branch,
      }),
    onSuccess: (r) => {
      setResult(r);
      onCommitted();
    },
  });
  return (
    <Modal
      open
      wide
      onClose={onClose}
      title={t("pd.commitPush")}
      subtitle={files.length === 1 ? first : t("pd.nFiles").replace("{n}", String(files.length))}
      footer={
        result ? (
          <button className="btn-primary" onClick={onClose}>{t("common.done")}</button>
        ) : (
          <>
            <button className="btn-outline" onClick={onClose}>{t("common.cancel")}</button>
            <button
              className="btn-primary"
              disabled={!message.trim() || !branch.trim() || commit.isPending}
              onClick={() => commit.mutate()}
            >
              {commit.isPending ? <Spinner /> : <GitBranch size={14} />} {t("pd.commitPush")}
            </button>
          </>
        )
      }
    >
      {result ? (
        <div className="space-y-3">
          <p className="text-sm text-ink">
            {t("pd.committedTo")} <span className="chip font-mono">{result.branch}</span>
          </p>
          <p className="font-mono text-xs text-ink-faint">{result.commit.slice(0, 10)}</p>
          {result.prUrl && (
            <a href={result.prUrl} target="_blank" rel="noreferrer" className="btn-outline inline-flex w-fit gap-1.5 text-sm">
              <ExternalLink size={14} /> {t("pd.openPr")}
            </a>
          )}
        </div>
      ) : (
        <div className="space-y-3">
          <p className="text-xs text-ink-dim">{t("pd.commitDesc")}</p>
          {commit.error && <ErrorText>{(commit.error as Error).message}</ErrorText>}
          <div className="rounded-lg border border-border">
            {files.map((f) => (
              <div key={f.path} className="border-b border-border last:border-0">
                <button
                  className="flex w-full items-center gap-1.5 px-2.5 py-1.5 text-left text-xs hover:bg-surface-2/40"
                  onClick={() => setExpanded(expanded === f.path ? null : f.path)}
                >
                  <ChevronRight size={12} className={clsx("transition-transform", expanded === f.path && "rotate-90")} />
                  <span className="font-mono text-ink-dim">{f.path}</span>
                </button>
                {expanded === f.path && <div className="px-2.5 pb-2.5"><DiffView file={f} /></div>}
              </div>
            ))}
          </div>
          <Field label={t("pd.commitMessage")}>
            <input className="input" value={message} onChange={(e) => setMessage(e.target.value)} />
          </Field>
          <Field label={t("pd.branch")} hint={t("pd.branchHint2")}>
            <input className="input font-mono text-xs" value={branch} onChange={(e) => setBranch(e.target.value)} />
          </Field>
        </div>
      )}
    </Modal>
  );
}

// A distinguishable inventory name from a file path: the parent folder + file
// name (last two segments), so `inventories/dev/inv-k8s.yml` → "dev/inv-k8s.yml"
// and several `inv-k8s.yml`s in different folders stay tellable apart in the
// launcher's inventory dropdown.
function invNameFromPath(path: string): string {
  const parts = path.split("/").filter(Boolean);
  return parts.slice(-2).join("/") || path;
}

// Flatten the file tree to a list of file paths (for the inventory file picker).
function flattenFiles(nodes: FileNode[], out: string[] = []): string[] {
  for (const n of nodes) {
    if (n.type === "dir") {
      if (n.children) flattenFiles(n.children, out);
    } else {
      out.push(n.path);
    }
  }
  return out;
}

// Prune the tree to entries matching `q` (keeping ancestor folders of matches).
function filterTree(nodes: FileNode[], q: string): FileNode[] {
  if (!q) return nodes;
  const out: FileNode[] = [];
  for (const n of nodes) {
    if (n.type === "dir") {
      const kids = n.children ? filterTree(n.children, q) : [];
      if (kids.length > 0 || n.name.toLowerCase().includes(q)) out.push({ ...n, children: kids });
    } else if (n.path.toLowerCase().includes(q)) {
      out.push(n);
    }
  }
  return out;
}

interface TreeProps {
  nodes: FileNode[];
  selected: string | null;
  onSelect: (p: string) => void;
  onRename: (path: string, name: string) => void;
  onDelete: (path: string, isDir: boolean) => void;
  forceOpen?: boolean;
  depth?: number;
}

function Tree({ nodes, depth = 0, ...rest }: TreeProps) {
  return (
    <ul className={depth === 0 ? "" : "ml-3 border-l border-border/60"}>
      {nodes.map((n) => (
        <TreeNode key={n.path} node={n} depth={depth} {...rest} />
      ))}
    </ul>
  );
}

// Hover-revealed rename / delete actions on a tree row — minimal, no layout shift.
function RowActions({ onRename, onDelete }: { onRename: () => void; onDelete: () => void }) {
  const { t } = usePrefs();
  return (
    <span className="flex shrink-0 items-center gap-0.5 pr-0.5 opacity-0 transition-opacity group-hover:opacity-100 focus-within:opacity-100">
      <button title={t("pd.rename")} onClick={(e) => { e.stopPropagation(); onRename(); }} className="rounded p-1 text-ink-faint hover:text-ink">
        <Pencil size={12} />
      </button>
      <button title={t("common.delete")} onClick={(e) => { e.stopPropagation(); onDelete(); }} className="rounded p-1 text-ink-faint hover:text-danger">
        <Trash2 size={12} />
      </button>
    </span>
  );
}

function TreeNode({ node, selected, onSelect, onRename, onDelete, forceOpen, depth }: Omit<TreeProps, "nodes"> & { node: FileNode; depth: number }) {
  const [open, setOpen] = useState(depth < 1);
  const expanded = forceOpen || open; // a search auto-expands matching folders
  if (node.type === "dir") {
    return (
      <li>
        <div className="group flex items-center rounded pr-0.5 text-sm text-ink-dim hover:bg-white/5">
          <button onClick={() => setOpen((o) => !o)} className="flex min-w-0 flex-1 items-center gap-1.5 px-1.5 py-1 text-left">
            <ChevronRight size={13} className={clsx("shrink-0 transition-transform", expanded && "rotate-90")} />
            <Folder size={14} className="shrink-0 text-info" />
            <span className="truncate">{node.name}</span>
          </button>
          <RowActions onRename={() => onRename(node.path, node.name)} onDelete={() => onDelete(node.path, true)} />
        </div>
        {expanded && node.children && <Tree nodes={node.children} selected={selected} onSelect={onSelect} onRename={onRename} onDelete={onDelete} forceOpen={forceOpen} depth={depth + 1} />}
      </li>
    );
  }
  return (
    <li>
      <div
        className={clsx(
          "group flex items-center rounded pl-6 pr-0.5 text-sm hover:bg-white/5",
          selected === node.path ? "bg-accent/10 text-accent" : "text-ink-dim",
        )}
      >
        <button onClick={() => onSelect(node.path)} className="flex min-w-0 flex-1 items-center gap-1.5 py-1 text-left">
          <FileIcon size={13} className="shrink-0" />
          <span className="truncate">{node.name}</span>
        </button>
        <RowActions onRename={() => onRename(node.path, node.name)} onDelete={() => onDelete(node.path, false)} />
      </div>
    </li>
  );
}

function Inventories({ projectId }: { projectId: string }) {
  const { t } = usePrefs();
  const confirm = useConfirm();
  const toast = useToast();
  const qc = useQueryClient();
  const inventories = useQuery({ queryKey: ["inventories", projectId], queryFn: () => api.listInventories(projectId) });
  const project = useQuery({ queryKey: ["project", projectId], queryFn: () => api.getProject(projectId) });
  const [editing, setEditing] = useState<Inventory | null>(null);
  const [creating, setCreating] = useState(false);
  const [hostsFor, setHostsFor] = useState<Inventory | null>(null);
  const [q, setQ] = useState("");

  const del = useMutation({
    mutationFn: (invId: string) => api.deleteInventory(invId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["inventories", projectId] }),
  });
  // Inventory files detected in the project tree that aren't already saved as a
  // file-inventory — surfaced automatically so the user doesn't re-enter them.
  const savedPaths = new Set((inventories.data ?? []).filter((i) => i.type === "file").map((i) => i.content));
  const detected = (project.data?.inventoryFiles ?? []).filter((p) => !savedPaths.has(p));
  const ql = q.trim().toLowerCase();
  const stored = (inventories.data ?? []).filter((i) => !ql || i.name.toLowerCase().includes(ql) || (i.content || "").toLowerCase().includes(ql));
  const detectedShown = detected.filter((p) => !ql || p.toLowerCase().includes(ql));
  const useDetected = useMutation({
    mutationFn: (p: string) => api.createInventory(projectId, { name: invNameFromPath(p), type: "file", content: p }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ["inventories", projectId] }); toast.success(t("pd.invAdded")); },
    onError: (e) => toast.error((e as Error).message),
  });
  const total = (inventories.data?.length ?? 0) + detected.length;

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2">
        {total > 6 && <div className="min-w-0 flex-1"><SearchBox value={q} onChange={setQ} placeholder={t("pd.searchInventories")} /></div>}
        <button className="btn-outline ml-auto shrink-0" onClick={() => setCreating(true)}>
          <Plus size={15} /> {t("pd.newInventory")}
        </button>
      </div>
      {inventories.isLoading ? (
        <div className="flex justify-center py-12 text-ink-faint"><Spinner /></div>
      ) : total === 0 ? (
        <EmptyState icon={<Server size={28} />} title={t("pd.noInventories")} description={t("pd.noInventoriesHint")} />
      ) : (
        <div className="space-y-3">
          {stored.length > 0 && (
            <div className="card divide-y divide-border">
              {stored.map((inv) => (
                <div key={inv.id} className="flex items-center gap-3 px-4 py-3">
                  <Server size={16} className="text-accent" />
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <span className="truncate text-sm text-ink">{inv.name}</span>
                      <span className="chip text-[10px] uppercase">{inv.type ?? "static"}</span>
                    </div>
                    <div className="truncate font-mono text-xs text-ink-faint">
                      {inv.type === "static"
                        ? `${inv.content.split("\n").filter((l) => l.trim() && !l.trim().startsWith("#") && !l.includes("[")).length} hosts`
                        : inv.content || "—"}
                    </div>
                  </div>
                  <button className="btn-ghost p-1.5" onClick={() => setHostsFor(inv)} title={t("pd.viewHosts")}>
                    <Network size={15} />
                  </button>
                  <button className="btn-ghost p-1.5" onClick={() => setEditing(inv)} title={t("common.edit")}>
                    <FileCode2 size={15} />
                  </button>
                  <button className="btn-ghost p-1.5" onClick={() => confirm({ title: t("pd.deleteInventory"), body: inv.name }).then((ok) => ok && del.mutate(inv.id))} title={t("common.delete")}>
                    <Trash2 size={15} />
                  </button>
                </div>
              ))}
            </div>
          )}
          {detectedShown.length > 0 && (
            <div>
              <div className="mb-1.5 px-1 text-[11px] font-semibold uppercase tracking-wide text-ink-faint">{t("pd.detectedInventories")}</div>
              <div className="card divide-y divide-border">
                {detectedShown.map((p) => (
                  <div key={p} className="group flex items-center gap-3 px-4 py-3">
                    <Server size={16} className="text-ink-faint" />
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2">
                        <span className="truncate font-mono text-sm text-ink-dim">{p}</span>
                        <span className="chip text-[10px] uppercase text-info">{t("pd.auto")}</span>
                      </div>
                    </div>
                    <button className="btn-outline px-3 py-1.5" onClick={() => useDetected.mutate(p)} disabled={useDetected.isPending} title={t("pd.addInventoryHint")}>
                      <Plus size={14} /> {t("common.add")}
                    </button>
                  </div>
                ))}
              </div>
            </div>
          )}
          {stored.length === 0 && detectedShown.length === 0 && (
            <div className="py-8 text-center text-sm text-ink-faint">{t("pd.noMatch")}</div>
          )}
        </div>
      )}
      {(creating || editing) && (
        <InventoryForm projectId={projectId} inventory={editing} onClose={() => { setCreating(false); setEditing(null); }} />
      )}
      {hostsFor && <InventoryHostsModal inventory={hostsFor} onClose={() => setHostsFor(null)} />}
    </div>
  );
}

// InventoryHostsModal resolves an inventory via ansible-inventory and shows its
// groups + hosts — a live preview of exactly what a run would target.
function InventoryHostsModal({ inventory, onClose }: { inventory: Inventory; onClose: () => void }) {
  const { t } = usePrefs();
  const toast = useToast();
  const hosts = useQuery({
    queryKey: ["inventory-hosts", inventory.id],
    queryFn: () => api.inventoryHosts(inventory.id),
    retry: false,
  });
  const gather = useMutation({
    mutationFn: () => api.gatherFacts(inventory.id),
    onSuccess: (res) => toast.success(t("pd.factsGathered").replace("{n}", String(res.count))),
    onError: (e) => toast.error((e as Error).message),
  });
  const monitor = useMutation({
    mutationFn: () => api.enableMonitor(inventory.id),
    onSuccess: (res) => toast.success(t("pd.monitorEnabled").replace("{n}", String(res.monitored))),
    onError: (e) => toast.error((e as Error).message),
  });
  const canGather = !hosts.data?.unsupported && (hosts.data?.total ?? 0) > 0;
  return (
    <Modal open onClose={onClose} title={`${t("pd.viewHosts")} · ${inventory.name}`}
      footer={canGather ? (
        <>
          <button className="btn-outline" onClick={() => monitor.mutate()} disabled={monitor.isPending}>
            {monitor.isPending ? <Spinner /> : <HardDrive size={15} />} {t("pd.enableMonitor")}
          </button>
          <button className="btn-outline" onClick={() => gather.mutate()} disabled={gather.isPending}>
            {gather.isPending ? <Spinner /> : <Network size={15} />} {t("pd.gatherFacts")}
          </button>
        </>
      ) : undefined}>
      {hosts.isLoading ? (
        <div className="flex justify-center py-10 text-ink-faint"><Spinner /></div>
      ) : hosts.isError ? (
        <ErrorText>{(hosts.error as Error).message}</ErrorText>
      ) : hosts.data?.unsupported ? (
        <div className="py-6 text-center text-sm text-ink-faint">{hosts.data.message}</div>
      ) : (hosts.data?.total ?? 0) === 0 ? (
        <div className="py-6 text-center text-sm text-ink-faint">{t("pd.noHosts")}</div>
      ) : (
        <div className="space-y-3">
          <div className="text-xs text-ink-faint">{t("pd.hostsTotal")}: <span className="font-semibold text-ink">{hosts.data!.total}</span></div>
          {hosts.data!.groups.map((g) => (
            <div key={g.name} className="rounded-md border border-border">
              <div className="flex items-center gap-2 border-b border-border bg-surface px-3 py-1.5">
                <Server size={13} className="text-accent" />
                <span className="font-mono text-xs font-semibold text-ink">{g.name}</span>
                <span className="chip text-[10px]">{g.hosts.length}</span>
              </div>
              <div className="flex flex-wrap gap-1.5 px-3 py-2">
                {g.hosts.length === 0
                  ? <span className="text-xs text-ink-faint">{g.children?.length ? `→ ${g.children.join(", ")}` : "—"}</span>
                  : g.hosts.map((h) => <span key={h} className="chip font-mono text-[11px]">{h}</span>)}
              </div>
            </div>
          ))}
        </div>
      )}
    </Modal>
  );
}

function InventoryForm({
  projectId,
  inventory,
  onClose,
}: {
  projectId: string;
  inventory: Inventory | null;
  onClose: () => void;
}) {
  const { t } = usePrefs();
  const qc = useQueryClient();
  const [name, setName] = useState(inventory?.name ?? "");
  const [type, setType] = useState<"static" | "file" | "dynamic" | "cloud" | "url">(inventory?.type ?? "static");
  const [content, setContent] = useState(inventory?.content ?? "[all]\nlocalhost ansible_connection=local\n");
  const [provider, setProvider] = useState(inventory?.provider || "aws_ec2");
  const [credentialId, setCredentialId] = useState(inventory?.credentialId ?? "");
  const [region, setRegion] = useState(inventory?.region ?? "");
  const [runnerTag, setRunnerTag] = useState(inventory?.runnerTag ?? "");
  const [connCredentialIds, setConnCredentialIds] = useState<string[]>(inventory?.connCredentialIds ?? []);
  const [error, setError] = useState("");
  // Project files → a picker for file/dynamic inventories (pick from the tree,
  // no typing). Type a name first → pre-fills from the chosen file's basename.
  const filesQ = useQuery({ queryKey: ["files", projectId], queryFn: () => api.projectFiles(projectId) });
  const filePaths = useMemo(() => flattenFiles(filesQ.data ?? []), [filesQ.data]);
  const creds = useQuery({ queryKey: ["credentials"], queryFn: api.listCredentials });
  const sshCreds = useMemo(() => (creds.data ?? []).filter((c) => c.type === "ssh"), [creds.data]);
  const toggleConnCred = (id: string) => setConnCredentialIds((v) => (v.includes(id) ? v.filter((x) => x !== id) : [...v, id]));

  const save = useMutation({
    mutationFn: () => {
      const body = { name, type, content: type === "cloud" ? content : content, provider, credentialId: credentialId || null, region, runnerTag, connCredentialIds };
      return inventory ? api.updateInventory(inventory.id, body) : api.createInventory(projectId, body);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["inventories", projectId] });
      onClose();
    },
    onError: (e) => setError((e as Error).message),
  });

  const typeHelp: Record<string, string> = {
    static: t("pd.invHelpStatic"),
    file: t("pd.invHelpFile"),
    dynamic: t("pd.invHelpDynamic"),
    cloud: t("pd.invHelpCloud"),
  };

  return (
    <Modal
      open
      onClose={onClose}
      wide
      title={inventory ? t("pd.invEdit") : t("pd.invNew")}
      footer={
        <>
          <button className="btn-outline" onClick={onClose}>{t("common.cancel")}</button>
          <button className="btn-primary" onClick={() => save.mutate()} disabled={!name || save.isPending}>
            {save.isPending ? <Spinner /> : <Save size={15} />} {t("common.save")}
          </button>
        </>
      }
    >
      <div className="space-y-4">
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label={t("common.name")}>
            <input className="input" value={name} onChange={(e) => setName(e.target.value)} placeholder="Production" autoFocus />
          </Field>
          <Field label={t("common.type")} hint={typeHelp[type]}>
            <Select
              value={type}
              onChange={(v) => {
                const next = v as typeof type;
                // Reset content when crossing the editor(static) ↔ path(file/dynamic)
                // boundary so the static INI never shows up as a "selected file".
                if (next === "static" && type !== "static") setContent("[all]\nlocalhost ansible_connection=local\n");
                else if (next === "cloud" || next === "url") setContent("");
                else if (next !== "static" && type === "static") setContent("");
                setType(next);
              }}
              options={[
                { label: t("pd.invTypeStatic"), value: "static" },
                { label: t("pd.invTypeFile"), value: "file" },
                { label: t("pd.invTypeDynamic"), value: "dynamic" },
                { label: t("pd.invTypeUrl"), value: "url" },
                { label: t("pd.invTypeCloud"), value: "cloud" },
              ]}
            />
          </Field>
        </div>
        {type === "static" ? (
          <Field label={t("pd.invContent")}>
            <div className="overflow-hidden rounded-lg border border-border bg-surface">
              <CodeEditor value={content} language="ini" onChange={setContent} className="h-72" />
            </div>
          </Field>
        ) : type === "cloud" ? (
          <div className="space-y-4">
            <div className="grid gap-4 sm:grid-cols-2">
              <Field label={t("pd.invProvider")}>
                <Select
                  value={provider}
                  onChange={setProvider}
                  options={[
                    { label: "AWS EC2", value: "aws_ec2" },
                    { label: "GCP Compute", value: "gcp_compute" },
                    { label: "Azure", value: "azure_rm" },
                  ]}
                />
              </Field>
              {provider !== "azure_rm" && (
                <Field label={provider === "gcp_compute" ? t("pd.invProject") : t("pd.invRegion")} hint={provider === "gcp_compute" ? t("pd.invProjectHint") : undefined}>
                  <input className="input font-mono text-xs" value={region} onChange={(e) => setRegion(e.target.value)} placeholder={provider === "gcp_compute" ? "(from service account)" : "eu-central-1"} />
                </Field>
              )}
            </div>
            <Field label={t("pd.invCredential")} hint={t("pd.invCredentialHint")}>
              <Select
                value={credentialId}
                onChange={setCredentialId}
                placeholder={(creds.data ?? []).length ? t("pd.invSelectCred") : t("pd.invNoCreds")}
                options={[{ label: t("common.none"), value: "" }, ...(creds.data ?? []).map((c) => ({ label: c.name, value: c.id }))]}
              />
            </Field>
            <Field label={t("pd.invExtraYaml")} hint={t("pd.invExtraYamlHint")}>
              <div className="overflow-hidden rounded-lg border border-border bg-surface">
                <CodeEditor value={content} language="yaml" onChange={setContent} className="h-40" />
              </div>
            </Field>
          </div>
        ) : type === "url" ? (
          <Field label={t("pd.invUrl")} hint={t("pd.invUrlHint")}>
            <input
              className="input font-mono text-xs"
              value={content}
              onChange={(e) => setContent(e.target.value)}
              placeholder="https://inventory.example.com/hosts.ini"
            />
          </Field>
        ) : (
          <Field label={type === "dynamic" ? t("pd.scriptPath") : t("pd.filePath")} hint={t("pd.pickFromTree")}>
            <Select
              value={content}
              onChange={(v) => {
                setContent(v);
                if (!name && v) setName(invNameFromPath(v));
              }}
              placeholder={filePaths.length ? t("pd.selectFromProject") : t("pd.noFilesUpload")}
              options={[
                // keep an out-of-tree custom path selectable when editing
                ...(content && !filePaths.includes(content) ? [{ label: content, value: content }] : []),
                ...filePaths.map((p) => ({ label: p, value: p })),
              ]}
            />
          </Field>
        )}
        {type !== "cloud" && (
          <Field label={t("pd.invConnCreds")} hint={t("pd.invConnCredsHint")}>
            {sshCreds.length ? (
              <div className="flex flex-wrap gap-x-5 gap-y-2">
                {sshCreds.map((c) => (
                  <Checkbox key={c.id} checked={connCredentialIds.includes(c.id)} onChange={() => toggleConnCred(c.id)} label={c.name} />
                ))}
              </div>
            ) : (
              <p className="text-xs text-ink-faint">{t("pd.invNoSshCreds")}</p>
            )}
          </Field>
        )}
        <Field label={t("pd.invRunnerTag")} hint={t("pd.invRunnerTagHint")}>
          <input className="input font-mono text-xs" value={runnerTag} onChange={(e) => setRunnerTag(e.target.value)} placeholder="edge-dc1" />
        </Field>
        {error && <ErrorText>{error}</ErrorText>}
      </div>
    </Modal>
  );
}

function Members({ projectId, myRole }: { projectId: string; myRole?: string }) {
  const { t } = usePrefs();
  const qc = useQueryClient();
  const confirm = useConfirm();
  const toast = useToast();
  const { user } = useAuth();
  const canManage = myRole === "admin";
  const members = useQuery({ queryKey: ["members", projectId], queryFn: () => api.listMembers(projectId) });
  const users = useQuery({ queryKey: ["users"], queryFn: api.listUsers, enabled: canManage });
  const customRoles = useQuery({ queryKey: ["roles"], queryFn: api.listRoles, enabled: canManage });
  const [addUser, setAddUser] = useState("");
  const [addRole, setAddRole] = useState<string>("viewer");

  const refresh = () => qc.invalidateQueries({ queryKey: ["members", projectId] });
  const set = useMutation({
    mutationFn: ({ uid, role }: { uid: string; role: string }) => api.setMember(projectId, uid, role),
    onSuccess: () => { refresh(); setAddUser(""); },
    onError: (e) => toast.error((e as Error).message),
  });
  const remove = useMutation({
    mutationFn: (uid: string) => api.removeMember(projectId, uid),
    onSuccess: () => { refresh(); toast.success(t("pd.memberRemoved")); },
    onError: (e) => toast.error((e as Error).message),
  });

  const roleOpts = [
    { label: t("pd.roleViewer"), value: "viewer" },
    { label: t("pd.roleEditor"), value: "editor" },
    { label: t("pd.roleAdmin"), value: "admin" },
    ...(customRoles.data ?? []).map((cr) => ({ label: `${cr.name} (${cr.permissions.join("/")})`, value: cr.id })),
  ];
  const roleName = (role: string) => (customRoles.data ?? []).find((cr) => cr.id === role)?.name ?? role;
  const memberIds = new Set((members.data ?? []).map((m) => m.userId));
  const candidates = (users.data ?? []).filter((u) => !memberIds.has(u.id));

  return (
    <div className="space-y-4">
      <p className="text-sm text-ink-dim">{t("pd.membersHint")}</p>

      {canManage && (
        <div className="card flex flex-wrap items-end gap-3 p-3.5">
          <div className="min-w-[200px] flex-1">
            <span className="label">{t("pd.addMember")}</span>
            <Select
              value={addUser}
              onChange={setAddUser}
              placeholder={candidates.length ? t("pd.selectUser") : t("pd.noUsersLeft")}
              options={candidates.map((u) => ({ label: u.username, value: u.id }))}
            />
          </div>
          <div className="w-40">
            <span className="label">{t("set.role")}</span>
            <Select value={addRole} onChange={(v) => setAddRole(v as typeof addRole)} options={roleOpts} />
          </div>
          <button className="btn-primary" disabled={!addUser || set.isPending} onClick={() => set.mutate({ uid: addUser, role: addRole })}>
            <Plus size={15} /> {t("common.add")}
          </button>
        </div>
      )}

      {members.isLoading ? (
        <div className="flex justify-center py-12 text-ink-faint"><Spinner /></div>
      ) : members.data?.length ? (
        <div className="card divide-y divide-border">
          {members.data.map((m) => (
            <div key={m.userId} className="flex items-center gap-3 px-4 py-3">
              {m.role === "admin" ? <Shield size={17} className="text-accent" /> : <Users size={17} className="text-ink-faint" />}
              <div className="min-w-0 flex-1">
                <div className="truncate text-sm text-ink">
                  {m.username}{m.userId === user?.id && <span className="ml-1.5 text-xs text-ink-faint">({t("pd.you")})</span>}
                </div>
                {m.email && <div className="truncate text-xs text-ink-faint">{m.email}</div>}
              </div>
              {canManage ? (
                <div className="w-44">
                  <Select value={m.role} onChange={(v) => set.mutate({ uid: m.userId, role: v })} options={roleOpts} />
                </div>
              ) : (
                <span className="chip text-[10px] uppercase">{roleName(m.role)}</span>
              )}
              {canManage && (
                <button className="btn-ghost p-1.5" title={t("pd.removeMember")} onClick={() => confirm({ title: t("pd.removeMember"), body: m.username }).then((ok) => ok && remove.mutate(m.userId))}>
                  <Trash2 size={15} />
                </button>
              )}
            </div>
          ))}
        </div>
      ) : (
        <EmptyState icon={<Users size={28} />} title={t("pd.membersNone")} description={t("pd.membersOpen")} />
      )}
    </div>
  );
}

function ProjectRuns({ projectId }: { projectId: string }) {
  const { t } = usePrefs();
  const runs = useQuery({
    queryKey: ["runs", { projectId }],
    queryFn: () => api.listRuns({ projectId, limit: 100 }),
    refetchInterval: 2000,
  });
  if (runs.isLoading) return <div className="flex justify-center py-12 text-ink-faint"><Spinner /></div>;
  if (!runs.data?.length)
    return <EmptyState icon={<Play size={28} />} title={t("pd.noRuns")} description={t("pd.noRunsHint")} />;
  return (
    <div className="card divide-y divide-border">
      {runs.data.map((r) => (
        <RunRow key={r.id} run={r} />
      ))}
    </div>
  );
}
