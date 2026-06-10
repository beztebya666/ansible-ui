import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Boxes, Cloud, KeyRound, Pencil, Plus, Trash2, X } from "lucide-react";
import { Page, PageHeader } from "../components/PageHeader";
import { Checkbox, EmptyState, ErrorText, Field, Modal, Spinner } from "../components/ui";
import { useConfirm, useToast } from "../components/feedback";
import { Select } from "../components/Select";
import { api, type EnvSecret, type Environment, type ExternalSecretRef, type SecretBackend } from "../lib/api";
import { envVarsToText, extraVarsToText, parseEnvVars, parseExtraVars } from "../lib/extravars";
import { fmtRelative } from "../lib/format";
import { usePrefs } from "../lib/prefs";
import { useAuth } from "../lib/auth";

export function Environments() {
  const { t, lang } = usePrefs();
  const { user } = useAuth();
  const envs = useQuery({ queryKey: ["environments"], queryFn: api.listEnvironments });
  const [editing, setEditing] = useState<Environment | null>(null);
  const [creating, setCreating] = useState(false);
  const [backendsOpen, setBackendsOpen] = useState(false);

  return (
    <>
      <PageHeader
        title={t("env.title")}
        subtitle={t("env.subtitle")}
        actions={
          <>
            {user?.role === "admin" && (
              <button className="btn-outline" onClick={() => setBackendsOpen(true)}>
                <Cloud size={15} /> {t("env.backends")}
              </button>
            )}
            <button className="btn-primary" onClick={() => setCreating(true)}>
              <Plus size={15} /> {t("env.new")}
            </button>
          </>
        }
      />
      <Page>
        {envs.isLoading ? (
          <div className="flex justify-center py-20 text-ink-faint"><Spinner /></div>
        ) : envs.data?.length ? (
          <div className="card divide-y divide-border">
            {envs.data.map((e) => (
              <div key={e.id} className="flex items-center gap-3 px-4 py-3">
                <Boxes size={18} className="text-accent" />
                <div className="min-w-0 flex-1">
                  <div className="truncate text-sm text-ink">{e.name}</div>
                  <div className="truncate text-xs text-ink-faint">
                    {e.description || t("env.noDescription")} · {Object.keys(e.extraVars || {}).length} {t("env.varsN")} ·{" "}
                    {Object.keys(e.envVars || {}).length} {t("env.envN")} ·{" "}
                    <span className={e.secrets?.length ? "text-warn" : ""}>{e.secrets?.length ?? 0} {t("env.secretsN")}</span> ·
                    {" "}{t("env.updated")} {fmtRelative(e.updatedAt, lang)}
                  </div>
                </div>
                <button className="btn-ghost p-1.5" onClick={() => setEditing(e)} title={t("common.edit")}>
                  <Pencil size={15} />
                </button>
                <Delete id={e.id} name={e.name} />
              </div>
            ))}
          </div>
        ) : (
          <EmptyState
            icon={<Boxes size={28} />}
            title={t("env.none")}
            description={t("env.emptyDesc")}
            action={
              <button className="btn-primary" onClick={() => setCreating(true)}>
                <Plus size={15} /> {t("env.new")}
              </button>
            }
          />
        )}
      </Page>
      {(creating || editing) && (
        <EnvForm environment={editing} onClose={() => { setCreating(false); setEditing(null); }} />
      )}
      {backendsOpen && <SecretBackendsModal onClose={() => setBackendsOpen(false)} />}
    </>
  );
}

function backendSubtitle(b: SecretBackend): string {
  switch (b.type) {
    case "aws":
      return `${b.region || "?"} · ${b.accessKeyId || "?"}`;
    case "azure":
      return b.address || "?";
    case "gcp":
      return `project ${b.accessKeyId || "?"}`;
    case "cyberark":
      return `${b.address || "?"} · ${b.accessKeyId || "?"}`;
    default:
      return `${b.address} · ${b.mount || "secret"}`;
  }
}

function backendValid(b: Partial<SecretBackend>): boolean {
  if (!b.name) return false;
  switch (b.type) {
    case "aws":
      return !!b.region && !!b.accessKeyId;
    case "azure":
      return !!b.address && !!b.tenantId && !!b.accessKeyId;
    case "gcp":
      return !!b.accessKeyId;
    case "cyberark":
      return !!b.address && !!b.accessKeyId;
    default:
      return !!b.address;
  }
}

function SecretBackendsModal({ onClose }: { onClose: () => void }) {
  const { t } = usePrefs();
  const qc = useQueryClient();
  const confirm = useConfirm();
  const toast = useToast();
  const backends = useQuery({ queryKey: ["secret-backends"], queryFn: api.listSecretBackends });
  const blank: Partial<SecretBackend> = { name: "", type: "vault", address: "", mount: "secret", namespace: "", insecure: false, region: "", tenantId: "", accessKeyId: "", token: "" };
  const [form, setForm] = useState<Partial<SecretBackend>>(blank);
  const [test, setTest] = useState<{ ok: boolean; message: string } | null>(null);
  const editing = !!form.id;
  const set = (patch: Partial<SecretBackend>) => { setForm((f) => ({ ...f, ...patch })); setTest(null); };
  const refresh = () => qc.invalidateQueries({ queryKey: ["secret-backends"] });

  const save = useMutation({
    mutationFn: () => editing ? api.updateSecretBackend(form.id!, form) : api.createSecretBackend(form),
    onSuccess: () => { refresh(); setForm(blank); toast.success(t("common.saved")); },
    onError: (e) => toast.error((e as Error).message),
  });
  const del = useMutation({
    mutationFn: (id: string) => api.deleteSecretBackend(id),
    onSuccess: refresh,
    onError: (e) => toast.error((e as Error).message),
  });
  const runTest = useMutation({
    mutationFn: () => api.testSecretBackend(form),
    onSuccess: (r) => setTest(r),
    onError: (e) => setTest({ ok: false, message: (e as Error).message }),
  });

  return (
    <Modal open onClose={onClose} wide title={t("env.backendsTitle")} footer={<button className="btn-outline" onClick={onClose}>{t("common.close")}</button>}>
      <div className="space-y-4">
        <p className="text-sm text-ink-dim">{t("env.backendsHint")}</p>

        {backends.data?.length ? (
          <div className="card divide-y divide-border">
            {backends.data.map((b) => (
              <div key={b.id} className="flex items-center gap-3 px-3.5 py-2.5">
                <Cloud size={16} className="text-accent" />
                <div className="min-w-0 flex-1">
                  <div className="truncate text-sm text-ink">{b.name} <span className="chip ml-1 text-[10px] uppercase">{b.type}</span></div>
                  <div className="truncate font-mono text-xs text-ink-faint">{backendSubtitle(b)}{b.hasToken ? " · 🔑" : ""}</div>
                </div>
                <button className="btn-ghost p-1.5" title={t("common.edit")} onClick={() => { setForm({ ...b, token: "" }); setTest(null); }}>
                  <Pencil size={14} />
                </button>
                <button className="btn-ghost p-1.5" title={t("common.delete")} onClick={() => confirm({ title: t("common.delete"), body: b.name }).then((ok) => ok && del.mutate(b.id))}>
                  <Trash2 size={14} />
                </button>
              </div>
            ))}
          </div>
        ) : (
          <p className="text-xs text-ink-faint">{t("env.backendsEmpty")}</p>
        )}

        <div className="card space-y-3 p-3.5">
          <div className="text-sm font-medium text-ink">{editing ? t("env.backendEdit") : t("env.backendNew")}</div>
          <div className="grid gap-3 sm:grid-cols-2">
            <Field label={t("common.name")}>
              <input className="input" value={form.name ?? ""} onChange={(e) => set({ name: e.target.value })} placeholder={form.type === "aws" ? "aws-prod" : "vault-prod"} />
            </Field>
            <Field label={t("env.backendType")}>
              <Select
                value={form.type ?? "vault"}
                onChange={(v) => set({ type: v as SecretBackend["type"] })}
                options={[
                  { label: t("env.backendVault"), value: "vault" },
                  { label: t("env.backendAws"), value: "aws" },
                  { label: t("env.backendAzure"), value: "azure" },
                  { label: t("env.backendGcp"), value: "gcp" },
                  { label: "CyberArk Conjur", value: "cyberark" },
                ]}
              />
            </Field>
            {form.type === "aws" && (
              <>
                <Field label={t("env.backendRegion")}>
                  <input className="input font-mono text-xs" value={form.region ?? ""} onChange={(e) => set({ region: e.target.value })} placeholder="eu-central-1" />
                </Field>
                <Field label={t("env.backendAccessKey")}>
                  <input className="input font-mono text-xs" value={form.accessKeyId ?? ""} onChange={(e) => set({ accessKeyId: e.target.value })} placeholder="AKIA…" />
                </Field>
                <Field label={t("env.backendSecretKey")} hint={editing ? t("env.backendTokenKeep") : t("env.backendSecretKeyHint")}>
                  <input className="input font-mono text-xs" type="password" value={form.token ?? ""} onChange={(e) => set({ token: e.target.value })} placeholder={editing ? "••••••••" : "wJalrXUtn…"} />
                </Field>
                <Field label={t("env.backendEndpoint")} hint={t("env.backendEndpointHint")}>
                  <input className="input font-mono text-xs" value={form.address ?? ""} onChange={(e) => set({ address: e.target.value })} placeholder="(default)" />
                </Field>
              </>
            )}
            {form.type === "azure" && (
              <>
                <Field label={t("env.backendVaultUrl")}>
                  <input className="input font-mono text-xs" value={form.address ?? ""} onChange={(e) => set({ address: e.target.value })} placeholder="https://myvault.vault.azure.net" />
                </Field>
                <Field label={t("env.backendTenant")}>
                  <input className="input font-mono text-xs" value={form.tenantId ?? ""} onChange={(e) => set({ tenantId: e.target.value })} placeholder="00000000-0000-0000-0000-000000000000" />
                </Field>
                <Field label={t("env.backendClientId")}>
                  <input className="input font-mono text-xs" value={form.accessKeyId ?? ""} onChange={(e) => set({ accessKeyId: e.target.value })} placeholder="app (client) id" />
                </Field>
                <Field label={t("env.backendClientSecret")} hint={editing ? t("env.backendTokenKeep") : t("env.backendClientSecretHint")}>
                  <input className="input font-mono text-xs" type="password" value={form.token ?? ""} onChange={(e) => set({ token: e.target.value })} placeholder={editing ? "••••••••" : "client secret"} />
                </Field>
              </>
            )}
            {form.type === "gcp" && (
              <>
                <Field label={t("env.backendProject")}>
                  <input className="input font-mono text-xs" value={form.accessKeyId ?? ""} onChange={(e) => set({ accessKeyId: e.target.value })} placeholder="my-gcp-project" />
                </Field>
                <div className="sm:col-span-2">
                  <Field label={t("env.backendSaJson")} hint={editing ? t("env.backendTokenKeep") : t("env.backendSaJsonHint")}>
                    <textarea
                      className="input font-mono text-xs"
                      rows={4}
                      value={form.token ?? ""}
                      onChange={(e) => set({ token: e.target.value })}
                      placeholder={editing ? "•••••••• (paste to replace)" : '{ "type": "service_account", "project_id": …, "private_key": … }'}
                    />
                  </Field>
                </div>
              </>
            )}
            {form.type === "cyberark" && (
              <>
                <Field label={t("env.backendConjurUrl")}>
                  <input className="input font-mono text-xs" value={form.address ?? ""} onChange={(e) => set({ address: e.target.value })} placeholder="https://conjur.example.com" />
                </Field>
                <Field label={t("env.backendConjurAccount")}>
                  <input className="input font-mono text-xs" value={form.namespace ?? ""} onChange={(e) => set({ namespace: e.target.value })} placeholder="default" />
                </Field>
                <Field label={t("env.backendConjurLogin")}>
                  <input className="input font-mono text-xs" value={form.accessKeyId ?? ""} onChange={(e) => set({ accessKeyId: e.target.value })} placeholder="host/myapp" />
                </Field>
                <Field label={t("env.backendConjurApiKey")} hint={editing ? t("env.backendTokenKeep") : undefined}>
                  <input className="input font-mono text-xs" type="password" value={form.token ?? ""} onChange={(e) => set({ token: e.target.value })} placeholder={editing ? "••••••••" : "API key"} />
                </Field>
              </>
            )}
            {(!form.type || form.type === "vault") && (
              <>
                <Field label={t("env.backendAddress")}>
                  <input className="input font-mono text-xs" value={form.address ?? ""} onChange={(e) => set({ address: e.target.value })} placeholder="https://vault.internal:8200" />
                </Field>
                <Field label={t("env.backendMount")} hint={t("env.backendMountHint")}>
                  <input className="input font-mono text-xs" value={form.mount ?? ""} onChange={(e) => set({ mount: e.target.value })} placeholder="secret" />
                </Field>
                <Field label={t("env.backendNamespace")} hint={t("env.backendNamespaceHint")}>
                  <input className="input font-mono text-xs" value={form.namespace ?? ""} onChange={(e) => set({ namespace: e.target.value })} placeholder="admin/team" />
                </Field>
                <Field label={t("env.backendToken")} hint={editing ? t("env.backendTokenKeep") : t("env.backendTokenHint")}>
                  <input className="input font-mono text-xs" type="password" value={form.token ?? ""} onChange={(e) => set({ token: e.target.value })} placeholder={editing ? "••••••••" : "hvs.…"} />
                </Field>
                <label className="flex items-center gap-2 self-end pb-2 text-sm text-ink-dim">
                  <Checkbox checked={!!form.insecure} onChange={(v) => set({ insecure: v })} />
                  {t("env.backendInsecure")}
                </label>
              </>
            )}
          </div>
          {test && <div className={test.ok ? "text-xs text-accent" : "text-xs text-danger"}>{test.ok ? "✓ " : "✗ "}{test.message}</div>}
          <div className="flex items-center gap-2">
            <button className="btn-primary" disabled={!backendValid(form) || save.isPending} onClick={() => save.mutate()}>
              {save.isPending ? <Spinner /> : null} {editing ? t("common.save") : t("common.add")}
            </button>
            <button className="btn-outline" disabled={!backendValid(form) || runTest.isPending} onClick={() => runTest.mutate()}>
              {runTest.isPending ? <Spinner /> : null} {t("env.backendTest")}
            </button>
            {editing && <button className="btn-ghost" onClick={() => { setForm(blank); setTest(null); }}>{t("common.cancel")}</button>}
          </div>
        </div>
      </div>
    </Modal>
  );
}

function Delete({ id, name }: { id: string; name: string }) {
  const { t } = usePrefs();
  const confirm = useConfirm();
  const qc = useQueryClient();
  const del = useMutation({
    mutationFn: () => api.deleteEnvironment(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["environments"] }),
  });
  return (
    <button className="btn-ghost p-1.5" title={t("common.delete")} onClick={() => confirm({ title: t("env.confirmDelete"), body: name }).then((ok) => ok && del.mutate())}>
      <Trash2 size={15} />
    </button>
  );
}

function SecretsEditor({
  value,
  onChange,
  editing,
}: {
  value: EnvSecret[];
  onChange: (v: EnvSecret[]) => void;
  editing: boolean;
}) {
  const { t } = usePrefs();
  const update = (i: number, patch: Partial<EnvSecret>) =>
    onChange(value.map((s, idx) => (idx === i ? { ...s, ...patch } : s)));
  const remove = (i: number) => onChange(value.filter((_, idx) => idx !== i));
  const add = () => onChange([...value, { name: "", type: "env", value: "" }]);

  return (
    <div>
      <div className="mb-1.5 flex items-center justify-between">
        <span className="label mb-0 flex items-center gap-1.5">
          <KeyRound size={13} /> {t("env.secrets")}
        </span>
        <button type="button" className="btn-ghost px-2 py-1 text-xs" onClick={add}>
          <Plus size={13} /> {t("env.addSecret")}
        </button>
      </div>
      {value.length === 0 && (
        <p className="text-xs text-ink-faint">
          {t("env.secretsHelp")}
        </p>
      )}
      <div className="space-y-2">
        {value.map((s, i) => (
          <div key={i} className="grid grid-cols-[1fr_110px_1fr_auto] items-center gap-2">
            <input
              className="input font-mono text-xs"
              placeholder="SECRET_NAME"
              value={s.name}
              onChange={(e) => update(i, { name: e.target.value })}
            />
            <Select
              value={s.type}
              onChange={(val) => update(i, { type: val as EnvSecret["type"] })}
              options={[{ label: t("env.typeEnv"), value: "env" }, { label: t("env.typeVar"), value: "var" }]}
            />
            <input
              className="input font-mono text-xs"
              type="password"
              placeholder={editing && s.hasValue ? t("env.secretUnchanged") : t("env.valuePh")}
              value={s.value ?? ""}
              onChange={(e) => update(i, { value: e.target.value })}
            />
            <button type="button" className="btn-ghost p-1.5" onClick={() => remove(i)} title={t("common.remove")}>
              <X size={14} />
            </button>
          </div>
        ))}
      </div>
    </div>
  );
}

function ExternalSecretsEditor({
  value,
  onChange,
}: {
  value: ExternalSecretRef[];
  onChange: (v: ExternalSecretRef[]) => void;
}) {
  const { t } = usePrefs();
  const backends = useQuery({ queryKey: ["secret-backends"], queryFn: api.listSecretBackends });
  const update = (i: number, patch: Partial<ExternalSecretRef>) =>
    onChange(value.map((s, idx) => (idx === i ? { ...s, ...patch } : s)));
  const remove = (i: number) => onChange(value.filter((_, idx) => idx !== i));
  const add = () => onChange([...value, { name: "", backend: backends.data?.[0]?.id ?? "", path: "", field: "", asVar: false }]);
  const opts = (backends.data ?? []).map((b) => ({ label: b.name, value: b.id }));

  return (
    <div>
      <div className="mb-1.5 flex items-center justify-between">
        <span className="label mb-0 flex items-center gap-1.5">
          <Cloud size={13} /> {t("env.external")}
        </span>
        <button type="button" className="btn-ghost px-2 py-1 text-xs" onClick={add} disabled={!opts.length}>
          <Plus size={13} /> {t("env.addExternal")}
        </button>
      </div>
      <p className="mb-2 text-xs text-ink-faint">
        {opts.length ? t("env.externalHelp") : t("env.externalNone")}
      </p>
      <div className="space-y-2">
        {value.map((s, i) => (
          <div key={i} className="grid grid-cols-[1fr_1fr] items-center gap-2 rounded-lg border border-border p-2 sm:grid-cols-[1fr_1fr_1fr_90px_100px_auto]">
            <input className="input font-mono text-xs" placeholder="VAR_NAME" value={s.name} onChange={(e) => update(i, { name: e.target.value })} />
            <Select value={s.backend} onChange={(v) => update(i, { backend: v })} placeholder={t("env.backend")} options={opts} />
            <input className="input font-mono text-xs" placeholder="app/db" value={s.path} onChange={(e) => update(i, { path: e.target.value })} />
            <input className="input font-mono text-xs" placeholder="field" value={s.field ?? ""} onChange={(e) => update(i, { field: e.target.value })} />
            <Select
              value={s.asVar ? "var" : "env"}
              onChange={(v) => update(i, { asVar: v === "var" })}
              options={[{ label: t("env.typeEnv"), value: "env" }, { label: t("env.typeVar"), value: "var" }]}
            />
            <button type="button" className="btn-ghost p-1.5" onClick={() => remove(i)} title={t("common.remove")}>
              <X size={14} />
            </button>
          </div>
        ))}
      </div>
    </div>
  );
}

function EnvForm({ environment, onClose }: { environment: Environment | null; onClose: () => void }) {
  const { t } = usePrefs();
  const qc = useQueryClient();
  const editing = !!environment;
  const [name, setName] = useState(environment?.name ?? "");
  const [description, setDescription] = useState(environment?.description ?? "");
  const [extraVars, setExtraVars] = useState(extraVarsToText(environment?.extraVars));
  const [envVars, setEnvVars] = useState(envVarsToText(environment?.envVars));
  const [secrets, setSecrets] = useState<EnvSecret[]>(environment?.secrets ?? []);
  const [externalSecrets, setExternalSecrets] = useState<ExternalSecretRef[]>(environment?.externalSecrets ?? []);
  const [error, setError] = useState("");

  const save = useMutation({
    mutationFn: () => {
      const ex = parseExtraVars(extraVars);
      if (ex.error) throw new Error(ex.error);
      const en = parseEnvVars(envVars);
      if (en.error) throw new Error(en.error);
      const cleanedSecrets = secrets.filter((s) => s.name.trim());
      const cleanedExternal = externalSecrets.filter((s) => s.name.trim() && s.backend && s.path.trim());
      const body: Partial<Environment> = {
        name,
        description,
        extraVars: ex.vars,
        envVars: en.vars,
        secrets: cleanedSecrets,
        externalSecrets: cleanedExternal,
      };
      return editing ? api.updateEnvironment(environment!.id, body) : api.createEnvironment(body);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["environments"] });
      onClose();
    },
    onError: (e) => setError((e as Error).message),
  });

  return (
    <Modal
      open
      onClose={onClose}
      wide
      title={editing ? t("env.edit") : t("env.new")}
      footer={
        <>
          <button className="btn-outline" onClick={onClose}>{t("common.cancel")}</button>
          <button className="btn-primary" onClick={() => { setError(""); save.mutate(); }} disabled={!name || save.isPending}>
            {save.isPending ? <Spinner /> : null} {t("common.save")}
          </button>
        </>
      }
    >
      <div className="space-y-4">
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label={t("common.name")}>
            <input className="input" value={name} onChange={(e) => setName(e.target.value)} placeholder="production" autoFocus />
          </Field>
          <Field label={t("common.description")}>
            <input className="input" value={description} onChange={(e) => setDescription(e.target.value)} placeholder={t("common.optional")} />
          </Field>
        </div>
        <Field label={t("env.extraVars")} hint={t("env.extraVarsHint")}>
          <textarea className="input min-h-[90px] font-mono text-xs" value={extraVars} onChange={(e) => setExtraVars(e.target.value)} placeholder={'{"region": "eu-west-1"}'} />
        </Field>
        <Field label={t("env.envVars")} hint={t("env.envVarsHint")}>
          <textarea className="input min-h-[80px] font-mono text-xs" value={envVars} onChange={(e) => setEnvVars(e.target.value)} placeholder={"AWS_REGION=eu-west-1\nANSIBLE_TIMEOUT=60"} />
        </Field>
        <SecretsEditor value={secrets} onChange={setSecrets} editing={editing} />
        <ExternalSecretsEditor value={externalSecrets} onChange={setExternalSecrets} />
        {error && <ErrorText>{error}</ErrorText>}
      </div>
    </Modal>
  );
}
