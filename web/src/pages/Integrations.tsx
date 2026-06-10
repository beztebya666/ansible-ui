import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Copy, Trash2, Webhook } from "lucide-react";
import { Page, PageHeader } from "../components/PageHeader";
import { Checkbox, EmptyState, ErrorText, Field, Modal, Segmented, Spinner } from "../components/ui";
import { useConfirm } from "../components/feedback";
import { Select } from "../components/Select";
import { api, type Integration, type IntegrationAuth } from "../lib/api";
import { fmtRelative } from "../lib/format";
import { usePrefs } from "../lib/prefs";

function webhookURL(token: string) {
  return `${location.origin}/api/webhooks/${token}`;
}

export function Integrations() {
  const { t } = usePrefs();
  const integrations = useQuery({ queryKey: ["integrations"], queryFn: () => api.listIntegrations() });
  const [creating, setCreating] = useState(false);

  return (
    <>
      <PageHeader
        title={t("integ.title")}
        subtitle={t("integ.subtitle")}
        actions={
          <button className="btn-primary" onClick={() => setCreating(true)}>
            <Webhook size={15} /> {t("integ.new")}
          </button>
        }
      />
      <Page>
        {integrations.isLoading ? (
          <div className="flex justify-center py-20 text-ink-faint"><Spinner /></div>
        ) : integrations.data?.length ? (
          <div className="space-y-3">
            {integrations.data.map((i) => (
              <IntegrationRow key={i.id} integration={i} />
            ))}
          </div>
        ) : (
          <EmptyState
            icon={<Webhook size={28} />}
            title={t("integ.none")}
            description={t("integ.emptyDesc")}
            action={
              <button className="btn-primary" onClick={() => setCreating(true)}>
                <Webhook size={15} /> {t("integ.new")}
              </button>
            }
          />
        )}
      </Page>
      {creating && <IntegrationForm onClose={() => setCreating(false)} />}
    </>
  );
}

function IntegrationRow({ integration }: { integration: Integration }) {
  const { t, lang } = usePrefs();
  const confirm = useConfirm();
  const qc = useQueryClient();
  const [copied, setCopied] = useState(false);
  const url = webhookURL(integration.token);

  const del = useMutation({
    mutationFn: () => api.deleteIntegration(integration.id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["integrations"] }),
  });
  const copy = () => {
    navigator.clipboard?.writeText(url);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };

  return (
    <div className="card p-4">
      <div className="flex items-start justify-between gap-3">
        <div className="flex min-w-0 items-center gap-2.5">
          <Webhook size={18} className="text-accent" />
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <span className="truncate text-sm text-ink">{integration.name || integration.templateName}</span>
              {integration.authMethod && integration.authMethod !== "none" && (
                <span className="chip text-[10px] uppercase text-info">{integration.authMethod}</span>
              )}
              {integration.passPayload && <span className="chip text-[10px] uppercase">payload</span>}
            </div>
            <div className="truncate text-xs text-ink-faint">
              {integration.templateName}
              {integration.lastTriggeredAt ? ` · ${t("integ.lastTriggered")} ${fmtRelative(integration.lastTriggeredAt, lang)}` : ` · ${t("integ.neverTriggered")}`}
            </div>
          </div>
        </div>
        <button className="btn-ghost p-1.5" title={t("common.delete")} onClick={() => confirm({ title: t("integ.confirmDelete"), confirmLabel: t("common.delete"), cancelLabel: t("common.cancel") }).then((ok) => ok && del.mutate())}>
          <Trash2 size={15} />
        </button>
      </div>
      <div className="mt-3 flex items-center gap-2 rounded-lg border border-border bg-surface p-2">
        <code className="flex-1 truncate font-mono text-xs text-ink-dim">POST {url}</code>
        <button className="btn-ghost px-2 py-1 text-xs" onClick={copy}>
          <Copy size={13} /> {copied ? t("common.copied") : t("common.copy")}
        </button>
      </div>
    </div>
  );
}

const AUTH_METHODS: { value: IntegrationAuth; label?: string; labelKey?: string; hintKey: string }[] = [
  { value: "none", labelKey: "common.none", hintKey: "integ.authNoneHint" },
  { value: "token", labelKey: "integ.authToken", hintKey: "integ.authTokenHint" },
  { value: "hmac", label: "HMAC-SHA256", hintKey: "integ.authHmacHint" },
  { value: "github", label: "GitHub", hintKey: "integ.authGithubHint" },
  { value: "bitbucket", label: "Bitbucket", hintKey: "integ.authBitbucketHint" },
  { value: "basic", labelKey: "integ.authBasic", hintKey: "integ.authBasicHint" },
];

function IntegrationForm({ onClose }: { onClose: () => void }) {
  const { t } = usePrefs();
  const qc = useQueryClient();
  const templates = useQuery({ queryKey: ["templates"], queryFn: () => api.listTemplates() });
  const workflows = useQuery({ queryKey: ["allWorkflows"], queryFn: api.listAllWorkflows });
  const [target, setTarget] = useState<"template" | "workflow">("template");
  const [templateId, setTemplateId] = useState("");
  const [workflowId, setWorkflowId] = useState("");
  const [name, setName] = useState("");
  const [authMethod, setAuthMethod] = useState<IntegrationAuth>("none");
  const [authHeader, setAuthHeader] = useState("");
  const [authSecret, setAuthSecret] = useState("");
  const [passPayload, setPassPayload] = useState(false);
  const [aliases, setAliases] = useState("");
  const [error, setError] = useState("");

  const meta = AUTH_METHODS.find((m) => m.value === authMethod)!;
  const needsHeader = authMethod === "token" || authMethod === "hmac";
  const needsSecret = authMethod !== "none";

  const targetIds = target === "workflow" ? { workflowId } : { templateId };
  const targetSet = target === "workflow" ? !!workflowId : !!templateId;
  const create = useMutation({
    mutationFn: () =>
      api.createIntegration({
        ...targetIds,
        name,
        authMethod,
        authHeader: needsHeader ? authHeader || undefined : undefined,
        authSecret: needsSecret ? authSecret || undefined : undefined,
        passPayload,
        aliases: aliases.split(",").map((a) => a.trim()).filter(Boolean),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["integrations"] });
      onClose();
    },
    onError: (e) => setError((e as Error).message),
  });

  return (
    <Modal
      open
      onClose={onClose}
      title={t("integ.new")}
      subtitle={t("integ.formSub")}
      footer={
        <>
          <button className="btn-outline" onClick={onClose}>{t("common.cancel")}</button>
          <button className="btn-primary" onClick={() => { setError(""); create.mutate(); }} disabled={!targetSet || create.isPending}>
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
          <Field label={t("common.template")}>
            <Select
              value={templateId}
              onChange={setTemplateId}
              placeholder={t("common.selectTemplate")}
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
          <input className="input" value={name} onChange={(e) => setName(e.target.value)} placeholder="github-push" />
        </Field>
        <Field label={t("integ.auth")} hint={t(meta.hintKey)}>
          <Select
            value={authMethod}
            onChange={(v) => setAuthMethod(v as IntegrationAuth)}
            options={AUTH_METHODS.map((m) => ({ label: m.labelKey ? t(m.labelKey) : m.label!, value: m.value }))}
          />
        </Field>
        {needsHeader && (
          <Field label={t("integ.headerName")} hint={authMethod === "token" ? t("integ.headerHintToken") : t("integ.headerHintSig")}>
            <input className="input font-mono text-xs" value={authHeader} onChange={(e) => setAuthHeader(e.target.value)} placeholder={authMethod === "token" ? "X-Auth-Token" : "X-Signature"} />
          </Field>
        )}
        {needsSecret && (
          <Field label={t("integ.secret")} hint={authMethod === "basic" ? t("integ.secretHintBasic") : t("integ.secretHint")}>
            <input className="input font-mono text-xs" type="password" value={authSecret} onChange={(e) => setAuthSecret(e.target.value)} />
          </Field>
        )}
        <Checkbox
          checked={passPayload}
          onChange={setPassPayload}
          label={<>{t("integ.passPayload")} <code className="font-mono text-xs">webhook</code></>}
        />
        <Field label={t("integ.aliases")} hint={t("integ.aliasesHint")}>
          <input className="input font-mono text-xs" value={aliases} onChange={(e) => setAliases(e.target.value)} placeholder="deploy-prod, ci-main" />
        </Field>
        {error && <ErrorText>{error}</ErrorText>}
      </div>
    </Modal>
  );
}
