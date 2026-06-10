import { useState, type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import { Activity, CheckCircle2, Clock, Hourglass } from "lucide-react";
import { Page, PageHeader } from "../components/PageHeader";
import { Select } from "../components/Select";
import { Spinner } from "../components/ui";
import { api, type DayBucket } from "../lib/api";
import { usePrefs } from "../lib/prefs";

function fmtDur(sec: number): string {
  if (sec <= 0) return "—";
  if (sec < 60) return `${sec.toFixed(sec < 10 ? 1 : 0)}s`;
  const m = Math.floor(sec / 60);
  const s = Math.round(sec % 60);
  if (m < 60) return `${m}m ${s}s`;
  return `${Math.floor(m / 60)}h ${m % 60}m`;
}

export function Insights() {
  const { t } = usePrefs();
  const projects = useQuery({ queryKey: ["projects"], queryFn: api.listProjects });
  const [projectId, setProjectId] = useState("");
  const [days, setDays] = useState(14);
  const q = useQuery({
    queryKey: ["insights", projectId, days],
    queryFn: () => api.getInsights({ projectId: projectId || undefined, days }),
    refetchInterval: 15000,
  });
  const data = q.data;

  return (
    <>
      <PageHeader title={t("ins.title")} subtitle={t("ins.subtitle")} />
      <Page>
        <div className="mb-4 flex flex-wrap items-center gap-2">
          <Select
            className="w-56"
            value={projectId}
            onChange={setProjectId}
            options={[{ label: t("ins.allProjects"), value: "" }, ...(projects.data ?? []).map((p) => ({ label: p.name, value: p.id }))]}
          />
          <Select
            className="w-32"
            value={String(days)}
            onChange={(v) => setDays(Number(v))}
            options={[
              { label: t("ins.last7"), value: "7" },
              { label: t("ins.last14"), value: "14" },
              { label: t("ins.last30"), value: "30" },
            ]}
          />
        </div>

        {q.isLoading || !data ? (
          <div className="flex justify-center py-20 text-ink-faint"><Spinner /></div>
        ) : (
          <div className="space-y-4">
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
              <StatCard icon={<Activity size={16} />} label={t("ins.totalRuns")} value={String(data.total)} tone="text-info" />
              <StatCard icon={<CheckCircle2 size={16} />} label={t("ins.successRate")} value={data.total ? `${data.successRate.toFixed(1)}%` : "—"} tone="text-success" />
              <StatCard icon={<Clock size={16} />} label={t("ins.avgDuration")} value={fmtDur(data.avgDurationSec)} tone="text-warn" />
              <StatCard icon={<Hourglass size={16} />} label={t("ins.avgWait")} value={fmtDur(data.avgWaitSec)} tone="text-ink-dim" />
            </div>

            <div className="card p-4">
              <div className="mb-3 flex items-center justify-between">
                <h2 className="text-sm font-semibold text-ink">{t("ins.runsPerDay")}</h2>
                <Legend />
              </div>
              {data.total === 0 ? (
                <p className="py-10 text-center text-sm text-ink-faint">{t("ins.noRuns")}</p>
              ) : (
                <DayChart days={data.perDay} />
              )}
            </div>

            {data.byTemplate?.length > 0 && (
              <div className="card overflow-hidden">
                <h2 className="border-b border-border px-4 py-3 text-sm font-semibold text-ink">{t("ins.byTemplate")}</h2>
                <table className="w-full text-sm">
                  <thead className="text-left text-xs uppercase text-ink-faint">
                    <tr>
                      <th className="px-4 py-2 font-medium">{t("ins.template")}</th>
                      <th className="px-4 py-2 text-right font-medium">{t("ins.runs")}</th>
                      <th className="px-4 py-2 text-right font-medium">{t("ins.successRate")}</th>
                      <th className="px-4 py-2 text-right font-medium">{t("ins.avgDuration")}</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-border">
                    {data.byTemplate.map((ti) => {
                      const ok = ti.successRate >= 90 ? "text-success" : ti.successRate >= 50 ? "text-warn" : "text-danger";
                      return (
                        <tr key={ti.templateId} className="hover:bg-surface/50">
                          <td className="px-4 py-2 text-ink">{ti.templateName}</td>
                          <td className="px-4 py-2 text-right font-mono text-ink-dim">{ti.runs}</td>
                          <td className={`px-4 py-2 text-right font-mono ${ok}`}>{ti.successRate.toFixed(0)}%</td>
                          <td className="px-4 py-2 text-right font-mono text-ink-dim">{fmtDur(ti.avgDurationSec)}</td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        )}
      </Page>
    </>
  );
}

function StatCard({ icon, label, value, tone }: { icon: ReactNode; label: string; value: string; tone: string }) {
  return (
    <div className="card p-4">
      <div className={`mb-1 flex items-center gap-1.5 text-xs text-ink-faint`}>
        <span className={tone}>{icon}</span> {label}
      </div>
      <div className="text-2xl font-semibold text-ink">{value}</div>
    </div>
  );
}

function Legend() {
  const { t } = usePrefs();
  const item = (cls: string, label: string) => (
    <span className="flex items-center gap-1 text-xs text-ink-faint">
      <span className={`inline-block h-2.5 w-2.5 rounded-sm ${cls}`} /> {label}
    </span>
  );
  return (
    <div className="flex items-center gap-3">
      {item("bg-success", t("status.success"))}
      {item("bg-danger", t("status.failed"))}
      {item("bg-ink-faint/50", t("ins.other"))}
    </div>
  );
}

// DayChart is a hand-rolled stacked bar chart (no chart lib) — one column per day,
// height proportional to that day's run count, segmented by outcome.
function DayChart({ days }: { days: DayBucket[] }) {
  const max = Math.max(1, ...days.map((d) => d.total));
  const seg = (n: number, cls: string) =>
    n > 0 ? <div className={cls} style={{ height: `${(n / max) * 160}px` }} /> : null;
  return (
    <div className="flex items-end gap-1 overflow-x-auto pb-1" style={{ minHeight: 184 }}>
      {days.map((d) => (
        <div key={d.date} className="group flex min-w-[14px] flex-1 flex-col items-center justify-end">
          <div className="relative flex w-full max-w-[28px] flex-col-reverse overflow-hidden rounded-sm">
            {seg(d.success, "bg-success")}
            {seg(d.failed, "bg-danger")}
            {seg(d.other, "bg-ink-faint/50")}
            {d.total === 0 && <div className="h-[2px] w-full bg-border" />}
            <div className="pointer-events-none absolute -top-7 left-1/2 z-10 -translate-x-1/2 whitespace-nowrap rounded bg-surface-2 px-1.5 py-0.5 text-[10px] text-ink opacity-0 shadow transition-opacity group-hover:opacity-100">
              {d.date.slice(5)} · {d.total}
            </div>
          </div>
          <span className="mt-1 text-[9px] text-ink-faint">{d.date.slice(8)}</span>
        </div>
      ))}
    </div>
  );
}
