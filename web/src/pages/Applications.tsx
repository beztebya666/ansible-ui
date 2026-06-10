import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Boxes, Pencil, Plus, Terminal, Trash2 } from "lucide-react";
import { Page, PageHeader } from "../components/PageHeader";
import { EmptyState, ErrorText, Field, Modal, Spinner, Toggle } from "../components/ui";
import { useConfirm } from "../components/feedback";
import { api, type Application } from "../lib/api";
import { appMeta } from "../lib/apps";
import { usePrefs } from "../lib/prefs";

export function Applications() {
  const { t } = usePrefs();
  const confirm = useConfirm();
  const qc = useQueryClient();
  const apps = useQuery({ queryKey: ["applications"], queryFn: api.listApplications });
  const [editing, setEditing] = useState<Application | null>(null);
  const [creating, setCreating] = useState(false);

  const toggle = useMutation({
    mutationFn: (a: Application) => api.updateApplication(a.id, { active: !a.active }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["applications"] }),
  });
  const del = useMutation({
    mutationFn: (id: string) => api.deleteApplication(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["applications"] }),
  });

  return (
    <>
      <PageHeader
        title={t("app.title")}
        subtitle={t("app.subtitle")}
        actions={
          <button className="btn-primary" onClick={() => setCreating(true)}>
            <Plus size={15} /> {t("app.new")}
          </button>
        }
      />
      <Page>
        {apps.isLoading ? (
          <div className="flex justify-center py-20 text-ink-faint"><Spinner /></div>
        ) : apps.data?.length ? (
          <div className="card divide-y divide-border">
            {apps.data.map((a) => {
              const Icon = a.kind === "builtin" ? appMeta(a.id).icon : Terminal;
              return (
                <div key={a.id} className="flex items-center gap-3 px-4 py-3">
                  <Toggle checked={a.active} onChange={() => toggle.mutate(a)} label="" />
                  <Icon size={17} className={a.active ? "text-accent" : "text-ink-faint"} />
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <span className="truncate text-sm text-ink">{a.name}</span>
                      <span className="chip font-mono text-[10px]">{a.id}</span>
                      <span className="chip text-[10px] uppercase">{a.kind === "builtin" ? t("app.builtin") : t("app.custom")}</span>
                      {a.bin && <span className="chip font-mono text-[10px] text-ink-faint">{a.bin}</span>}
                    </div>
                    {a.args?.length ? (
                      <div className="truncate font-mono text-xs text-ink-faint">{t("app.argsLabel")}: {a.args.join(" ")}</div>
                    ) : null}
                  </div>
                  <button className="btn-ghost p-1.5" title={t("common.edit")} onClick={() => setEditing(a)}>
                    <Pencil size={15} />
                  </button>
                  {a.kind === "custom" && (
                    <button className="btn-ghost p-1.5" title={t("common.delete")} onClick={() => confirm({ title: t("app.confirmDelete"), body: a.name }).then((ok) => ok && del.mutate(a.id))}>
                      <Trash2 size={15} />
                    </button>
                  )}
                </div>
              );
            })}
          </div>
        ) : (
          <EmptyState icon={<Boxes size={28} />} title={t("app.none")} />
        )}
      </Page>
      {(creating || editing) && <AppForm app={editing} onClose={() => { setCreating(false); setEditing(null); }} />}
    </>
  );
}

function AppForm({ app, onClose }: { app: Application | null; onClose: () => void }) {
  const { t } = usePrefs();
  const qc = useQueryClient();
  const isCustom = !app || app.kind === "custom";
  const [name, setName] = useState(app?.name ?? "");
  const [icon, setIcon] = useState(app?.icon ?? "");
  const [bin, setBin] = useState(app?.bin ?? "");
  const [args, setArgs] = useState((app?.args ?? []).join(" "));
  const [priority, setPriority] = useState(app?.priority ?? 10);
  const [error, setError] = useState("");

  const save = useMutation({
    mutationFn: () => {
      const body = {
        name,
        icon,
        bin: isCustom ? bin : undefined,
        args: args.trim() ? args.trim().split(/\s+/) : [],
        priority,
      };
      return app ? api.updateApplication(app.id, body) : api.createApplication(body);
    },
    onSuccess: () => { qc.invalidateQueries({ queryKey: ["applications"] }); onClose(); },
    onError: (e) => setError((e as Error).message),
  });

  return (
    <Modal
      open
      onClose={onClose}
      title={app ? `${t("common.edit")} ${app.name}` : t("app.new")}
      subtitle={isCustom ? t("app.customSub") : t("app.builtinSub")}
      footer={
        <>
          <button className="btn-outline" onClick={onClose}>{t("common.cancel")}</button>
          <button className="btn-primary" onClick={() => { setError(""); save.mutate(); }} disabled={!name || (isCustom && !bin) || save.isPending}>
            {save.isPending ? <Spinner /> : null} {t("common.save")}
          </button>
        </>
      }
    >
      <div className="space-y-4">
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label={t("common.name")}>
            <input className="input" value={name} onChange={(e) => setName(e.target.value)} placeholder="My Tool" autoFocus />
          </Field>
          <Field label={t("app.priority")} hint={t("app.priorityHint")}>
            <input className="input" type="number" value={priority} onChange={(e) => setPriority(Number(e.target.value))} />
          </Field>
        </div>
        {isCustom && (
          <Field label={t("app.binary")} hint={t("app.binaryHint")}>
            <input className="input font-mono text-xs" value={bin} onChange={(e) => setBin(e.target.value)} placeholder="/usr/local/bin/mytool" />
          </Field>
        )}
        <Field label={t("app.baseArgs")} hint={t("app.baseArgsHint")}>
          <input className="input font-mono text-xs" value={args} onChange={(e) => setArgs(e.target.value)} placeholder="run --yes" />
        </Field>
        <Field label={t("app.icon")} hint={t("app.iconHint")}>
          <input className="input" value={icon} onChange={(e) => setIcon(e.target.value)} placeholder="terminal" />
        </Field>
        {error && <ErrorText>{error}</ErrorText>}
      </div>
    </Modal>
  );
}
