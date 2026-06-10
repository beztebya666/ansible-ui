import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import clsx from "clsx";
import { History, Search, User as UserIcon } from "lucide-react";
import { Page, PageHeader } from "../components/PageHeader";
import { EmptyState, Spinner } from "../components/ui";
import { Select } from "../components/Select";
import { api, type Activity } from "../lib/api";
import { actionLabel, fmtRelative, fmtTime } from "../lib/format";
import { usePrefs } from "../lib/prefs";

function tone(action: string): string {
  if (action.includes("success")) return "bg-success";
  if (action.includes("failed")) return "bg-danger";
  if (action.includes("canceled")) return "bg-warn";
  if (action.includes("deleted")) return "bg-danger";
  if (action.includes("launched") || action.includes("created")) return "bg-info";
  return "bg-ink-faint";
}

const cap = (s: string) => s.charAt(0).toUpperCase() + s.slice(1);

export function Audit() {
  const { t, lang } = usePrefs();
  const acts = useQuery({
    queryKey: ["activity"],
    queryFn: () => api.listActivity(500),
    refetchInterval: 3000,
  });
  const [group, setGroup] = useState("");
  const [search, setSearch] = useState("");

  const groups = useMemo(() => {
    const s = new Set<string>();
    for (const a of acts.data ?? []) s.add(a.action.split(".")[0]);
    return Array.from(s).sort();
  }, [acts.data]);

  const rows = (acts.data ?? []).filter((a) => {
    if (group && !a.action.startsWith(group + ".")) return false;
    if (search) {
      const q = search.toLowerCase();
      if (!a.actor.toLowerCase().includes(q) && !a.target.toLowerCase().includes(q) && !a.action.toLowerCase().includes(q)) {
        return false;
      }
    }
    return true;
  });

  return (
    <>
      <PageHeader title={t("audit.title")} subtitle={t("audit.subtitle")} />
      <Page>
        <div className="mb-4 flex flex-wrap items-center gap-2">
          <div className="relative">
            <Search size={14} className="pointer-events-none absolute left-2.5 top-2.5 text-ink-faint" />
            <input className="input w-56 pl-8" placeholder={t("audit.search")} value={search} onChange={(e) => setSearch(e.target.value)} />
          </div>
          <Select
            className="w-44"
            value={group}
            onChange={setGroup}
            options={[{ label: t("audit.allCategories"), value: "" }, ...groups.map((g) => ({ label: cap(g), value: g }))]}
          />
          <span className="ml-auto text-xs text-ink-faint">{rows.length} {t("audit.events")}</span>
        </div>

        {acts.isLoading ? (
          <div className="flex justify-center py-20 text-ink-faint"><Spinner /></div>
        ) : rows.length ? (
          <div className="card divide-y divide-border">
            {rows.map((a: Activity) => (
              <div key={a.id} className="flex items-center gap-3 px-4 py-2.5 text-sm">
                <span className={clsx("h-2 w-2 shrink-0 rounded-full", tone(a.action))} />
                <span className="inline-flex w-40 shrink-0 items-center gap-1.5 text-ink-dim">
                  <UserIcon size={13} className="text-ink-faint" />
                  <span className="truncate">{a.actor || t("audit.system")}</span>
                </span>
                <span className="w-44 shrink-0 truncate text-xs font-medium text-ink-dim" title={a.action}>{actionLabel(a.action, t)}</span>
                <span className="min-w-0 flex-1 truncate text-ink">
                  {a.target}
                  {a.detail ? <span className="text-ink-faint"> · {a.detail}</span> : null}
                </span>
                <span className="shrink-0 text-xs text-ink-faint" title={fmtTime(a.createdAt)}>{fmtRelative(a.createdAt, lang)}</span>
              </div>
            ))}
          </div>
        ) : (
          <EmptyState icon={<History size={28} />} title={t("audit.none")} description={t("audit.noneHint")} />
        )}
      </Page>
    </>
  );
}
