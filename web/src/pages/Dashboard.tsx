import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import clsx from "clsx";
import {
  Activity,
  FolderGit2,
  ListChecks,
  SquareTerminal,
  User as UserIcon,
  type LucideIcon,
} from "lucide-react";
import { Page, PageHeader } from "../components/PageHeader";
import { RunRow } from "../components/RunRow";
import { EmptyState, Spinner } from "../components/ui";
import { LaunchButton } from "../components/Launcher";
import { api } from "../lib/api";
import { actionLabel, fmtRelative } from "../lib/format";
import { usePrefs } from "../lib/prefs";

type Tab = "overview" | "activity" | "history";

export function Dashboard() {
  const { t } = usePrefs();
  const [tab, setTab] = useState<Tab>("overview");
  const stats = useQuery({ queryKey: ["stats"], queryFn: api.stats, refetchInterval: 2000 });
  const d = stats.data;
  const recent = d?.recentRuns ?? [];

  const successRate = (() => {
    if (!d) return null;
    const s = d.runsByState ?? {};
    const ok = s.success ?? 0;
    const done = ok + (s.failed ?? 0) + (s.canceled ?? 0);
    return done ? Math.round((ok / done) * 100) : null;
  })();

  return (
    <>
      <PageHeader
        title={t("dash.title")}
        subtitle={t("dash.subtitle")}
        actions={<LaunchButton />}
      />
      <Page>
        <div className="mb-4 flex gap-1 border-b border-border">
          {(["overview", "activity", "history"] as Tab[]).map((tb) => (
            <button
              key={tb}
              onClick={() => setTab(tb)}
              className={clsx(
                "relative px-3 py-2 text-sm font-medium transition-colors",
                tab === tb ? "text-ink" : "text-ink-faint hover:text-ink-dim",
              )}
            >
              {t(`dash.tab.${tb}`)}
              {tab === tb && <span className="absolute inset-x-2 -bottom-px h-0.5 rounded-full bg-accent" />}
            </button>
          ))}
        </div>

        {tab === "activity" && <ActivityFeed />}
        {tab === "history" && <History />}
        {tab !== "overview" ? null : (
        <>
        <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
          <StatCard icon={FolderGit2} label={t("dash.projects")} value={d?.projects} to="/projects" />
          <StatCard icon={ListChecks} label={t("dash.templates")} value={d?.templates} to="/templates" />
          <StatCard icon={SquareTerminal} label={t("dash.totalRuns")} value={d?.runsTotal} to="/runs" />
          <StatCard
            icon={Activity}
            label={t("dash.activeNow")}
            value={d?.active}
            accent={!!d?.active}
            to="/runs"
          />
        </div>

        <div className="mt-4 grid gap-4 lg:grid-cols-3">
          <div className="card lg:col-span-2">
            <div className="flex items-center justify-between border-b border-border px-4 py-3">
              <h2 className="text-sm font-semibold text-ink">{t("dash.recentRuns")}</h2>
              <Link to="/runs" className="text-xs text-accent hover:underline">
                {t("common.viewAll")}
              </Link>
            </div>
            {stats.isLoading ? (
              <div className="flex items-center justify-center py-16 text-ink-faint">
                <Spinner />
              </div>
            ) : recent.length ? (
              <div className="divide-y divide-border">
                {recent.map((r) => (
                  <RunRow key={r.id} run={r} />
                ))}
              </div>
            ) : (
              <div className="p-6">
                <EmptyState
                  icon={<SquareTerminal size={28} />}
                  title={t("dash.noRuns")}
                  description={t("dash.noRunsHint")}
                  action={<LaunchButton />}
                />
              </div>
            )}
          </div>

          <div className="card p-4">
            <h2 className="text-sm font-semibold text-ink">{t("dash.health")}</h2>
            <div className="mt-4 space-y-4">
              <Gauge label={t("dash.successRate")} value={successRate} suffix="%" />
              <div className="space-y-2">
                {["success", "failed", "running", "queued", "pending", "canceled"].map((k) => (
                  <StateBar key={k} label={k} count={d?.runsByState?.[k] ?? 0} total={d?.runsTotal ?? 0} />
                ))}
              </div>
            </div>
          </div>
        </div>
        </>
        )}
      </Page>
    </>
  );
}

function ActivityFeed() {
  const { t, lang } = usePrefs();
  const acts = useQuery({ queryKey: ["activity"], queryFn: () => api.listActivity(100), refetchInterval: 3000 });
  if (acts.isLoading) return <div className="flex justify-center py-16 text-ink-faint"><Spinner /></div>;
  if (!acts.data?.length)
    return <EmptyState icon={<Activity size={28} />} title={t("dash.noActivity")} description={t("dash.noActivityHint")} />;
  return (
    <div className="card divide-y divide-border">
      {acts.data.map((a) => (
        <div key={a.id} className="flex items-center gap-3 px-4 py-2.5 text-sm">
          <span className={clsx("h-1.5 w-1.5 shrink-0 rounded-full", actionTone(a.action))} />
          <span className="w-36 shrink-0 truncate text-xs font-medium text-ink-dim" title={a.action}>{actionLabel(a.action, t)}</span>
          {/* The mini-audit carries the actor everywhere it appears, not just the Audit tab. */}
          <span className="inline-flex w-32 shrink-0 items-center gap-1.5 text-xs text-ink-dim" title={a.actor || t("audit.system")}>
            <UserIcon size={12} className="text-ink-faint" />
            <span className="truncate">{a.actor || t("audit.system")}</span>
          </span>
          <span className="min-w-0 flex-1 truncate text-ink">{a.target}{a.detail ? <span className="text-ink-faint"> · {a.detail}</span> : null}</span>
          <span className="shrink-0 text-xs text-ink-faint">{fmtRelative(a.createdAt, lang)}</span>
        </div>
      ))}
    </div>
  );
}

function actionTone(action: string): string {
  if (action.includes("success")) return "bg-success";
  if (action.includes("failed")) return "bg-danger";
  if (action.includes("canceled")) return "bg-warn";
  if (action.includes("launched") || action.includes("created")) return "bg-info";
  return "bg-ink-faint";
}

function History() {
  const { t } = usePrefs();
  const runs = useQuery({ queryKey: ["runs", "history"], queryFn: () => api.listRuns({ limit: 100 }), refetchInterval: 3000 });
  if (runs.isLoading) return <div className="flex justify-center py-16 text-ink-faint"><Spinner /></div>;
  if (!runs.data?.length)
    return <EmptyState icon={<SquareTerminal size={28} />} title={t("dash.noRuns")} description={t("dash.noHistoryHint")} />;
  return (
    <div className="card divide-y divide-border">
      {runs.data.map((r) => (
        <RunRow key={r.id} run={r} />
      ))}
    </div>
  );
}

function StatCard({
  icon: Icon,
  label,
  value,
  to,
  accent,
}: {
  icon: LucideIcon;
  label: string;
  value?: number;
  to: string;
  accent?: boolean;
}) {
  return (
    <Link to={to} className="card group p-4 transition-colors hover:border-border-strong">
      <div className="flex items-center justify-between">
        <span className="text-sm text-ink-dim">{label}</span>
        <Icon size={18} className={accent ? "text-accent" : "text-ink-faint"} />
      </div>
      <div className={`mt-3 text-3xl font-semibold tabular-nums ${accent ? "text-accent" : "text-ink"}`}>
        {value ?? "—"}
      </div>
    </Link>
  );
}

function Gauge({ label, value, suffix }: { label: string; value: number | null; suffix?: string }) {
  return (
    <div>
      <div className="flex items-baseline justify-between">
        <span className="text-sm text-ink-dim">{label}</span>
        <span className="text-2xl font-semibold tabular-nums text-ink">
          {value === null ? "—" : value}
          {value !== null && suffix}
        </span>
      </div>
      <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-border">
        <div
          className="h-full rounded-full bg-accent transition-all duration-500"
          style={{ width: `${value ?? 0}%` }}
        />
      </div>
    </div>
  );
}

function StateBar({ label, count, total }: { label: string; count: number; total: number }) {
  const { t } = usePrefs();
  const pct = total ? Math.round((count / total) * 100) : 0;
  const color =
    label === "success"
      ? "bg-success"
      : label === "failed"
        ? "bg-danger"
        : label === "running"
          ? "bg-running"
          : label === "queued"
            ? "bg-info"
            : label === "canceled"
              ? "bg-warn"
              : "bg-ink-faint";
  return (
    <div className="flex items-center gap-3 text-xs">
      <span className="w-16 text-ink-dim">{t(`status.${label}`)}</span>
      <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-border">
        <div className={`h-full rounded-full ${color}`} style={{ width: `${pct}%` }} />
      </div>
      <span className="w-6 text-right tabular-nums text-ink-faint">{count}</span>
    </div>
  );
}
