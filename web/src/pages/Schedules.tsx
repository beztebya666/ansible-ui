import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import clsx from "clsx";
import { CalendarClock, Plus, Trash2 } from "lucide-react";
import { Page, PageHeader } from "../components/PageHeader";
import { EmptyState, ErrorText, Field, Modal, Segmented, Spinner, Toggle } from "../components/ui";
import { useConfirm } from "../components/feedback";
import { Select } from "../components/Select";
import { api, type Schedule } from "../lib/api";
import { fmtTime, fmtRelative } from "../lib/format";
import { usePrefs } from "../lib/prefs";

const presets = [
  { key: "sched.preset5min", cron: "*/5 * * * *" },
  { key: "sched.presetHourly", cron: "0 * * * *" },
  { key: "sched.presetDaily", cron: "0 2 * * *" },
  { key: "sched.presetWeekly", cron: "0 3 * * 1" },
];

export function Schedules() {
  const { t } = usePrefs();
  const schedules = useQuery({ queryKey: ["schedules"], queryFn: () => api.listSchedules(), refetchInterval: 15000 });
  const [creating, setCreating] = useState(false);

  return (
    <>
      <PageHeader
        title={t("sched.title")}
        subtitle={t("sched.subtitle")}
        actions={
          <button className="btn-primary" onClick={() => setCreating(true)}>
            <Plus size={15} /> {t("sched.new")}
          </button>
        }
      />
      <Page>
        {schedules.isLoading ? (
          <div className="flex justify-center py-20 text-ink-faint"><Spinner /></div>
        ) : schedules.data?.length ? (
          <div className="card divide-y divide-border">
            {schedules.data.map((sc) => (
              <ScheduleRow key={sc.id} schedule={sc} />
            ))}
          </div>
        ) : (
          <EmptyState
            icon={<CalendarClock size={28} />}
            title={t("sched.none")}
            description={t("sched.emptyDesc")}
            action={
              <button className="btn-primary" onClick={() => setCreating(true)}>
                <Plus size={15} /> {t("sched.new")}
              </button>
            }
          />
        )}
      </Page>
      {creating && <ScheduleForm onClose={() => setCreating(false)} />}
    </>
  );
}

function ScheduleRow({ schedule }: { schedule: Schedule }) {
  const { t, lang } = usePrefs();
  const confirm = useConfirm();
  const qc = useQueryClient();
  const toggle = useMutation({
    mutationFn: () => api.updateSchedule(schedule.id, { active: !schedule.active }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["schedules"] }),
  });
  const del = useMutation({
    mutationFn: () => api.deleteSchedule(schedule.id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["schedules"] }),
  });

  return (
    <div className="flex items-center gap-3 px-4 py-3">
      <CalendarClock size={18} className={schedule.active ? "text-accent" : "text-ink-faint"} />
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span className="truncate text-sm text-ink">{schedule.name || schedule.templateName}</span>
          {schedule.workflowId && <span className="chip text-[10px] uppercase text-accent">{t("nav.workflows")}</span>}
          {schedule.once ? (
            <span className="chip text-info">{t("sched.onceTag")} · {fmtTime(schedule.nextRunAt)}</span>
          ) : (
            <span className="chip font-mono">{schedule.cron}</span>
          )}
        </div>
        <div className="truncate text-xs text-ink-faint">
          {schedule.templateName}
          {schedule.active && schedule.nextRunAt ? <> · {t("sched.next")} {fmtTime(schedule.nextRunAt)}</> : <> · {schedule.once ? t("sched.statusDone") : t("sched.statusPaused")}</>}
          {schedule.lastRunAt ? <> · {t("sched.last")} {fmtRelative(schedule.lastRunAt, lang)}</> : null}
        </div>
      </div>
      <button
        onClick={() => toggle.mutate()}
        className={clsx("text-xs", schedule.active ? "text-success" : "text-ink-faint")}
        title={schedule.active ? t("sched.pause") : t("sched.resume")}
      >
        {schedule.active ? t("sched.statusActive") : t("sched.statusPaused")}
      </button>
      <button
        className="btn-ghost p-1.5"
        title={t("common.delete")}
        onClick={() => confirm(t("sched.confirmDelete")).then((ok) => ok && del.mutate())}
      >
        <Trash2 size={15} />
      </button>
    </div>
  );
}

function ScheduleForm({ onClose }: { onClose: () => void }) {
  const { t } = usePrefs();
  const qc = useQueryClient();
  const templates = useQuery({ queryKey: ["templates"], queryFn: () => api.listTemplates() });
  const workflows = useQuery({ queryKey: ["allWorkflows"], queryFn: api.listAllWorkflows });
  const [target, setTarget] = useState<"template" | "workflow">("template");
  const [templateId, setTemplateId] = useState("");
  const [workflowId, setWorkflowId] = useState("");
  const [name, setName] = useState("");
  const [mode, setMode] = useState<"cron" | "once">("cron");
  const [cron, setCron] = useState("0 2 * * *");
  const [runAt, setRunAt] = useState("");
  const [active, setActive] = useState(true);
  const [error, setError] = useState("");

  const targetIds = target === "workflow" ? { workflowId } : { templateId };
  const targetSet = target === "workflow" ? !!workflowId : !!templateId;
  const create = useMutation({
    mutationFn: () => {
      if (mode === "once") {
        if (!runAt) throw new Error(t("sched.pickDate"));
        return api.createSchedule({ ...targetIds, name, once: true, runAt: new Date(runAt).toISOString(), active });
      }
      return api.createSchedule({ ...targetIds, name, cron, active });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["schedules"] });
      onClose();
    },
    onError: (e) => setError((e as Error).message),
  });

  return (
    <Modal
      open
      onClose={onClose}
      title={t("sched.new")}
      subtitle={t("sched.cronHelp")}
      footer={
        <>
          <button className="btn-outline" onClick={onClose}>{t("common.cancel")}</button>
          <button className="btn-primary" onClick={() => { setError(""); create.mutate(); }} disabled={!targetSet || (mode === "cron" ? !cron : !runAt) || create.isPending}>
            {create.isPending ? <Spinner /> : null} {t("common.create")}
          </button>
        </>
      }
    >
      <div className="space-y-4">
        <Segmented
          value={target}
          onChange={(v) => setTarget(v as "template" | "workflow")}
          options={[
            { label: t("sched.targetTemplate"), value: "template" },
            { label: t("sched.targetWorkflow"), value: "workflow" },
          ]}
        />
        {target === "template" ? (
          <Field label={t("sched.template")}>
            <Select
              value={templateId}
              onChange={setTemplateId}
              placeholder={t("sched.selectTemplate")}
              options={(templates.data ?? []).map((tpl) => ({ label: tpl.name, value: tpl.id }))}
            />
          </Field>
        ) : (
          <Field label={t("nav.workflows")}>
            <Select
              value={workflowId}
              onChange={setWorkflowId}
              placeholder={t("sched.selectWorkflow")}
              options={(workflows.data ?? []).map((wf) => ({ label: wf.name, value: wf.id }))}
            />
          </Field>
        )}
        <Field label={t("common.name")} hint={t("common.optional")}>
          <input className="input" value={name} onChange={(e) => setName(e.target.value)} placeholder={t("sched.namePlaceholder")} />
        </Field>
        <Segmented
          value={mode}
          onChange={(m) => setMode(m as "cron" | "once")}
          options={[
            { label: t("sched.recurring"), value: "cron" },
            { label: t("sched.runOnce"), value: "once" },
          ]}
        />
        {mode === "cron" ? (
          <>
            <Field label={t("sched.cronExpr")}>
              <input className="input font-mono" value={cron} onChange={(e) => setCron(e.target.value)} placeholder="0 2 * * *" />
            </Field>
            <div className="flex flex-wrap gap-1.5">
              {presets.map((p) => (
                <button key={p.cron} type="button" className="chip hover:border-accent/50 hover:text-ink" onClick={() => setCron(p.cron)}>
                  {t(p.key)}
                </button>
              ))}
            </div>
          </>
        ) : (
          <Field label={t("sched.runAt")} hint={t("sched.runAtHint")}>
            <input className="input" type="datetime-local" value={runAt} onChange={(e) => setRunAt(e.target.value)} />
          </Field>
        )}
        <Toggle checked={active} onChange={setActive} label={t("common.active")} />
        {error && <ErrorText>{error}</ErrorText>}
      </div>
    </Modal>
  );
}
