import { useEffect, useState } from "react";
import clsx from "clsx";
import { Ban, Download, FileText, Maximize2, Minimize2 } from "lucide-react";
import { LogView } from "./LogView";
import { StatusDot } from "./StatusBadge";
import { api, type Run, type RunStatus } from "../lib/api";
import { runDuration, statusMeta } from "../lib/format";
import { usePrefs } from "../lib/prefs";

interface Props {
  runId: string;
  initialRun?: Run;
  onRun?: (run: Run) => void;
}

export function RunConsole({ runId, initialRun }: Props) {
  const { t } = usePrefs();
  const run = initialRun;
  const status: RunStatus = run?.status ?? "pending";
  const [fullscreen, setFullscreen] = useState(false);
  const [, force] = useState(0);

  // Esc exits fullscreen.
  useEffect(() => {
    if (!fullscreen) return;
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && setFullscreen(false);
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [fullscreen]);

  // Live elapsed timer while running (RunDetail polls the run + the WS event bus
  // keeps the status fresh; this just re-renders the duration each second).
  useEffect(() => {
    if (status !== "running" && status !== "pending") return;
    const id = window.setInterval(() => force((n) => n + 1), 1000);
    return () => window.clearInterval(id);
  }, [status]);

  const cancel = async () => {
    try {
      await api.cancelRun(runId);
    } catch {
      /* RunDetail will reconcile the status on its next poll */
    }
  };

  const isLive = status === "running" || status === "pending";
  const finished = status === "success" || status === "failed" || status === "canceled";
  const m = statusMeta[status];
  const stats = run?.stats;
  // Only Ansible runs that reached a PLAY RECAP have meaningful ok/changed/failed
  // counts. Otherwise (non-Ansible, or a failure before the recap) the exit code
  // is the truth — showing "0 failed" on a failed run is misleading.
  const hasRecap =
    !!stats &&
    (stats.hosts > 0 || stats.ok + stats.changed + stats.failed + stats.unreachable + stats.skipped > 0);

  return (
    <div
      className={clsx(
        "flex flex-col overflow-hidden border border-border bg-bg shadow-soft",
        fullscreen ? "fixed inset-0 z-50 rounded-none" : "h-full rounded-xl",
      )}
    >
      {/* header */}
      <div className="flex items-center justify-between gap-4 border-b border-border bg-panel/60 px-4 py-2.5">
        <div className="flex min-w-0 items-center gap-3">
          <StatusDot status={status} animate />
          <div className="min-w-0">
            <div className="flex items-center gap-2 truncate font-mono text-sm text-ink">
              {run?.playbook ?? "—"}
            </div>
            <div className="truncate text-xs text-ink-faint">
              <span className={m.color}>{t(`status.${status}`)}</span>
              {run?.projectName && <> · {run.projectName}</>}
              {run && <> · {runDuration(run.startedAt, run.finishedAt)}</>}
            </div>
          </div>
        </div>

        <div className="flex shrink-0 items-center gap-3">
          {finished && hasRecap && stats && (
            <div className="hidden items-center gap-1.5 sm:flex">
              <Pill label="ok" value={stats.ok} tone="text-success" />
              <Pill label="changed" value={stats.changed} tone="text-warn" />
              <Pill label="failed" value={stats.failed} tone="text-danger" />
              <Pill label="skipped" value={stats.skipped} tone="text-ink-dim" />
              {stats.unreachable > 0 && (
                <Pill label="unreachable" value={stats.unreachable} tone="text-danger" />
              )}
            </div>
          )}
          {finished && !hasRecap && (
            // No PLAY RECAP (e.g. failed at parse/galaxy, or a non-Ansible app):
            // surface the exit code instead of a misleading 0/0/0.
            <div className="hidden items-center gap-1.5 sm:flex">
              <span
                className={clsx(
                  "inline-flex items-center gap-1 rounded-md border px-2 py-0.5 text-xs font-medium",
                  status === "success"
                    ? "border-success/30 bg-success/10 text-success"
                    : "border-danger/30 bg-danger/10 text-danger",
                )}
              >
                exit {run?.exitCode ?? "?"}
                {status !== "success" && <span className="opacity-80">· no recap</span>}
              </span>
            </div>
          )}

          <div className="flex items-center gap-1">
            {isLive ? (
              <button onClick={cancel} className="btn-danger px-2 py-1.5" title={t("common.cancelRun")}>
                <Ban size={15} />
                <span className="hidden md:inline">{t("common.cancelRun")}</span>
              </button>
            ) : (
              <a
                href={api.runOutputURL(runId)}
                download={`${runId}.log`}
                className="btn-ghost p-1.5"
                title={t("run.downloadLog")}
              >
                <Download size={16} />
              </a>
            )}
            <a
              href={`/runs/${runId}/raw`}
              target="_blank"
              rel="noreferrer"
              className="btn-ghost p-1.5"
              title={t("run.rawLog")}
            >
              <FileText size={16} />
            </a>
            <button
              onClick={() => setFullscreen((f) => !f)}
              className="btn-ghost p-1.5"
              title={fullscreen ? t("run.exitFullscreen") : t("run.fullscreen")}
            >
              {fullscreen ? <Minimize2 size={16} /> : <Maximize2 size={16} />}
            </button>
          </div>
        </div>
      </div>

      {/* the Semaphore-style structured log */}
      <div className="relative min-h-0 flex-1 bg-bg">
        <LogView runId={runId} active={isLive} />
      </div>
    </div>
  );
}

function Pill({ label, value, tone }: { label: string; value: number; tone: string }) {
  return (
    <span className="inline-flex items-center gap-1 rounded-md border border-border bg-surface px-1.5 py-0.5 text-xs font-medium text-ink-dim">
      <span className={tone}>{value}</span>
      <span className="text-ink-faint">{label}</span>
    </span>
  );
}
