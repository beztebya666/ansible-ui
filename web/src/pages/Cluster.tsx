import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { Crown, Network, Server } from "lucide-react";
import { Page, PageHeader } from "../components/PageHeader";
import { StatusBadge } from "../components/StatusBadge";
import { Spinner } from "../components/ui";
import { api, type Run, type RunnerCapacity } from "../lib/api";
import { usePrefs } from "../lib/prefs";
import { fmtRelative } from "../lib/format";

export function Cluster() {
  const { t, lang } = usePrefs();
  const [tab, setTab] = useState<"queued" | "running">("running");
  const cluster = useQuery({
    queryKey: ["cluster"],
    queryFn: api.getCluster,
    refetchInterval: 2000,
  });
  const d = cluster.data;
  const runnerName = (id?: string) => d?.runners.find((r) => r.id === id)?.name ?? (id ? id.slice(0, 8) : "—");

  return (
    <>
      <PageHeader title={t("cluster.title")} subtitle={t("cluster.subtitle")} />
      <Page>
        {!d ? (
          <div className="flex justify-center py-16 text-ink-faint"><Spinner /></div>
        ) : (
          <>
            <div className="mb-5 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
              <Stat
                icon={<Crown size={16} />}
                label={t("cluster.thisInstance")}
                value={d.leader ? t("cluster.leader") : t("cluster.standby")}
                tone={d.leader ? "text-success" : "text-ink-dim"}
              />
              <Stat icon={<Server size={16} />} label={t("cluster.runnersOnline")} value={String(d.counts.runnersOnline)} />
              <Stat icon={<Network size={16} />} label={t("status.running")} value={String(d.counts.running)} tone="text-info" />
              <Stat icon={<Network size={16} />} label={t("status.queued")} value={String(d.counts.queued)} tone="text-warn" />
            </div>

            <h2 className="mb-2 text-sm font-semibold text-ink">{t("cluster.capacity")}</h2>
            <div className="mb-6 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
              {d.runners.map((rn) => <RunnerCard key={rn.id} rn={rn} t={t} />)}
            </div>

            <div className="mb-3 flex gap-1 border-b border-border">
              {(["running", "queued"] as const).map((k) => (
                <button
                  key={k}
                  onClick={() => setTab(k)}
                  className={`-mb-px border-b-2 px-3 py-2 text-sm ${tab === k ? "border-accent text-ink" : "border-transparent text-ink-faint hover:text-ink-dim"}`}
                >
                  {t("status." + k)} <span className="ml-1 text-xs text-ink-faint">{d.counts[k]}</span>
                </button>
              ))}
            </div>
            <TaskTable runs={(tab === "running" ? d.running : d.queued) ?? []} runnerName={runnerName} t={t} lang={lang} />
          </>
        )}
      </Page>
    </>
  );
}

function Stat({ icon, label, value, tone }: { icon: React.ReactNode; label: string; value: string; tone?: string }) {
  return (
    <div className="card flex items-center gap-3 p-3">
      <div className="grid h-9 w-9 shrink-0 place-items-center rounded-lg bg-surface text-ink-dim">{icon}</div>
      <div className="min-w-0">
        <div className="truncate text-xs text-ink-faint">{label}</div>
        <div className={`truncate text-lg font-semibold ${tone ?? "text-ink"}`}>{value}</div>
      </div>
    </div>
  );
}

function RunnerCard({ rn, t }: { rn: RunnerCapacity; t: (k: string) => string }) {
  const cap = rn.maxConcurrent > 0 ? rn.maxConcurrent : 0;
  const pct = cap > 0 ? Math.min(100, (rn.active / cap) * 100) : rn.active > 0 ? 100 : 0;
  const full = cap > 0 && rn.active >= cap;
  return (
    <div className="card p-3">
      <div className="flex items-center justify-between gap-2">
        <div className="flex min-w-0 items-center gap-2">
          <span className={`inline-block h-2.5 w-2.5 shrink-0 rounded-full ${rn.online ? "bg-success" : "bg-ink-faint/40"}`} />
          <span className="truncate text-sm font-medium text-ink">{rn.name}</span>
          {rn.builtin && <span className="chip text-[10px]">{t("cluster.builtin")}</span>}
        </div>
        <span className={`shrink-0 font-mono text-xs ${full ? "text-warn" : "text-ink-dim"}`}>
          {rn.active} / {cap > 0 ? cap : "∞"}
        </span>
      </div>
      <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-surface">
        <div className={`h-full ${full ? "bg-warn" : "bg-accent"}`} style={{ width: `${pct}%` }} />
      </div>
      <div className="mt-2 flex flex-wrap gap-1">
        {rn.tags.length ? rn.tags.map((tg) => <span key={tg} className="chip text-[10px]">{tg}</span>)
          : <span className="text-[11px] text-ink-faint">default</span>}
      </div>
    </div>
  );
}

function TaskTable({ runs, runnerName, t, lang }: { runs: Run[]; runnerName: (id?: string) => string; t: (k: string) => string; lang: string }) {
  if (!runs?.length) return <div className="card p-8 text-center text-sm text-ink-faint">{t("cluster.noTasks")}</div>;
  return (
    <div className="card overflow-hidden">
      <table className="w-full text-sm">
        <thead className="border-b border-border text-left text-xs uppercase text-ink-faint">
          <tr>
            <th className="px-4 py-2 font-medium">{t("cluster.colTask")}</th>
            <th className="px-4 py-2 font-medium">{t("cluster.colProject")}</th>
            <th className="px-4 py-2 font-medium">{t("cluster.colRunner")}</th>
            <th className="px-4 py-2 font-medium">{t("run.status")}</th>
            <th className="px-4 py-2 text-right font-medium">{t("cluster.colWhen")}</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-border">
          {runs.map((r) => (
            <tr key={r.id} className="hover:bg-surface/50">
              <td className="px-4 py-2"><Link to={`/runs/${r.id}`} className="text-ink hover:text-accent">{r.name}</Link></td>
              <td className="px-4 py-2 text-ink-dim">{r.projectName}</td>
              <td className="px-4 py-2 font-mono text-xs text-ink-dim">{r.status === "queued" ? "—" : runnerName(r.runnerId)}</td>
              <td className="px-4 py-2"><StatusBadge status={r.status} /></td>
              <td className="px-4 py-2 text-right text-xs text-ink-faint">{fmtRelative(r.startedAt || r.createdAt, lang)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
