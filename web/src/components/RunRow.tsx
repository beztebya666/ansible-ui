import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import { ChevronRight, Trash2 } from "lucide-react";
import { StatusBadge, StatusDot } from "./StatusBadge";
import { ConfirmDialog } from "./ui";
import { api, type Run } from "../lib/api";
import { AppBadge } from "../lib/apps";
import { fmtRelative, runDuration } from "../lib/format";
import { usePrefs } from "../lib/prefs";

/** Whether a run produced an Ansible PLAY RECAP (only Ansible runs do). */
function hasRecap(run: Run): boolean {
  const s = run.stats;
  return s.hosts > 0 || s.ok + s.changed + s.failed + s.unreachable + s.skipped > 0;
}

export function RunRow({ run }: { run: Run }) {
  const navigate = useNavigate();
  const qc = useQueryClient();
  const { t, lang } = usePrefs();
  const [confirming, setConfirming] = useState(false);
  const [busy, setBusy] = useState(false);
  const finished = run.status === "success" || run.status === "failed" || run.status === "canceled";

  const del = async () => {
    setBusy(true);
    try {
      await api.deleteRun(run.id);
      qc.invalidateQueries({ queryKey: ["runs"] });
      qc.invalidateQueries({ queryKey: ["stats"] });
      setConfirming(false);
    } finally {
      setBusy(false);
    }
  };
  return (
    <>
    <Link
      to={`/runs/${run.id}`}
      className="group flex min-h-[3.75rem] cursor-pointer items-center gap-3 px-4 py-3 transition-colors hover:bg-white/[0.03]"
    >
      <StatusDot status={run.status} animate />
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          {run.app !== "ansible" && <AppBadge app={run.app} action={run.action} />}
          <span className="truncate font-mono text-sm text-ink">{run.playbook}</span>
        </div>
        <div className="truncate text-xs text-ink-faint">
          <span
            role="link"
            title={t("runs.openProject")}
            onClick={(e) => {
              e.preventDefault();
              e.stopPropagation();
              navigate(`/projects/${run.projectId}`);
            }}
            className="cursor-pointer hover:text-accent hover:underline"
          >
            {run.projectName ?? "project"}
          </span>{" "}
          · {fmtRelative(run.startedAt ?? run.createdAt, lang)}
          {run.triggeredBy ? <> · {t("runs.by")} {run.triggeredBy}</> : null}
          {" · "}
          <span className="font-mono opacity-70">#{run.id.replace(/^run_/, "").slice(0, 8)}</span>
        </div>
      </div>

      {/* Fixed-width metric columns so exit/recap + duration line up on every
          row regardless of status (a finished row, a running row, a recap…). */}
      <div className="hidden items-center gap-3 text-xs text-ink-faint sm:flex">
        <span className="w-20 whitespace-nowrap text-right font-mono">
          {finished &&
            (hasRecap(run) ? (
              <>
                <span className="text-success">{run.stats.ok}</span>
                {" / "}
                <span className="text-warn">{run.stats.changed}</span>
                {" / "}
                <span className="text-danger">{run.stats.failed}</span>
              </>
            ) : (
              <span className={run.status === "success" ? "text-success" : "text-danger"}>
                exit {run.exitCode ?? "?"}
              </span>
            ))}
        </span>
        <span className="w-12 text-right tabular-nums">{runDuration(run.startedAt, run.finishedAt)}</span>
      </div>
      {/* Consistent colored status pill, in a fixed column so a short label
          ("Failed") doesn't shove the rest of the row sideways. */}
      <div className="flex w-24 shrink-0 justify-end">
        <StatusBadge status={run.status} />
      </div>
      {/* Minimalist per-row delete: appears on hover, finished runs only. */}
      {finished && (
        <span
          role="button"
          tabIndex={0}
          title={t("runs.delete")}
          onClick={(e) => {
            e.preventDefault();
            e.stopPropagation();
            setConfirming(true);
          }}
          className="shrink-0 rounded-md p-1 text-ink-faint opacity-0 transition-all hover:bg-danger/10 hover:text-danger group-hover:opacity-100"
        >
          <Trash2 size={15} />
        </span>
      )}
      <ChevronRight size={16} className="shrink-0 text-ink-faint transition-transform group-hover:translate-x-0.5" />
    </Link>

    <ConfirmDialog
      open={confirming}
      title={t("runs.deleteTitle")}
      body={<><span className="font-mono text-ink">{run.playbook}</span> · {run.projectName}</>}
      confirmLabel={t("common.delete")}
      cancelLabel={t("common.cancel")}
      busy={busy}
      onConfirm={del}
      onClose={() => setConfirming(false)}
    />
    </>
  );
}
