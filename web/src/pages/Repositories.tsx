import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import clsx from "clsx";
import { GitBranch, Github, Plus, RefreshCw, Trash2 } from "lucide-react";
import { Page, PageHeader } from "../components/PageHeader";
import { EmptyState, ErrorText, Field, Modal, Spinner, Toggle } from "../components/ui";
import { useConfirm } from "../components/feedback";
import { Select } from "../components/Select";
import { api, type Repository } from "../lib/api";
import { fmtRelative } from "../lib/format";
import { usePrefs } from "../lib/prefs";

const repoStatusMeta: Record<string, { color: string; dot: string }> = {
  unknown: { color: "text-ink-dim", dot: "bg-ink-faint" },
  syncing: { color: "text-running", dot: "bg-running" },
  ready: { color: "text-success", dot: "bg-success" },
  error: { color: "text-danger", dot: "bg-danger" },
};

export function Repositories() {
  const { t } = usePrefs();
  const repos = useQuery({ queryKey: ["repositories"], queryFn: api.listRepositories });
  const [editing, setEditing] = useState<Repository | null>(null);
  const [creating, setCreating] = useState(false);

  return (
    <>
      <PageHeader
        title={t("repo.title")}
        subtitle={t("repo.subtitle")}
        actions={
          <button className="btn-primary" onClick={() => setCreating(true)}>
            <Plus size={15} /> {t("repo.new")}
          </button>
        }
      />
      <Page>
        {repos.isLoading ? (
          <div className="flex justify-center py-20 text-ink-faint">
            <Spinner />
          </div>
        ) : repos.data?.length ? (
          <div className="card divide-y divide-border">
            {repos.data.map((repo) => (
              <RepoRow key={repo.id} repo={repo} onEdit={() => setEditing(repo)} />
            ))}
          </div>
        ) : (
          <EmptyState
            icon={<Github size={28} />}
            title={t("repo.none")}
            description={t("repo.noneHint")}
            action={
              <button className="btn-primary" onClick={() => setCreating(true)}>
                <Plus size={15} /> {t("repo.new")}
              </button>
            }
          />
        )}
      </Page>
      {(creating || editing) && (
        <RepoForm repository={editing} onClose={() => { setCreating(false); setEditing(null); }} />
      )}
    </>
  );
}

function RepoRow({ repo, onEdit }: { repo: Repository; onEdit: () => void }) {
  const { t, lang } = usePrefs();
  const confirm = useConfirm();
  const qc = useQueryClient();
  const st = repoStatusMeta[repo.status] ?? repoStatusMeta.unknown;
  const stLabel = t(`repo.status.${repo.status in repoStatusMeta ? repo.status : "unknown"}`);

  const sync = useMutation({
    mutationFn: () => api.syncRepository(repo.id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["repositories"] }),
  });
  const del = useMutation({
    mutationFn: () => api.deleteRepository(repo.id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["repositories"] }),
  });
  const syncing = sync.isPending || repo.status === "syncing";

  return (
    <div className="flex items-center gap-3 px-4 py-3">
      <GitBranch size={18} className="shrink-0 text-accent" />
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span className="truncate text-sm text-ink">{repo.name}</span>
          <span className="chip">{repo.branch}</span>
          {repo.cacheEnabled && <span className="chip text-[10px] text-info" title={t("repo.cacheHint")}>{t("repo.cacheBadge")}</span>}
          <span className={clsx("flex items-center gap-1 text-xs", st.color)}>
            <span className={clsx("h-1.5 w-1.5 rounded-full", st.dot, syncing && "animate-pulseDot")} />
            {stLabel}
          </span>
        </div>
        <div className="truncate font-mono text-xs text-ink-faint">{repo.gitUrl}</div>
        {repo.status === "error" && repo.lastError && (
          <div className="mt-0.5 truncate text-xs text-danger">{repo.lastError}</div>
        )}
        {repo.status === "ready" && repo.lastCommit && (
          <div className="mt-0.5 truncate text-xs text-ink-faint">
            {repo.lastCommit.slice(0, 10)} · {t("repo.synced")} {fmtRelative(repo.lastSyncedAt, lang)}
          </div>
        )}
      </div>
      <button className="btn-outline px-2.5 py-1.5" onClick={() => sync.mutate()} disabled={syncing} title={t("repo.syncNow")}>
        <RefreshCw size={14} className={syncing ? "animate-spin" : ""} />
        <span className="hidden sm:inline">{t("common.sync")}</span>
      </button>
      <button className="btn-ghost p-1.5" onClick={onEdit} title={t("common.edit")}>
        <GitBranch size={15} />
      </button>
      <button
        className="btn-ghost p-1.5"
        title={t("common.delete")}
        onClick={() => confirm({ title: t("repo.confirmDelete"), body: repo.name }).then((ok) => ok && del.mutate())}
      >
        <Trash2 size={15} />
      </button>
    </div>
  );
}

function RepoForm({ repository, onClose }: { repository: Repository | null; onClose: () => void }) {
  const { t } = usePrefs();
  const qc = useQueryClient();
  const editing = !!repository;
  const credentials = useQuery({ queryKey: ["credentials"], queryFn: api.listCredentials });
  const [name, setName] = useState(repository?.name ?? "");
  const [gitUrl, setGitUrl] = useState(repository?.gitUrl ?? "");
  const [branch, setBranch] = useState(repository?.branch ?? "main");
  const [credentialId, setCredentialId] = useState(repository?.credentialId ?? "");
  const [cacheEnabled, setCacheEnabled] = useState(repository?.cacheEnabled ?? true);
  const [error, setError] = useState("");

  const save = useMutation({
    mutationFn: () =>
      editing
        ? api.updateRepository(repository!.id, { name, gitUrl, branch, credentialId: credentialId || null, cacheEnabled })
        : api.createRepository({ name, gitUrl, branch, credentialId: credentialId || null, cacheEnabled }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["repositories"] });
      onClose();
    },
    onError: (e) => setError((e as Error).message),
  });

  return (
    <Modal
      open
      onClose={onClose}
      title={editing ? t("repo.edit") : t("repo.new")}
      subtitle={t("repo.formSub")}
      footer={
        <>
          <button className="btn-outline" onClick={onClose}>{t("common.cancel")}</button>
          <button className="btn-primary" onClick={() => { setError(""); save.mutate(); }} disabled={!name || !gitUrl || save.isPending}>
            {save.isPending ? <Spinner /> : null} {t("common.save")}
          </button>
        </>
      }
    >
      <div className="space-y-4">
        <Field label={t("common.name")}>
          <input className="input" value={name} onChange={(e) => setName(e.target.value)} placeholder="infra" autoFocus />
        </Field>
        <Field label={t("repo.gitUrl")}>
          <input
            className="input font-mono text-xs"
            value={gitUrl}
            onChange={(e) => setGitUrl(e.target.value)}
            placeholder="git@github.com:org/repo.git  or  https://github.com/org/repo.git"
          />
        </Field>
        <div className="grid grid-cols-2 gap-4">
          <Field label={t("repo.branch")}>
            <input className="input" value={branch} onChange={(e) => setBranch(e.target.value)} placeholder="main" />
          </Field>
          <Field label={t("repo.credential")} hint={t("repo.credentialHint")}>
            <Select
              value={credentialId ?? ""}
              onChange={setCredentialId}
              options={[
                { label: t("repo.credNone"), value: "" },
                ...(credentials.data?.filter((c) => c.type === "ssh" || c.type === "login_password").map((c) => ({ label: c.name, value: c.id })) ?? []),
              ]}
            />
          </Field>
        </div>
        <div className="rounded-lg border border-border p-3">
          <Toggle checked={cacheEnabled} onChange={setCacheEnabled} label={t("repo.cache")} />
          <p className="mt-1 text-xs text-ink-faint">{t("repo.cacheHint")}</p>
        </div>
        {error && <ErrorText>{error}</ErrorText>}
      </div>
    </Modal>
  );
}
