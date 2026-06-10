import { useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import clsx from "clsx";
import { ArrowLeft, Boxes, Check, Download, ExternalLink, Eye, FileCode2, Package, RotateCw, X } from "lucide-react";
import { RunConsole } from "../components/RunConsole";
import { StatusBadge } from "../components/StatusBadge";
import { Spinner } from "../components/ui";
import { api, type RequirementItem, type Run } from "../lib/api";
import { AppBadge } from "../lib/apps";
import { fmtBytes, fmtTime, runDuration } from "../lib/format";
import { usePrefs } from "../lib/prefs";
import { useAuth } from "../lib/auth";

const SECRET_MASK = "••••••";

export function RunDetail() {
  const { id = "" } = useParams();
  const navigate = useNavigate();
  const { t } = usePrefs();
  const query = useQuery({
    queryKey: ["run", id],
    queryFn: () => api.getRun(id),
    // Live fallback: poll while the run is active (the WS bus updates instantly;
    // this guarantees freshness even if a socket hiccups). Stops once finished.
    refetchInterval: (q) => {
      const st = q.state.data?.run?.status;
      return ["running", "pending", "queued", "awaiting"].includes(st ?? "") ? 1500 : false;
    },
  });
  const [run, setRun] = useState<Run | undefined>();
  const { user } = useAuth();
  const proj = useQuery({
    queryKey: ["project", run?.projectId],
    queryFn: () => api.getProject(run!.projectId),
    enabled: !!run && run.status === "awaiting",
  });
  const canApprove = user?.role === "admin" || proj.data?.myRole === "admin";

  useEffect(() => {
    if (query.data?.run) setRun(query.data.run);
  }, [query.data]);

  const decide = async (approve: boolean) => {
    if (!run) return;
    try {
      await (approve ? api.approveRun(run.id) : api.rejectRun(run.id));
    } finally {
      query.refetch();
    }
  };

  const rerun = async () => {
    if (!run) return;
    const fresh = await api.createRun({
      projectId: run.projectId,
      app: run.app,
      action: run.action,
      playbook: run.playbook,
      name: run.name,
      environmentId: run.environmentId ?? undefined,
      limit: run.limit,
      tags: run.tags,
      skipTags: run.skipTags,
      extraVars: run.extraVars,
      check: run.check,
      diff: run.diff,
      verbosity: run.verbosity,
    });
    navigate(`/runs/${fresh.id}`);
  };

  if (query.isLoading || !run) {
    return (
      <div className="flex h-[calc(100vh-3.5rem)] items-center justify-center text-ink-faint">
        <Spinner />
      </div>
    );
  }

  return (
    <div className="flex h-[calc(100vh-3.5rem)] flex-col">
      <div className="flex items-center justify-between gap-3 border-b border-border px-5 py-3">
        <div className="flex min-w-0 items-center gap-3">
          <button onClick={() => navigate(-1)} className="btn-ghost p-1.5">
            <ArrowLeft size={16} />
          </button>
          <div className="min-w-0">
            <h1 className="truncate font-mono text-sm font-medium text-ink">{run.name || run.playbook}</h1>
            <p className="flex items-center gap-1.5 truncate text-xs text-ink-faint">
              {run.projectName}
              {" · "}
              {/* Short, unique run id so identical tasks are tell-apart-able. Click to copy. */}
              <button
                type="button"
                onClick={() => navigator.clipboard?.writeText(run.id.replace(/^run_/, ""))}
                title={`${t("run.id")} — ${run.id.replace(/^run_/, "")}`}
                className="font-mono text-ink-faint transition-colors hover:text-accent"
              >
                #{run.id.replace(/^run_/, "")}
              </button>
            </p>
          </div>
        </div>
        <div className="flex items-center gap-3">
          <StatusBadge status={run.status} />
          {run.status === "awaiting" && canApprove && (
            <>
              <button className="btn-primary" onClick={() => decide(true)} title={t("run.approveHint")}>
                <Check size={14} /> {t("run.approve")}
              </button>
              <button className="btn-outline text-danger" onClick={() => decide(false)}>
                <X size={14} /> {t("run.reject")}
              </button>
            </>
          )}
          <button className="btn-outline" onClick={rerun}>
            <RotateCw size={14} /> {t("common.rerun")}
          </button>
        </div>
      </div>

      <div className="flex min-h-0 flex-1 gap-4 p-4">
        <div className="min-w-0 flex-1">
          <RunConsole runId={id} initialRun={run} onRun={setRun} />
        </div>
        <aside className="hidden w-72 shrink-0 space-y-4 overflow-y-auto lg:block">
          <Meta run={run} />
        </aside>
      </div>
    </div>
  );
}

function Meta({ run }: { run: Run }) {
  const { t } = usePrefs();
  const isAnsible = !run.app || run.app === "ansible";
  const extra = run.extraVars && Object.keys(run.extraVars).length > 0;
  const hasRecap =
    run.stats.hosts > 0 ||
    run.stats.ok + run.stats.changed + run.stats.failed + run.stats.unreachable + run.stats.skipped > 0;
  return (
    <>
      <Section title={t("run.summary")}>
        <Row k={t("run.app")} v={<AppBadge app={run.app} action={run.action} />} />
        <Row k={t("run.status")} v={<StatusBadge status={run.status} />} />
        {/* The mini-audit travels with the run: always show who triggered it. */}
        <Row k={t("run.triggeredBy")} v={run.triggeredBy || t("audit.system")} />
        {run.exitCode !== null && run.exitCode !== undefined && (
          <Row k={t("run.exitCode")} v={String(run.exitCode)} mono />
        )}
        <Row k={t("run.duration")} v={runDuration(run.startedAt, run.finishedAt)} />
        {run.commit && <Row k={t("run.commit")} v={run.commit.slice(0, 10)} mono />}
        {run.version && <Row k={t("run.artifact")} v={run.version} mono />}
        {run.runnerName && <Row k={t("run.runner")} v={run.runnerName} />}
      </Section>

      <Section title={t("run.timing")}>
        <Row k={t("run.started")} v={fmtTime(run.startedAt)} />
        <Row k={t("run.finished")} v={fmtTime(run.finishedAt)} />
        <Row k={t("run.duration")} v={runDuration(run.startedAt, run.finishedAt)} />
        {run.exitCode !== null && run.exitCode !== undefined && (
          <Row k={t("run.exitCode")} v={String(run.exitCode)} mono />
        )}
      </Section>

      {hasRecap && (
        <Section title={t("run.recap")}>
          <div className="grid grid-cols-2 gap-2 text-xs">
            <Stat label="ok" value={run.stats.ok} tone="text-success" />
            <Stat label="changed" value={run.stats.changed} tone="text-warn" />
            <Stat label="failed" value={run.stats.failed} tone="text-danger" />
            <Stat label="skipped" value={run.stats.skipped} tone="text-ink-dim" />
            <Stat label="unreachable" value={run.stats.unreachable} tone="text-danger" />
            <Stat label="rescued" value={run.stats.rescued} tone="text-info" />
          </div>
        </Section>
      )}

      <Section title={t("run.configuration")}>
        <Row k={t("run.project")} v={<Link to={`/projects/${run.projectId}`} className="text-accent hover:underline">{run.projectName}</Link>} />
        <Row
          k={isAnsible ? t("run.playbook") : t("run.entrypoint")}
          v={
            <Link
              to={`/projects/${run.projectId}?tab=files&file=${encodeURIComponent(run.playbook)}`}
              className="font-mono text-xs text-accent no-underline transition hover:brightness-125"
              title={t("pd.files")}
            >
              {run.playbook}
            </Link>
          }
        />
        {run.action && <Row k={t("run.action")} v={run.action} mono />}
        {isAnsible && run.limit && <Row k={t("run.limit")} v={run.limit} mono />}
        {isAnsible && run.tags && <Row k={t("run.tags")} v={run.tags} mono />}
        {isAnsible && run.skipTags && <Row k={t("run.skipTags")} v={run.skipTags} mono />}
        {isAnsible && <Row k={t("run.check")} v={run.check ? t("common.yes") : t("common.no")} />}
        {isAnsible && <Row k={t("run.diff")} v={run.diff ? t("common.yes") : t("common.no")} />}
        {isAnsible && run.verbosity > 0 && <Row k={t("run.verbosity")} v={`-${"v".repeat(run.verbosity)}`} mono />}
      </Section>

      {extra && <ExtraVarsBlock run={run} />}

      <ArtifactsBlock run={run} />

      <Section title={t("run.command")}>
        <CommandBlock run={run} />
      </Section>

      {isAnsible && <Requirements run={run} />}
    </>
  );
}

// ArtifactsBlock lists files captured from the run's working directory (per the
// template's artifact globs), each a one-click download.
function ArtifactsBlock({ run }: { run: Run }) {
  const { t } = usePrefs();
  const q = useQuery({ queryKey: ["artifacts", run.id], queryFn: () => api.listArtifacts(run.id) });
  const arts = q.data ?? [];
  if (!arts.length) return null;
  return (
    <Section title={t("run.artifacts")}>
      <div className="space-y-1">
        {arts.map((a) => (
          <a
            key={a.id}
            href={api.artifactURL(run.id, a.id)}
            download
            className="flex items-center gap-2 rounded-md px-2 py-1.5 text-xs text-ink-dim transition hover:bg-white/5 hover:text-ink"
            title={t("run.download")}
          >
            <Download size={13} className="shrink-0 text-ink-faint" />
            <span className="truncate font-mono">{a.name}</span>
            <span className="ml-auto shrink-0 text-ink-faint">{fmtBytes(a.size)}</span>
          </a>
        ))}
      </div>
    </Section>
  );
}

// ExtraVarsBlock shows the run's extra-vars (secrets masked). An admin can reveal
// the real secret values on demand — fetched fresh + decrypted server-side, never
// part of the stored log/record. The reveal is audited.
function ExtraVarsBlock({ run }: { run: Run }) {
  const { t } = usePrefs();
  const { user } = useAuth();
  const [revealed, setRevealed] = useState<Record<string, unknown> | null>(null);
  const [busy, setBusy] = useState(false);
  const vars = run.extraVars ?? {};
  const hasMasked = Object.values(vars).some((v) => v === SECRET_MASK);
  const canReveal = user?.role === "admin" && hasMasked && !revealed;
  const display = revealed ? { ...vars, ...revealed } : vars;

  const doReveal = async () => {
    setBusy(true);
    try {
      setRevealed(await api.runSecrets(run.id));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Section title={t("run.extraVars")}>
      <pre className="overflow-x-auto rounded-lg border border-border bg-surface p-2.5 text-xs text-ink-dim">
        {JSON.stringify(display, null, 2)}
      </pre>
      {canReveal && (
        <button
          className="btn-ghost mt-2 gap-1.5 text-xs"
          disabled={busy}
          onClick={doReveal}
          title={t("run.revealSecretsHint")}
        >
          {busy ? <Spinner /> : <Eye size={13} />} {t("run.revealSecrets")}
        </button>
      )}
    </Section>
  );
}

// Requirements lists the galaxy roles/collections a run pulls in, each a clickable
// badge linking to its source (galaxy / git page, or the local file editor).
function Requirements({ run }: { run: Run }) {
  const { t } = usePrefs();
  const q = useQuery({
    queryKey: ["requirements", run.projectId],
    queryFn: () => api.projectRequirements(run.projectId),
  });
  const items = q.data?.items ?? [];
  if (!items.length) return null;
  return (
    <Section title={t("run.requirements")}>
      <div className="flex flex-wrap gap-1.5">
        {items.map((it) => (
          <RequirementBadge key={it.kind + "|" + it.name} item={it} projectId={run.projectId} />
        ))}
      </div>
    </Section>
  );
}

function RequirementBadge({ item, projectId }: { item: RequirementItem; projectId: string }) {
  const Icon = item.kind === "collection" ? Boxes : Package;
  const tone =
    item.source === "local"
      ? "border-info/40 bg-info/10 text-info"
      : item.source === "git"
        ? "border-warn/40 bg-warn/10 text-warn"
        : "border-success/40 bg-success/10 text-success";
  const cls = clsx(
    "inline-flex max-w-full items-center gap-1.5 rounded-md border px-2 py-1 text-xs font-medium transition hover:brightness-125",
    tone,
  );
  const inner = (
    <>
      <Icon size={12} className="shrink-0" />
      <span className="truncate font-mono">{item.name}</span>
      {item.source === "local" ? (
        <FileCode2 size={11} className="shrink-0 opacity-70" />
      ) : (
        <ExternalLink size={11} className="shrink-0 opacity-70" />
      )}
    </>
  );
  if (item.source === "local" && item.path) {
    return (
      <Link to={`/projects/${projectId}?tab=files&file=${encodeURIComponent(item.path)}`} className={cls} title={item.path}>
        {inner}
      </Link>
    );
  }
  if (item.url) {
    return (
      <a href={item.url} target="_blank" rel="noreferrer" className={cls} title={item.url}>
        {inner}
      </a>
    );
  }
  return <span className={cls}>{inner}</span>;
}

// CommandBlock renders the effective command with light syntax colour and makes
// the inventory + playbook clickable (→ file editor). No underline even on hover
// (house style), and nothing is truncated — long commands wrap in full.
function CommandBlock({ run }: { run: Run }) {
  const { t } = usePrefs();
  const isAnsible = !run.app || run.app === "ansible";
  const tokens = isAnsible ? ["ansible-playbook", ...(run.args ?? [])] : run.args ?? [];
  const invFlags = new Set(["-i", "--inventory", "--inventory-file"]);
  let prevFlag = "";
  // Wrap only BETWEEN arguments — the flex gap is the inter-token space, and each
  // token is atomic (whitespace-nowrap) so a flag like "--diff" or a path never
  // splits across lines (the browser would otherwise break on the "-"/"/").
  return (
    <div className="flex flex-wrap items-baseline gap-x-[0.45rem] gap-y-1 rounded-lg border border-border bg-surface p-2.5 font-mono text-[11px] leading-relaxed">
      {tokens.map((tok, i) => {
        if (i === 0) return <span key={i} className="whitespace-nowrap text-accent">{tok}</span>; // binary
        if (tok.startsWith("-")) {
          prevFlag = tok;
          return <span key={i} className="whitespace-nowrap text-warn">{tok}</span>; // flag
        }
        // A path is clickable when it's the playbook or an inventory value living
        // inside the project (relative path, not absolute or a JSON extra-vars blob).
        const fileLike =
          (tok === run.playbook || invFlags.has(prevFlag)) && !tok.startsWith("/") && !tok.startsWith("{");
        prevFlag = "";
        if (fileLike) {
          return (
            <Link
              key={i}
              to={`/projects/${run.projectId}?tab=files&file=${encodeURIComponent(tok)}`}
              className={`min-w-0 text-info no-underline transition hover:brightness-125 ${tok.length > 32 ? "break-all" : "whitespace-nowrap"}`}
              title={t("pd.files")}
            >
              {tok}
            </Link>
          );
        }
        // Only an over-long value (e.g. an -e JSON blob) may break mid-token; every
        // ordinary value stays whole.
        const breakable = tok.startsWith("{") || tok.length > 40;
        return (
          <span key={i} className={`text-ink ${breakable ? "break-all" : "whitespace-nowrap"}`}>
            {tok}
          </span>
        );
      })}
    </div>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="card p-3.5">
      <h3 className="mb-2.5 text-xs font-semibold uppercase tracking-wide text-ink-faint">{title}</h3>
      <div className="space-y-1.5">{children}</div>
    </div>
  );
}

function Row({ k, v, mono }: { k: string; v: React.ReactNode; mono?: boolean }) {
  return (
    <div className="flex items-baseline justify-between gap-3 text-sm">
      <span className="shrink-0 text-ink-faint">{k}</span>
      {/* Never truncate — values wrap in full (no "…"). */}
      <span className={`min-w-0 break-words text-right text-ink-dim ${mono ? "font-mono text-xs" : ""}`}>{v}</span>
    </div>
  );
}

function Stat({ label, value, tone }: { label: string; value: number; tone: string }) {
  return (
    <div className="flex items-center justify-between rounded-md border border-border bg-surface px-2 py-1">
      <span className="text-ink-faint">{label}</span>
      <span className={`font-mono font-medium ${tone}`}>{value}</span>
    </div>
  );
}
