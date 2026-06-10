import clsx from "clsx";
import { Loader2 } from "lucide-react";
import type { RunStatus } from "../lib/api";
import { statusMeta } from "../lib/format";
import { usePrefs } from "../lib/prefs";

export function StatusDot({ status, animate }: { status: RunStatus; animate?: boolean }) {
  const m = statusMeta[status];
  return (
    <span className="relative flex h-2.5 w-2.5">
      {animate && status === "running" && (
        <span className={clsx("absolute inline-flex h-full w-full rounded-full opacity-60", m.dot, "animate-pulseDot")} />
      )}
      <span className={clsx("relative inline-flex h-2.5 w-2.5 rounded-full", m.dot)} />
    </span>
  );
}

/** RunningPill is the animated "Running" badge (spinner + shimmer sweep). */
export function RunningPill({ label = "Running", className }: { label?: string; className?: string }) {
  return (
    <span
      className={clsx(
        "relative inline-flex items-center gap-1.5 overflow-hidden rounded-full border border-running/40 bg-running/15 px-2.5 py-0.5 text-xs font-semibold text-running",
        className,
      )}
    >
      <span className="pointer-events-none absolute inset-0 -translate-x-full animate-shimmer bg-gradient-to-r from-transparent via-white/15 to-transparent" />
      <Loader2 size={12} className="animate-spin" />
      {label}
    </span>
  );
}

export function StatusBadge({ status }: { status: RunStatus }) {
  const { t } = usePrefs();
  const label = t(`status.${status}`);
  if (status === "running") return <RunningPill label={label} />;
  const m = statusMeta[status];
  return (
    <span
      className={clsx(
        "inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-xs font-semibold",
        m.badge,
      )}
    >
      <StatusDot status={status} />
      {label}
    </span>
  );
}
