import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import clsx from "clsx";
import { HardDrive, RefreshCw, Search, Trash2 } from "lucide-react";
import { Page, PageHeader } from "../components/PageHeader";
import { EmptyState, Modal, SearchBox, Spinner } from "../components/ui";
import { useConfirm, useToast } from "../components/feedback";
import { api, type HostFacts, type HostMonitor } from "../lib/api";
import { useAuth } from "../lib/auth";
import { usePrefs } from "../lib/prefs";
import { fmtRelative } from "../lib/format";

const monMeta: Record<string, { dot: string; text: string }> = {
  up: { dot: "bg-success", text: "text-success" },
  down: { dot: "bg-danger", text: "text-danger" },
  unknown: { dot: "bg-ink-faint", text: "text-ink-faint" },
};

interface Row { host: string; facts?: HostFacts; monitor?: HostMonitor; }

export function Hosts() {
  const { t, lang } = usePrefs();
  const { user } = useAuth();
  const confirm = useConfirm();
  const toast = useToast();
  const qc = useQueryClient();
  const isAdmin = user?.role === "admin";
  const hosts = useQuery({ queryKey: ["hosts"], queryFn: api.listHosts, refetchInterval: 15000 });
  const monitors = useQuery({ queryKey: ["monitors"], queryFn: api.listMonitors, refetchInterval: 15000 });
  const [q, setQ] = useState("");
  const [open, setOpen] = useState<string | null>(null);

  const del = useMutation({
    mutationFn: (host: string) => api.deleteHost(host),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ["hosts"] }); toast.success(t("hosts.removed")); },
    onError: (e) => toast.error((e as Error).message),
  });
  const check = useMutation({
    mutationFn: () => api.checkMonitors(),
    onSuccess: () => { setTimeout(() => qc.invalidateQueries({ queryKey: ["monitors"] }), 1500); toast.success(t("hosts.checking")); },
    onError: (e) => toast.error((e as Error).message),
  });

  // Merge fact rows + monitor rows (a host may have either or both).
  const byHost = new Map<string, Row>();
  for (const f of hosts.data ?? []) byHost.set(f.host, { host: f.host, facts: f });
  for (const m of monitors.data ?? []) byHost.set(m.host, { ...(byHost.get(m.host) ?? { host: m.host }), monitor: m });
  const all = [...byHost.values()];
  const ql = q.trim().toLowerCase();
  const rows = all.filter((r) => !ql || r.host.toLowerCase().includes(ql) || (r.facts?.distro ?? "").toLowerCase().includes(ql));
  const hasData = all.length > 0;
  const anyMonitored = (monitors.data?.length ?? 0) > 0;

  return (
    <>
      <PageHeader title={t("hosts.title")} subtitle={t("hosts.subtitle")}
        actions={anyMonitored ? (
          <button className="btn-outline" onClick={() => check.mutate()} disabled={check.isPending}>
            <RefreshCw size={15} className={check.isPending ? "animate-spin" : ""} /> {t("hosts.checkNow")}
          </button>
        ) : undefined}
      />
      <Page>
        {hosts.isLoading || monitors.isLoading ? (
          <div className="flex justify-center py-20 text-ink-faint"><Spinner /></div>
        ) : !hasData ? (
          <EmptyState icon={<HardDrive size={28} />} title={t("hosts.none")} description={t("hosts.noneHint")} />
        ) : (
          <>
            {all.length > 6 && (
              <div className="mb-3 max-w-sm"><SearchBox value={q} onChange={setQ} placeholder={t("hosts.search")} /></div>
            )}
            <div className="card overflow-hidden">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-border text-left text-xs uppercase text-ink-faint">
                    <th className="px-4 py-2.5 font-medium">{t("hosts.host")}</th>
                    <th className="px-4 py-2.5 font-medium">{t("hosts.status")}</th>
                    <th className="px-4 py-2.5 font-medium">{t("hosts.os")}</th>
                    <th className="px-4 py-2.5 font-medium">{t("hosts.ip")}</th>
                    <th className="px-4 py-2.5 font-medium">{t("hosts.kernel")}</th>
                    <th className="px-4 py-2.5 font-medium">{t("hosts.gathered")}</th>
                    <th className="px-4 py-2.5" />
                  </tr>
                </thead>
                <tbody className="divide-y divide-border">
                  {rows.map((r) => {
                    const f = r.facts;
                    const m = r.monitor;
                    const mm = m ? (monMeta[m.status] ?? monMeta.unknown) : null;
                    return (
                    <tr key={r.host} className={clsx(f && "cursor-pointer hover:bg-surface")} onClick={() => f && setOpen(r.host)}>
                      <td className="px-4 py-2.5">
                        <span className="flex items-center gap-2 font-mono text-ink"><HardDrive size={14} className="text-accent" /> {r.host}</span>
                      </td>
                      <td className="px-4 py-2.5">
                        {mm ? <span className={clsx("flex items-center gap-1.5 text-xs", mm.text)}><span className={clsx("h-1.5 w-1.5 rounded-full", mm.dot)} />{t(`hosts.st.${m!.status}`)}</span> : <span className="text-xs text-ink-faint">—</span>}
                      </td>
                      <td className="px-4 py-2.5 text-ink-dim">{f?.distro || f?.os || "—"}</td>
                      <td className="px-4 py-2.5 font-mono text-xs text-ink-dim">{f?.ip || "—"}</td>
                      <td className="px-4 py-2.5 font-mono text-xs text-ink-faint">{f?.kernel || "—"}</td>
                      <td className="px-4 py-2.5 text-xs text-ink-faint">{f ? fmtRelative(f.gatheredAt, lang) : "—"}</td>
                      <td className="px-4 py-2.5 text-right">
                        {isAdmin && f && (
                          <button className="btn-ghost p-1.5" title={t("common.delete")}
                            onClick={(e) => { e.stopPropagation(); confirm({ title: t("hosts.remove"), body: r.host }).then((ok) => ok && del.mutate(r.host)); }}>
                            <Trash2 size={15} />
                          </button>
                        )}
                      </td>
                    </tr>
                  );})}
                </tbody>
              </table>
            </div>
          </>
        )}
        {open && <HostFactsModal host={open} onClose={() => setOpen(null)} />}
      </Page>
    </>
  );
}

function HostFactsModal({ host, onClose }: { host: string; onClose: () => void }) {
  const { t } = usePrefs();
  const [q, setQ] = useState("");
  const data = useQuery({ queryKey: ["host", host], queryFn: () => api.getHost(host) });
  const facts = (data.data?.facts ?? {}) as Record<string, unknown>;
  const entries = Object.entries(facts)
    .filter(([k]) => !q.trim() || k.toLowerCase().includes(q.trim().toLowerCase()))
    .sort(([a], [b]) => a.localeCompare(b));
  return (
    <Modal open onClose={onClose} title={`${t("hosts.facts")} · ${host}`}>
      {data.isLoading ? (
        <div className="flex justify-center py-10 text-ink-faint"><Spinner /></div>
      ) : (
        <div className="space-y-3">
          <div className="relative">
            <Search size={14} className="pointer-events-none absolute left-2.5 top-2.5 text-ink-faint" />
            <input className="input pl-8" value={q} onChange={(e) => setQ(e.target.value)} placeholder={t("hosts.filterFacts")} />
          </div>
          <div className="max-h-[55vh] overflow-auto rounded-lg border border-border">
            <table className="w-full text-xs">
              <tbody className="divide-y divide-border">
                {entries.map(([k, v]) => (
                  <tr key={k} className="align-top">
                    <td className="whitespace-nowrap px-3 py-1.5 font-mono text-ink-dim">{k}</td>
                    <td className="px-3 py-1.5 font-mono text-ink">
                      <pre className="whitespace-pre-wrap break-all">{typeof v === "object" ? JSON.stringify(v, null, 2) : String(v)}</pre>
                    </td>
                  </tr>
                ))}
                {entries.length === 0 && <tr><td className="px-3 py-4 text-ink-faint">{t("hosts.noFacts")}</td></tr>}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </Modal>
  );
}
