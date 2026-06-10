import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import clsx from "clsx";
import { Server, Tag, Trash2 } from "lucide-react";
import { Page, PageHeader } from "../components/PageHeader";
import { EmptyState, Spinner } from "../components/ui";
import { useConfirm } from "../components/feedback";
import { api, type Runner } from "../lib/api";
import { usePrefs } from "../lib/prefs";
import { useAuth } from "../lib/auth";
import { fmtRelative } from "../lib/format";

export function Runners() {
  const { t, lang } = usePrefs();
  const { user } = useAuth();
  const isAdmin = user?.role === "admin";
  const runners = useQuery({
    queryKey: ["runners"],
    queryFn: api.listRunners,
    refetchInterval: 15000, // keep online/offline fresh even between WS events
  });

  return (
    <>
      <PageHeader title={t("runner.title")} subtitle={t("runner.subtitle")} />
      <Page>
        {runners.isLoading ? (
          <div className="flex justify-center py-20 text-ink-faint"><Spinner /></div>
        ) : runners.data?.length ? (
          <div className="card divide-y divide-border">
            {runners.data.map((rn) => (
              <RunnerRow key={rn.id} runner={rn} isAdmin={isAdmin} lang={lang} />
            ))}
          </div>
        ) : (
          <EmptyState icon={<Server size={28} />} title={t("runner.none")} description={t("runner.emptyDesc")} />
        )}
      </Page>
    </>
  );
}

function RunnerRow({ runner, isAdmin, lang }: { runner: Runner; isAdmin: boolean; lang: string }) {
  const { t } = usePrefs();
  const qc = useQueryClient();
  const confirm = useConfirm();
  const online = runner.status === "online";
  const del = useMutation({
    mutationFn: () => api.deleteRunner(runner.id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["runners"] }),
  });

  return (
    <div className="flex items-center gap-3 px-4 py-3">
      <Server size={18} className={online ? "text-accent" : "text-ink-faint"} />
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span className="truncate text-sm text-ink">{runner.name || runner.url}</span>
          {runner.builtin && <span className="chip text-[10px] uppercase">{t("runner.builtin")}</span>}
          <span className={clsx("inline-flex items-center gap-1.5 text-xs", online ? "text-success" : "text-ink-faint")}>
            <span className={clsx("h-1.5 w-1.5 rounded-full", online ? "bg-success" : "bg-ink-faint/50")} />
            {online ? t("runner.online") : t("runner.offline")}
          </span>
        </div>
        <div className="mt-0.5 flex flex-wrap items-center gap-1.5 truncate font-mono text-xs text-ink-faint">
          <span>{runner.url}</span>
          {runner.platform && <span>· {runner.platform}</span>}
          <span>· {runner.maxConcurrent && runner.maxConcurrent > 0 ? t("runner.cap").replace("{n}", String(runner.maxConcurrent)) : t("runner.capUnlimited")}</span>
          {!runner.builtin && runner.lastSeenAt && <span>· {t("runner.seen")} {fmtRelative(runner.lastSeenAt, lang)}</span>}
        </div>
      </div>
      <div className="flex shrink-0 flex-wrap items-center justify-end gap-1">
        {runner.tags?.length ? (
          runner.tags.map((tg) => (
            <span key={tg} className="chip flex items-center gap-1 text-xs"><Tag size={11} /> {tg}</span>
          ))
        ) : (
          <span className="text-xs text-ink-faint/60">{t("runner.noTags")}</span>
        )}
      </div>
      {isAdmin && !runner.builtin && (
        <button
          className="btn-ghost p-1.5"
          title={t("common.delete")}
          onClick={() => confirm({ title: t("runner.remove"), body: runner.name || runner.url }).then((ok) => ok && del.mutate())}
        >
          <Trash2 size={15} />
        </button>
      )}
    </div>
  );
}
