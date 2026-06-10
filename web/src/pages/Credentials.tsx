import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { KeyRound, Lock, Plus, Trash2, UserCircle } from "lucide-react";
import { Page, PageHeader } from "../components/PageHeader";
import { Checkbox, EmptyState, ErrorText, Field, Modal, Spinner } from "../components/ui";
import { useConfirm, useToast } from "../components/feedback";
import { Select } from "../components/Select";
import { api, type Credential } from "../lib/api";
import { fmtRelative } from "../lib/format";
import { usePrefs } from "../lib/prefs";

const typeMeta: Record<string, { labelKey: string; icon: typeof KeyRound }> = {
  ssh: { labelKey: "cred.typeSsh", icon: KeyRound },
  login_password: { labelKey: "cred.typeLogin", icon: UserCircle },
  vault: { labelKey: "cred.typeVault", icon: Lock },
  gcp: { labelKey: "cred.typeGcp", icon: Lock },
  azure: { labelKey: "cred.typeAzure", icon: Lock },
};

export function Credentials() {
  const { t, lang } = usePrefs();
  const creds = useQuery({ queryKey: ["credentials"], queryFn: api.listCredentials });
  const [editing, setEditing] = useState<Credential | null>(null);
  const [creating, setCreating] = useState(false);

  return (
    <>
      <PageHeader
        title={t("cred.title")}
        subtitle={t("cred.subtitle")}
        actions={
          <button className="btn-primary" onClick={() => setCreating(true)}>
            <Plus size={15} /> {t("cred.new")}
          </button>
        }
      />
      <Page>
        {creds.isLoading ? (
          <div className="flex justify-center py-20 text-ink-faint">
            <Spinner />
          </div>
        ) : creds.data?.length ? (
          <div className="card divide-y divide-border">
            {creds.data.map((c) => {
              const m = typeMeta[c.type] ?? typeMeta.ssh;
              return (
                <div key={c.id} className="flex items-center gap-3 px-4 py-3">
                  <m.icon size={18} className="text-accent" />
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <span className="truncate text-sm text-ink">{c.name}</span>
                      {c.ownerUserId && <span className="chip text-[10px] uppercase text-accent">{t("cred.personalBadge")}</span>}
                    </div>
                    <div className="truncate text-xs text-ink-faint">
                      {t(m.labelKey)}
                      {c.login && ` · ${c.login}`} · {t("cred.updated")} {fmtRelative(c.updatedAt, lang)}
                    </div>
                  </div>
                  <span className="chip">{c.hasSecret ? t("cred.secretSet") : t("cred.noSecret")}</span>
                  {c.type === "ssh" && c.sshCertificate && <span className="chip" title={t("cred.sshCertHint")}>{t("cred.sshCertBadge")}</span>}
                  {c.type === "ssh" && c.hasSecret && <PubKeyButton id={c.id} />}
                  <button className="btn-ghost p-1.5" onClick={() => setEditing(c)} title={t("common.edit")}>
                    <Lock size={15} />
                  </button>
                  <DeleteButton id={c.id} name={c.name} />
                </div>
              );
            })}
          </div>
        ) : (
          <EmptyState
            icon={<KeyRound size={28} />}
            title={t("cred.none")}
            description={t("cred.emptyDesc")}
            action={
              <button className="btn-primary" onClick={() => setCreating(true)}>
                <Plus size={15} /> {t("cred.new")}
              </button>
            }
          />
        )}
      </Page>
      {(creating || editing) && (
        <CredentialForm credential={editing} onClose={() => { setCreating(false); setEditing(null); }} />
      )}
    </>
  );
}

function PubKeyButton({ id }: { id: string }) {
  const { t } = usePrefs();
  const toast = useToast();
  const [open, setOpen] = useState(false);
  const pk = useQuery({ queryKey: ["pubkey", id], queryFn: () => api.credentialPubKey(id), enabled: open });
  return (
    <>
      <button className="btn-ghost p-1.5" title={t("cred.showPubKey")} onClick={() => setOpen(true)}>
        <KeyRound size={15} />
      </button>
      {open && (
        <Modal
          open
          onClose={() => setOpen(false)}
          title={t("cred.pubKeyTitle")}
          subtitle={t("cred.pubKeyHint")}
          footer={<button className="btn-outline" onClick={() => setOpen(false)}>{t("common.close")}</button>}
        >
          {pk.isLoading ? (
            <Spinner />
          ) : pk.error ? (
            <ErrorText>{(pk.error as Error).message}</ErrorText>
          ) : (
            <div className="space-y-2">
              <textarea readOnly className="input min-h-[96px] w-full break-all font-mono text-xs" value={pk.data?.publicKey ?? ""} />
              <button
                className="btn-outline"
                onClick={() => { void navigator.clipboard.writeText(pk.data?.publicKey ?? ""); toast.success(t("common.copied")); }}
              >
                {t("common.copy")}
              </button>
            </div>
          )}
        </Modal>
      )}
    </>
  );
}

function DeleteButton({ id, name }: { id: string; name: string }) {
  const { t } = usePrefs();
  const confirm = useConfirm();
  const qc = useQueryClient();
  const del = useMutation({
    mutationFn: () => api.deleteCredential(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["credentials"] }),
  });
  return (
    <button
      className="btn-ghost p-1.5"
      title={t("common.delete")}
      onClick={() => confirm({ title: t("cred.confirmDelete"), body: name }).then((ok) => ok && del.mutate())}
    >
      <Trash2 size={15} />
    </button>
  );
}

function CredentialForm({ credential, onClose }: { credential: Credential | null; onClose: () => void }) {
  const { t } = usePrefs();
  const qc = useQueryClient();
  const editing = !!credential;
  const [name, setName] = useState(credential?.name ?? "");
  const [type, setType] = useState<Credential["type"]>(credential?.type ?? "ssh");
  const [login, setLogin] = useState(credential?.login ?? "");
  const [sshPrivateKey, setSshKey] = useState("");
  const [sshCertificate, setSshCert] = useState(credential?.sshCertificate ?? "");
  const [passphrase, setPassphrase] = useState("");
  const [password, setPassword] = useState("");
  const [vaultPassword, setVaultPassword] = useState("");
  const [serviceAccount, setServiceAccount] = useState("");
  // Azure service principal (assembled into serviceAccount as JSON on save).
  const [azSub, setAzSub] = useState("");
  const [azTenant, setAzTenant] = useState("");
  const [azClient, setAzClient] = useState("");
  const [azSecret, setAzSecret] = useState("");
  const [personal, setPersonal] = useState(false);
  const [error, setError] = useState("");

  const save = useMutation({
    mutationFn: () => {
      const body: Record<string, unknown> = { name, type, login };
      if (!editing && personal) body.personal = true;
      if (sshPrivateKey) body.sshPrivateKey = sshPrivateKey;
      if (type === "ssh") body.sshCertificate = sshCertificate; // public cert; metadata, re-sent so edits preserve it
      if (passphrase) body.passphrase = passphrase;
      if (password) body.password = password;
      if (vaultPassword) body.vaultPassword = vaultPassword;
      if (serviceAccount) body.serviceAccount = serviceAccount;
      if (type === "azure" && (azSub || azClient || azSecret || azTenant)) {
        body.serviceAccount = JSON.stringify({ subscriptionId: azSub, tenantId: azTenant, clientId: azClient, secret: azSecret });
      }
      return editing ? api.updateCredential(credential!.id, body) : api.createCredential(body);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["credentials"] });
      onClose();
    },
    onError: (e) => setError((e as Error).message),
  });

  return (
    <Modal
      open
      onClose={onClose}
      title={editing ? t("cred.edit") : t("cred.new")}
      subtitle={editing ? t("cred.editSubtitle") : t("cred.newSubtitle")}
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
        <Field label={t("common.name")}>
          <input className="input" value={name} onChange={(e) => setName(e.target.value)} placeholder="prod-ssh-key" autoFocus />
        </Field>
        <Field label={t("common.type")}>
          <Select
            value={type}
            onChange={(v) => setType(v as Credential["type"])}
            options={[
              { label: t("cred.optSsh"), value: "ssh" },
              { label: t("cred.typeLogin"), value: "login_password" },
              { label: t("cred.typeVault"), value: "vault" },
              { label: t("cred.typeGcp"), value: "gcp" },
              { label: t("cred.typeAzure"), value: "azure" },
            ]}
          />
        </Field>
        {!editing && (
          <Checkbox checked={personal} onChange={setPersonal} label={t("cred.personal")} />
        )}

        {type === "ssh" && (
          <>
            <Field label={t("cred.optSsh")} hint={editing ? t("cred.blankUnchanged") : t("cred.sshFormat")}>
              <textarea
                className="input min-h-[120px] font-mono text-xs"
                spellCheck={false}
                value={sshPrivateKey}
                onChange={(e) => setSshKey(e.target.value)}
                placeholder="-----BEGIN OPENSSH PRIVATE KEY-----"
              />
            </Field>
            <Field label={t("cred.passphrase")} hint={t("common.optional")}>
              <input className="input" type="password" value={passphrase} onChange={(e) => setPassphrase(e.target.value)} />
            </Field>
            <Field label={t("cred.sshCert")} hint={t("cred.sshCertHint")}>
              <textarea
                className="input min-h-[72px] font-mono text-xs"
                spellCheck={false}
                value={sshCertificate}
                onChange={(e) => setSshCert(e.target.value)}
                placeholder="ssh-rsa-cert-v01@openssh.com AAAA…"
              />
            </Field>
          </>
        )}
        {type === "login_password" && (
          <>
            <Field label={t("common.username")}>
              <input className="input" value={login} onChange={(e) => setLogin(e.target.value)} />
            </Field>
            <Field label={t("common.password")} hint={editing ? t("cred.blankUnchanged") : undefined}>
              <input className="input" type="password" value={password} onChange={(e) => setPassword(e.target.value)} />
            </Field>
          </>
        )}
        {type === "vault" && (
          <Field label={t("cred.typeVault")} hint={editing ? t("cred.blankUnchanged") : t("cred.vaultHint")}>
            <input className="input" type="password" value={vaultPassword} onChange={(e) => setVaultPassword(e.target.value)} />
          </Field>
        )}
        {type === "gcp" && (
          <Field label={t("cred.gcpSaJson")} hint={editing ? t("cred.blankUnchanged") : t("cred.gcpSaJsonHint")}>
            <textarea
              className="input min-h-[120px] font-mono text-xs"
              spellCheck={false}
              value={serviceAccount}
              onChange={(e) => setServiceAccount(e.target.value)}
              placeholder='{ "type": "service_account", "project_id": …, "private_key": … }'
            />
          </Field>
        )}
        {type === "azure" && (
          <>
            {editing && <p className="text-xs text-ink-faint">{t("cred.azureKeep")}</p>}
            <div className="grid gap-3 sm:grid-cols-2">
              <Field label={t("cred.azureSub")}>
                <input className="input font-mono text-xs" value={azSub} onChange={(e) => setAzSub(e.target.value)} placeholder="00000000-0000-0000-0000-000000000000" />
              </Field>
              <Field label={t("cred.azureTenant")}>
                <input className="input font-mono text-xs" value={azTenant} onChange={(e) => setAzTenant(e.target.value)} placeholder="tenant id" />
              </Field>
              <Field label={t("cred.azureClient")}>
                <input className="input font-mono text-xs" value={azClient} onChange={(e) => setAzClient(e.target.value)} placeholder="app (client) id" />
              </Field>
              <Field label={t("cred.azureSecret")}>
                <input className="input font-mono text-xs" type="password" value={azSecret} onChange={(e) => setAzSecret(e.target.value)} placeholder="client secret" />
              </Field>
            </div>
          </>
        )}
        {error && <ErrorText>{error}</ErrorText>}
      </div>
    </Modal>
  );
}
