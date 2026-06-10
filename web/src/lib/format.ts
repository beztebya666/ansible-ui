import type { RunStatus } from "./api";

export function fmtDuration(ms: number): string {
  if (ms < 0) ms = 0;
  const s = Math.floor(ms / 1000);
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  const rs = s % 60;
  if (m < 60) return `${m}m ${rs}s`;
  const h = Math.floor(m / 60);
  return `${h}h ${m % 60}m`;
}

// fmtBytes renders a byte count as a compact human size (e.g. 2.0 KB, 3.4 MB).
export function fmtBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  const units = ["KB", "MB", "GB"];
  let v = n / 1024;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v.toFixed(1)} ${units[i]}`;
}

export function runDuration(start?: string | null, end?: string | null): string {
  if (!start) return "—";
  const a = new Date(start).getTime();
  const b = end ? new Date(end).getTime() : Date.now();
  return fmtDuration(b - a);
}

export function fmtRelative(iso?: string | null, lang?: string): string {
  if (!iso) return "—";
  // Language MUST come from the reactive prefs (pass `lang` from usePrefs) so a
  // language switch re-renders + relabels immediately. The localStorage fallback
  // is only for non-component callers; it lags by a render (effect-written), which
  // is exactly the bug we avoid by passing `lang`.
  const ru = lang ? lang === "ru" : typeof localStorage !== "undefined" && localStorage.getItem("aui.lang") === "ru";
  const d = new Date(iso).getTime();
  const diff = Date.now() - d;
  const s = Math.floor(diff / 1000);
  if (s < 5) return ru ? "только что" : "just now";
  if (s < 60) return ru ? `${s} с назад` : `${s}s ago`;
  const m = Math.floor(s / 60);
  if (m < 60) return ru ? `${m} мин назад` : `${m}m ago`;
  const h = Math.floor(m / 60);
  if (h < 24) return ru ? `${h} ч назад` : `${h}h ago`;
  const days = Math.floor(h / 24);
  if (days < 7) return ru ? `${days} дн назад` : `${days}d ago`;
  return new Date(iso).toLocaleDateString(ru ? "ru-RU" : undefined);
}

/**
 * Humanize a backend activity action code (e.g. "run.success") into a readable,
 * localized label via the i18n dictionary. Falls back to title-casing the code
 * so a raw machine string (`run.success`) never reaches the UI.
 */
export function actionLabel(action: string, t: (key: string) => string): string {
  const key = `act.${action}`;
  const label = t(key);
  if (label !== key) return label;
  return action
    .split(".")
    .map((p) => p.charAt(0).toUpperCase() + p.slice(1))
    .join(" ");
}

// Reads the saved display prefs (set in Settings). 12h vs 24h clock — default
// 24h — and an optional timezone override for run timestamps.
function timeOpts(): Intl.DateTimeFormatOptions {
  const opts: Intl.DateTimeFormatOptions = {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    hour12: typeof localStorage !== "undefined" && localStorage.getItem("aui.clock") === "12h",
  };
  const tz = typeof localStorage !== "undefined" ? localStorage.getItem("aui.tz") : null;
  if (tz) opts.timeZone = tz;
  return opts;
}

export function fmtTime(iso?: string | null): string {
  if (!iso) return "—";
  return new Date(iso).toLocaleString(undefined, timeOpts());
}

// `color` tints text/dots; `badge` is the full pill style (tinted bg + border +
// text) so every status reads as colored — never a flat gray chip.
export const statusMeta: Record<RunStatus, { label: string; color: string; dot: string; badge: string }> = {
  awaiting: { label: "Awaiting approval", color: "text-warn", dot: "bg-warn", badge: "border-warn/40 bg-warn/10 text-warn" },
  queued: { label: "Queued", color: "text-info", dot: "bg-info", badge: "border-info/40 bg-info/10 text-info" },
  pending: { label: "Pending", color: "text-ink-dim", dot: "bg-ink-faint", badge: "border-ink-faint/40 bg-ink-faint/10 text-ink-dim" },
  running: { label: "Running", color: "text-running", dot: "bg-running", badge: "border-running/45 bg-running/15 text-running" },
  success: { label: "Success", color: "text-success", dot: "bg-success", badge: "border-success/45 bg-success/15 text-success" },
  failed: { label: "Failed", color: "text-danger", dot: "bg-danger", badge: "border-danger/45 bg-danger/15 text-danger" },
  canceled: { label: "Canceled", color: "text-warn", dot: "bg-warn", badge: "border-warn/45 bg-warn/15 text-warn" },
};
