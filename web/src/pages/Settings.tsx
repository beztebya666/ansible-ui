import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Bell, CircleCheck, CircleX, Copy, Database, Download, Info, KeyRound, Package, Plus, Send, Shield, ShieldOff, Timer, Trash2, User as UserIcon } from "lucide-react";
import { Page, PageHeader } from "../components/PageHeader";
import { Checkbox, EmptyState, ErrorText, Field, Modal, Spinner, Toggle } from "../components/ui";
import { useConfirm, useToast } from "../components/feedback";
import { Select } from "../components/Select";
import { StatusBadge } from "../components/StatusBadge";
import { api, type NotifyType } from "../lib/api";
import { useAuth } from "../lib/auth";
import { usePrefs } from "../lib/prefs";
import { fmtRelative } from "../lib/format";

export function Settings() {
  const { t, lang } = usePrefs();
  const { user } = useAuth();
  const isAdmin = user?.role === "admin";
  const users = useQuery({ queryKey: ["users"], queryFn: api.listUsers, enabled: isAdmin });
  const [creating, setCreating] = useState(false);

  return (
    <>
      <PageHeader title={t("set.title")} subtitle={t("set.subtitle")} />
      <Page>
        <div className="card mb-4 p-4">
          <h2 className="mb-3 text-sm font-semibold text-ink">{t("set.account")}</h2>
          <div className="flex items-center gap-3">
            <div className="grid h-10 w-10 place-items-center rounded-full bg-accent/15 text-accent">
              <UserIcon size={18} />
            </div>
            <div>
              <div className="text-sm text-ink">{user?.username}</div>
              <div className="text-xs text-ink-faint">
                {user?.email || t("common.noEmail")} · <span className="capitalize">{user?.role}</span>
              </div>
            </div>
          </div>
        </div>

        <Preferences />

        <TwoFactorCard />

        {isAdmin && <DocsToggleCard />}

        {isAdmin && <NotificationsCard />}

        {isAdmin && <NotificationLogCard />}

        {isAdmin && <SecretAccessLogCard />}

        {isAdmin && <RetentionCard />}

        {isAdmin && <FlagsCard />}

        {isAdmin && <TaskSettingsCard />}

        {isAdmin && <SyslogCard />}

        {isAdmin && <SMTPCard />}

        {isAdmin && <GalaxyCard />}

        {isAdmin && <NotifyProxyCard />}

        {isAdmin && <AuthMappingCard />}

        {isAdmin && <CustomRolesCard />}

        {isAdmin && <SystemInfoCard />}

        <ApiTokens />

        {isAdmin && (
          <div className="card">
            <div className="flex items-center justify-between border-b border-border px-4 py-3">
              <h2 className="text-sm font-semibold text-ink">{t("set.users")}</h2>
              <button className="btn-outline" onClick={() => setCreating(true)}>
                <Plus size={15} /> {t("set.addUser")}
              </button>
            </div>
            {users.isLoading ? (
              <div className="flex justify-center py-10 text-ink-faint"><Spinner /></div>
            ) : users.data?.length ? (
              <div className="divide-y divide-border">
                {users.data.map((u) => (
                  <div key={u.id} className="flex items-center gap-3 px-4 py-3">
                    {u.role === "admin" ? (
                      <Shield size={17} className="text-accent" />
                    ) : (
                      <UserIcon size={17} className="text-ink-faint" />
                    )}
                    <div className="min-w-0 flex-1">
                      <div className="truncate text-sm text-ink">{u.username}</div>
                      <div className="truncate text-xs text-ink-faint">
                        {u.email || t("common.noEmail")} · {fmtRelative(u.createdAt, lang)}
                      </div>
                    </div>
                    {u.twoFactorEnabled && <span className="chip text-success">2FA</span>}
                    <span className="chip capitalize">{u.role}</span>
                    {u.twoFactorEnabled && u.id !== user?.id && <Reset2FA id={u.id} name={u.username} />}
                    {u.id !== user?.id && <DeleteUser id={u.id} name={u.username} />}
                  </div>
                ))}
              </div>
            ) : (
              <div className="p-6">
                <EmptyState icon={<UserIcon size={24} />} title={t("set.noUsers")} />
              </div>
            )}
          </div>
        )}
      </Page>
      {creating && <CreateUser onClose={() => setCreating(false)} />}
    </>
  );
}

function Preferences() {
  const { t, theme, setTheme, lang, setLang, tz, setTz, clock, setClock } = usePrefs();
  const detected = (() => {
    try {
      return Intl.DateTimeFormat().resolvedOptions().timeZone;
    } catch {
      return "UTC";
    }
  })();
  const zones = (() => {
    try {
      const all = (Intl as unknown as { supportedValuesOf?: (k: string) => string[] }).supportedValuesOf?.("timeZone");
      if (all && all.length) return all;
    } catch {
      /* older browsers */
    }
    return [
      "UTC",
      "Europe/Moscow",
      "Europe/London",
      "Europe/Berlin",
      "America/New_York",
      "America/Los_Angeles",
      "Asia/Tokyo",
      "Asia/Almaty",
    ];
  })();

  return (
    <div className="card mb-4 p-4">
      <h2 className="mb-3 text-sm font-semibold text-ink">{t("settings.preferences")}</h2>
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <Field label={t("settings.theme")}>
          <Select
            value={theme}
            onChange={(v) => setTheme(v as "dark" | "light")}
            options={[{ label: t("theme.dark"), value: "dark" }, { label: t("theme.light"), value: "light" }]}
          />
        </Field>
        <Field label={t("settings.language")}>
          <Select
            value={lang}
            onChange={(v) => setLang(v as "en" | "ru")}
            options={[{ label: "English", value: "en" }, { label: "Русский", value: "ru" }]}
          />
        </Field>
        <Field label={t("settings.clock")}>
          <Select
            value={clock}
            onChange={(v) => setClock(v as "12h" | "24h")}
            options={[{ label: t("clock.24h"), value: "24h" }, { label: t("clock.12h"), value: "12h" }]}
          />
        </Field>
        <Field label={t("settings.timezone")} hint={t("settings.timezoneHint")}>
          <Select
            value={tz}
            onChange={setTz}
            options={[{ label: `Browser (${detected})`, value: "" }, ...zones.map((z) => ({ label: z, value: z }))]}
          />
        </Field>
      </div>
    </div>
  );
}

function FlagsCard() {
  const { t } = usePrefs();
  const toast = useToast();
  const qc = useQueryClient();
  const flags = useQuery({ queryKey: ["flags"], queryFn: api.getFlags });
  const set = useMutation({
    mutationFn: (v: boolean) => api.setFlags({ nonadminCreateProject: v }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ["flags"] }); toast.success(t("set.saved")); },
    onError: (e) => toast.error((e as Error).message),
  });
  return (
    <div className="card mb-4 p-4">
      <h2 className="mb-1 text-sm font-semibold text-ink">{t("set.flags")}</h2>
      <p className="mb-3 text-xs text-ink-faint">{t("set.flagsHint")}</p>
      <Toggle
        checked={!!flags.data?.nonadminCreateProject}
        onChange={(v) => set.mutate(v)}
        label={t("set.flagNonadminCreate")}
      />
    </div>
  );
}

function TaskSettingsCard() {
  const { t } = usePrefs();
  const toast = useToast();
  const qc = useQueryClient();
  const ts = useQuery({ queryKey: ["taskSettings"], queryFn: api.getTaskSettings });
  const [mins, setMins] = useState("");
  const [alertMins, setAlertMins] = useState("");
  const [maxPar, setMaxPar] = useState("");
  useEffect(() => {
    if (ts.data) {
      setMins(ts.data.maxDurationSec ? String(Math.round(ts.data.maxDurationSec / 60)) : "");
      setAlertMins(ts.data.longRunAlertSec ? String(Math.round(ts.data.longRunAlertSec / 60)) : "");
      setMaxPar(ts.data.maxParallel ? String(ts.data.maxParallel) : "");
    }
  }, [ts.data]);
  const save = useMutation({
    mutationFn: () => api.setTaskSettings({
      maxDurationSec: Math.max(0, Math.round((parseFloat(mins) || 0) * 60)),
      longRunAlertSec: Math.max(0, Math.round((parseFloat(alertMins) || 0) * 60)),
      maxParallel: Math.max(0, Math.round(parseFloat(maxPar) || 0)),
    }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ["taskSettings"] }); qc.invalidateQueries({ queryKey: ["systemInfo"] }); toast.success(t("set.saved")); },
    onError: (e) => toast.error((e as Error).message),
  });
  return (
    <div className="card mb-4 p-4">
      <h2 className="mb-1 flex items-center gap-2 text-sm font-semibold text-ink"><Timer size={15} /> {t("set.taskSettings")}</h2>
      <p className="mb-3 text-xs text-ink-faint">{t("set.maxDurationHint")}</p>
      <div className="flex flex-wrap items-end gap-3">
        <Field label={t("set.maxDuration")}>
          <input type="number" min={0} className="input w-40" value={mins} onChange={(e) => setMins(e.target.value)} placeholder={t("set.unlimited")} />
        </Field>
        <Field label={t("set.longRunAlert")} hint={t("set.longRunAlertHint")}>
          <input type="number" min={0} className="input w-40" value={alertMins} onChange={(e) => setAlertMins(e.target.value)} placeholder={t("set.off")} />
        </Field>
        <Field label={t("set.maxParallel")} hint={t("set.maxParallelHint")}>
          <input type="number" min={0} className="input w-40" value={maxPar} onChange={(e) => setMaxPar(e.target.value)} placeholder={t("set.unlimited")} />
        </Field>
        <button className="btn-primary mb-0.5" onClick={() => save.mutate()} disabled={save.isPending}>
          {save.isPending ? <Spinner /> : null} {t("common.save")}
        </button>
      </div>
    </div>
  );
}

function AuthMappingCard() {
  const { t } = usePrefs();
  const toast = useToast();
  const qc = useQueryClient();
  const cfg = useQuery({ queryKey: ["authMapping"], queryFn: api.getAuthMapping });
  const [rules, setRules] = useState<{ group: string; role: string }[]>([]);
  const [ldapAttr, setLdapAttr] = useState("memberOf");
  const [oidcClaim, setOidcClaim] = useState("groups");
  const [ldapDebug, setLdapDebug] = useState(false);
  const [oidcRequiredClaim, setOidcRequiredClaim] = useState("");
  const [oidcAutoLogin, setOidcAutoLogin] = useState(false);
  useEffect(() => {
    if (cfg.data) {
      setRules(cfg.data.rules ?? []); setLdapAttr(cfg.data.ldapGroupAttr || "memberOf"); setOidcClaim(cfg.data.oidcGroupsClaim || "groups"); setLdapDebug(!!cfg.data.ldapDebug);
      setOidcRequiredClaim(cfg.data.oidcRequiredClaim || ""); setOidcAutoLogin(!!cfg.data.oidcAutoLogin);
    }
  }, [cfg.data]);
  const save = useMutation({
    mutationFn: () => api.setAuthMapping({ rules: rules.filter((r) => r.group.trim()), ldapGroupAttr: ldapAttr.trim(), oidcGroupsClaim: oidcClaim.trim(), ldapDebug, oidcRequiredClaim: oidcRequiredClaim.trim(), oidcAutoLogin }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ["authMapping"] }); toast.success(t("set.saved")); },
    onError: (e) => toast.error((e as Error).message),
  });
  const roleOpts = [{ label: t("set.roleAdmin"), value: "admin" }, { label: t("set.roleUser"), value: "user" }];
  return (
    <div className="card mb-4 p-4">
      <h2 className="mb-1 flex items-center gap-2 text-sm font-semibold text-ink"><Shield size={15} /> {t("set.authMapping")}</h2>
      <p className="mb-3 text-xs text-ink-faint">{t("set.authMappingHint")}</p>
      <div className="space-y-2">
        {rules.map((row, i) => (
          <div key={i} className="flex items-center gap-2">
            <input className="input flex-1 font-mono text-xs" value={row.group} placeholder={t("set.authGroup")}
              onChange={(e) => setRules((cur) => cur.map((r, j) => (j === i ? { ...r, group: e.target.value } : r)))} />
            <span className="text-ink-faint">→</span>
            <div className="w-36 shrink-0">
              <Select value={row.role || "user"} onChange={(v) => setRules((cur) => cur.map((r, j) => (j === i ? { ...r, role: v } : r)))} options={roleOpts} />
            </div>
            <button className="btn-ghost p-1 hover:text-danger" onClick={() => setRules((cur) => cur.filter((_, j) => j !== i))}><Trash2 size={14} /></button>
          </div>
        ))}
      </div>
      <button className="btn-ghost mt-2 text-sm" onClick={() => setRules((cur) => [...cur, { group: "", role: "user" }])}><Plus size={14} /> {t("set.authAddRule")}</button>
      <div className="mt-3 grid gap-3 sm:grid-cols-2">
        <Field label={t("set.authLdapAttr")}><input className="input font-mono text-xs" value={ldapAttr} onChange={(e) => setLdapAttr(e.target.value)} placeholder="memberOf" /></Field>
        <Field label={t("set.authOidcClaim")}><input className="input font-mono text-xs" value={oidcClaim} onChange={(e) => setOidcClaim(e.target.value)} placeholder="groups" /></Field>
      </div>
      <div className="mt-3 grid gap-3 sm:grid-cols-2">
        <Field label={t("set.oidcRequiredClaim")} hint={t("set.oidcRequiredClaimHint")}>
          <input className="input font-mono text-xs" value={oidcRequiredClaim} onChange={(e) => setOidcRequiredClaim(e.target.value)} placeholder="hd=example.com" />
        </Field>
      </div>
      <div className="mt-3 space-y-2">
        <Toggle checked={oidcAutoLogin} onChange={setOidcAutoLogin} label={t("set.oidcAutoLogin")} />
        <p className="text-xs text-ink-faint">{t("set.oidcAutoLoginHint")}</p>
        <Toggle checked={ldapDebug} onChange={setLdapDebug} label={t("set.ldapDebug")} />
        <p className="text-xs text-ink-faint">{t("set.ldapDebugHint")}</p>
      </div>
      <div className="mt-3"><button className="btn-primary" onClick={() => save.mutate()} disabled={save.isPending}>{save.isPending ? <Spinner /> : null} {t("common.save")}</button></div>
    </div>
  );
}

function SMTPCard() {
  const { t } = usePrefs();
  const toast = useToast();
  const qc = useQueryClient();
  const cfg = useQuery({ queryKey: ["smtp"], queryFn: api.getSMTP });
  const [host, setHost] = useState("");
  const [port, setPort] = useState("587");
  const [from, setFrom] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  useEffect(() => {
    if (cfg.data) { setHost(cfg.data.host); setPort(cfg.data.port || "587"); setFrom(cfg.data.from); setUsername(cfg.data.username); setPassword(cfg.data.password); }
  }, [cfg.data]);
  const save = useMutation({
    mutationFn: () => api.setSMTP({ host: host.trim(), port: port.trim(), from: from.trim(), username: username.trim(), password }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ["smtp"] }); toast.success(t("set.saved")); },
    onError: (e) => toast.error((e as Error).message),
  });
  return (
    <div className="card mb-4 p-4">
      <h2 className="mb-1 flex items-center gap-2 text-sm font-semibold text-ink"><Send size={15} /> {t("set.smtp")}</h2>
      <p className="mb-3 text-xs text-ink-faint">{t("set.smtpHint")}</p>
      <div className="grid gap-3 sm:grid-cols-2">
        <Field label={t("set.fldSmtpHost")}><input className="input font-mono text-xs" value={host} onChange={(e) => setHost(e.target.value)} placeholder="smtp.example.com" /></Field>
        <Field label={t("set.fldPort")}><input className="input font-mono text-xs" value={port} onChange={(e) => setPort(e.target.value)} placeholder="587" /></Field>
        <Field label={t("set.fldFrom")}><input className="input font-mono text-xs" value={from} onChange={(e) => setFrom(e.target.value)} placeholder="ansible-ui@example.com" /></Field>
        <Field label={t("common.username")}><input className="input font-mono text-xs" value={username} onChange={(e) => setUsername(e.target.value)} placeholder="(optional)" /></Field>
        <Field label={t("common.password")}><input type="password" className="input font-mono text-xs" value={password} onChange={(e) => setPassword(e.target.value)} placeholder="(optional)" /></Field>
      </div>
      <div className="mt-3"><button className="btn-primary" onClick={() => save.mutate()} disabled={save.isPending}>{save.isPending ? <Spinner /> : null} {t("common.save")}</button></div>
    </div>
  );
}

function GalaxyCard() {
  const { t } = usePrefs();
  const toast = useToast();
  const qc = useQueryClient();
  const cfg = useQuery({ queryKey: ["galaxy"], queryFn: api.getGalaxy });
  const [serverUrl, setServerUrl] = useState("");
  const [token, setToken] = useState("");
  const [cliArgs, setCliArgs] = useState("");
  useEffect(() => {
    if (cfg.data) { setServerUrl(cfg.data.serverUrl); setToken(cfg.data.token); setCliArgs(cfg.data.cliArgs); }
  }, [cfg.data]);
  const save = useMutation({
    mutationFn: () => api.setGalaxy({ serverUrl: serverUrl.trim(), token: token.trim(), cliArgs: cliArgs.trim() }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ["galaxy"] }); toast.success(t("set.saved")); },
    onError: (e) => toast.error((e as Error).message),
  });
  return (
    <div className="card mb-4 p-4">
      <h2 className="mb-1 flex items-center gap-2 text-sm font-semibold text-ink"><Package size={15} /> {t("set.galaxy")}</h2>
      <p className="mb-3 text-xs text-ink-faint">{t("set.galaxyHint")}</p>
      <div className="grid gap-3">
        <Field label={t("set.fldGalaxyUrl")}><input className="input font-mono text-xs" value={serverUrl} onChange={(e) => setServerUrl(e.target.value)} placeholder="https://hub.example.com/api/galaxy/" /></Field>
        <div className="grid gap-3 sm:grid-cols-2">
          <Field label={t("set.fldGalaxyToken")}><input type="password" className="input font-mono text-xs" value={token} onChange={(e) => setToken(e.target.value)} placeholder="(optional)" /></Field>
          <Field label={t("set.fldGalaxyArgs")}><input className="input font-mono text-xs" value={cliArgs} onChange={(e) => setCliArgs(e.target.value)} placeholder="--ignore-certs --timeout 60" /></Field>
        </div>
      </div>
      <div className="mt-3"><button className="btn-primary" onClick={() => save.mutate()} disabled={save.isPending}>{save.isPending ? <Spinner /> : null} {t("common.save")}</button></div>
    </div>
  );
}

function NotifyProxyCard() {
  const { t } = usePrefs();
  const toast = useToast();
  const qc = useQueryClient();
  const cfg = useQuery({ queryKey: ["notifyProxy"], queryFn: api.getNotifyProxy });
  const [proxyUrl, setProxyUrl] = useState("");
  useEffect(() => { if (cfg.data) setProxyUrl(cfg.data.proxyUrl); }, [cfg.data]);
  const save = useMutation({
    mutationFn: () => api.setNotifyProxy(proxyUrl.trim()),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ["notifyProxy"] }); toast.success(t("set.saved")); },
    onError: (e) => toast.error((e as Error).message),
  });
  return (
    <div className="card mb-4 p-4">
      <h2 className="mb-1 flex items-center gap-2 text-sm font-semibold text-ink"><Bell size={15} /> {t("set.notifyProxy")}</h2>
      <p className="mb-3 text-xs text-ink-faint">{t("set.notifyProxyHint")}</p>
      <Field label={t("set.fldNotifyProxy")}>
        <input className="input font-mono text-xs" value={proxyUrl} onChange={(e) => setProxyUrl(e.target.value)} placeholder="http://proxy.corp:3128" />
      </Field>
      <div className="mt-3"><button className="btn-primary" onClick={() => save.mutate()} disabled={save.isPending}>{save.isPending ? <Spinner /> : null} {t("common.save")}</button></div>
    </div>
  );
}

function SyslogCard() {
  const { t } = usePrefs();
  const toast = useToast();
  const qc = useQueryClient();
  const cfg = useQuery({ queryKey: ["syslog"], queryFn: api.getSyslog });
  const [enabled, setEnabled] = useState(false);
  const [address, setAddress] = useState("");
  const [protocol, setProtocol] = useState("udp");
  const [tag, setTag] = useState("ansible-ui");
  const [httpEnabled, setHttpEnabled] = useState(false);
  const [httpUrl, setHttpUrl] = useState("");
  const [httpToken, setHttpToken] = useState("");
  const [httpFormat, setHttpFormat] = useState("splunk");
  useEffect(() => {
    if (cfg.data) {
      setEnabled(cfg.data.enabled); setAddress(cfg.data.address); setProtocol(cfg.data.protocol || "udp"); setTag(cfg.data.tag || "ansible-ui");
      setHttpEnabled(cfg.data.httpEnabled); setHttpUrl(cfg.data.httpUrl || ""); setHttpToken(cfg.data.httpToken || ""); setHttpFormat(cfg.data.httpFormat || "splunk");
    }
  }, [cfg.data]);
  const save = useMutation({
    mutationFn: () => api.setSyslog({ enabled, address: address.trim(), protocol, tag: tag.trim(), httpEnabled, httpUrl: httpUrl.trim(), httpToken: httpToken.trim(), httpFormat }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ["syslog"] }); toast.success(t("set.saved")); },
    onError: (e) => toast.error((e as Error).message),
  });
  return (
    <div className="card mb-4 p-4">
      <h2 className="mb-1 flex items-center gap-2 text-sm font-semibold text-ink"><Database size={15} /> {t("set.syslog")}</h2>
      <p className="mb-3 text-xs text-ink-faint">{t("set.syslogHint")}</p>
      <div className="mb-3"><Toggle checked={enabled} onChange={setEnabled} label={enabled ? t("common.enabled") : t("common.disabled")} /></div>
      <div className="grid gap-3 sm:grid-cols-3">
        <Field label={t("set.syslogAddress")}>
          <input className="input font-mono text-xs" value={address} onChange={(e) => setAddress(e.target.value)} placeholder="logs.example.com:514" />
        </Field>
        <Field label={t("set.syslogProtocol")}>
          <Select value={protocol} onChange={setProtocol} options={[{ label: "UDP", value: "udp" }, { label: "TCP", value: "tcp" }]} />
        </Field>
        <Field label={t("set.syslogTag")}>
          <input className="input font-mono text-xs" value={tag} onChange={(e) => setTag(e.target.value)} placeholder="ansible-ui" />
        </Field>
      </div>
      <div className="mt-4 border-t border-border pt-3">
        <div className="mb-2"><Toggle checked={httpEnabled} onChange={setHttpEnabled} label={t("set.httpSink")} /></div>
        <div className="grid gap-3 sm:grid-cols-3">
          <Field label={t("set.httpSinkFormat")}>
            <Select value={httpFormat} onChange={setHttpFormat} options={[
              { label: "Splunk HEC", value: "splunk" },
              { label: "Elasticsearch", value: "elasticsearch" },
              { label: "Generic", value: "generic" },
            ]} />
          </Field>
          <Field label={t("set.httpSinkUrl")}>
            <input className="input font-mono text-xs" value={httpUrl} onChange={(e) => setHttpUrl(e.target.value)} placeholder={httpFormat === "elasticsearch" ? "https://es:9200/ansible-ui/_doc" : "https://splunk:8088/services/collector"} />
          </Field>
          <Field label={t("set.httpSinkToken")}>
            <input className="input font-mono text-xs" type="password" value={httpToken} onChange={(e) => setHttpToken(e.target.value)} placeholder={httpFormat === "splunk" ? "HEC token" : "ApiKey … / Bearer …"} />
          </Field>
        </div>
      </div>
      <div className="mt-3">
        <button className="btn-primary" onClick={() => save.mutate()} disabled={save.isPending}>
          {save.isPending ? <Spinner /> : null} {t("common.save")}
        </button>
      </div>
    </div>
  );
}

function CapRow({ label, ok }: { label: string; ok: boolean }) {
  return (
    <div className="flex items-center justify-between border-b border-border/60 py-1.5 last:border-0">
      <span className="text-sm text-ink-dim">{label}</span>
      {ok ? <CircleCheck size={16} className="text-success" /> : <CircleX size={16} className="text-ink-faint/50" />}
    </div>
  );
}

function ValRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between border-b border-border/60 py-1.5 last:border-0">
      <span className="text-sm text-ink-dim">{label}</span>
      <span className="font-mono text-xs text-ink">{value}</span>
    </div>
  );
}

function SystemInfoCard() {
  const { t } = usePrefs();
  const sys = useQuery({ queryKey: ["systemInfo"], queryFn: api.getSystemInfo });
  if (!sys.data) {
    return (
      <div className="card mb-4 p-4">
        <h2 className="mb-3 flex items-center gap-2 text-sm font-semibold text-ink"><Info size={15} /> {t("set.system")}</h2>
        <Spinner />
      </div>
    );
  }
  const d = sys.data;
  const authLabel: Record<string, string> = {
    local: t("set.authLocal"), totp: "TOTP (2FA)", ldap: "LDAP", oidc: "OpenID Connect",
    radius: "RADIUS", tacacs: "TACACS+", saml: "SAML", github: "GitHub", bitbucket: "Bitbucket", emailOtp: "Email OTP",
  };
  const notifyLabel: Record<string, string> = {
    telegram: "Telegram", slack: "Slack", teams: "Microsoft Teams", discord: "Discord",
    rocketchat: "Rocket.Chat", googlechat: "Google Chat", gotify: "Gotify", ntfy: "ntfy",
    pushover: "Pushover", dingtalk: "DingTalk", webhook: "Webhook", email: "Email",
  };
  const section = "mb-3 text-[11px] font-semibold uppercase tracking-wide text-ink-faint";
  return (
    <div className="card mb-4 p-4">
      <h2 className="mb-4 flex items-center gap-2 text-sm font-semibold text-ink"><Info size={15} /> {t("set.system")}</h2>
      <div className="grid gap-x-8 gap-y-4 md:grid-cols-2">
        <div>
          <div className={section}>{t("set.sysSystem")}</div>
          <ValRow label={t("set.sysVersion")} value={d.system.version} />
          <ValRow label="Go" value={d.system.goVersion} />
          <ValRow label={t("set.sysPlatform")} value={d.system.platform} />
          <ValRow label={t("set.sysDatabase")} value={d.database.dialect} />
          <ValRow label={t("set.maxDuration")} value={d.taskSettings.maxDurationSec ? `${Math.round(d.taskSettings.maxDurationSec / 60)} ${t("set.minutes")}` : t("set.unlimited")} />
        </div>
        <div>
          <div className={section}>{t("set.sysAuth")}</div>
          {Object.entries(d.auth).map(([k, v]) => <CapRow key={k} label={authLabel[k] ?? k} ok={v} />)}
        </div>
        <div>
          <div className={section}>{t("set.sysNotifications")}</div>
          {d.notifications.map((n) => <CapRow key={n.type} label={notifyLabel[n.type] ?? n.type} ok={n.configured} />)}
        </div>
        <div>
          <div className={section}>{t("set.sysClusterRunners")}</div>
          <CapRow label={t("set.sysHA")} ok={d.cluster.highAvailability} />
          <CapRow label={t("set.sysRemoteRunners")} ok={d.runners.remoteRunners} />
          <ValRow label={t("set.sysRunnersOnline")} value={`${d.runners.online} / ${d.runners.total}`} />
          <div className={`${section} mt-4`}>{t("set.flags")}</div>
          {Object.entries(d.featureFlags).map(([k, v]) => <CapRow key={k} label={k === "nonadminCreateProject" ? t("set.flagNonadminCreate") : k} ok={v} />)}
        </div>
      </div>
      {d.ansible && (
        <div className="mt-5">
          <div className={section}>Ansible</div>
          <pre className="max-h-56 overflow-auto rounded-lg border border-border bg-surface p-3 font-mono text-[11px] leading-relaxed text-ink-dim">{d.ansible.trim()}</pre>
        </div>
      )}
    </div>
  );
}

function DocsToggleCard() {
  const { t } = usePrefs();
  const { docsEnabled, refresh } = useAuth();
  const [busy, setBusy] = useState(false);
  const toggle = async (v: boolean) => {
    setBusy(true);
    try {
      await api.setDocsEnabled(v);
      await refresh();
    } finally {
      setBusy(false);
    }
  };
  return (
    <div className="card mb-4 p-4">
      <h2 className="mb-1 text-sm font-semibold text-ink">{t("set.docsTitle")}</h2>
      <p className="mb-3 text-xs text-ink-faint">{t("set.docsHint")}</p>
      <div className="flex items-center gap-3">
        <Toggle checked={docsEnabled} onChange={toggle} label={docsEnabled ? t("common.enabled") : t("common.disabled")} />
        {busy && <Spinner />}
      </div>
    </div>
  );
}

const NOTIFY_EVENTS = ["success", "failed", "canceled", "fixed", "long-running", "host-down", "host-up"];

function NotificationsCard() {
  const { t } = usePrefs();
  const toast = useToast();
  const confirm = useConfirm();
  const qc = useQueryClient();
  const channels = useQuery({ queryKey: ["notifications"], queryFn: api.listNotifications });
  const projects = useQuery({ queryKey: ["projects"], queryFn: api.listProjects });
  const projName = (id?: string | null) => (id ? projects.data?.find((p) => p.id === id)?.name ?? id : "");
  const [creating, setCreating] = useState(false);
  const [type, setType] = useState<NotifyType>("telegram");
  const [name, setName] = useState("");
  const [config, setConfig] = useState<Record<string, string>>({});
  const [events, setEvents] = useState<string[]>([]);
  const [projectId, setProjectId] = useState("");
  const [template, setTemplate] = useState("");
  const [error, setError] = useState("");
  const [testing, setTesting] = useState("");

  const reset = () => { setCreating(false); setName(""); setConfig({}); setEvents([]); setProjectId(""); setTemplate(""); setError(""); };
  const create = useMutation({
    mutationFn: () => api.createNotification({ type, name, enabled: true, events, config, projectId: projectId || undefined, template: template.trim() || undefined }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ["notifications"] }); reset(); },
    onError: (e) => setError((e as Error).message),
  });
  const del = useMutation({
    mutationFn: (id: string) => api.deleteNotification(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["notifications"] }),
  });
  const test = async (id: string) => {
    setTesting(id);
    try { await api.testNotification(id); toast.success(t("set.testSent")); }
    catch (e) { toast.error((e as Error).message); }
    finally { setTesting(""); }
  };

  const fields: Record<NotifyType, { key: string; label: string; ph: string }[]> = {
    telegram: [
      { key: "botToken", label: t("set.fldBotToken"), ph: "123456:ABC-DEF…" },
      { key: "chatId", label: t("set.fldChatId"), ph: "-1001234567890" },
      { key: "threadId", label: t("set.fldThreadId"), ph: "(optional)" },
    ],
    slack: [{ key: "url", label: t("set.fldWebhookUrl"), ph: "https://hooks.slack.com/services/…" }],
    webhook: [{ key: "url", label: t("set.fldUrl"), ph: "https://example.com/hook" }],
    email: [
      { key: "host", label: t("set.fldSmtpHost"), ph: "smtp.example.com" },
      { key: "port", label: t("set.fldPort"), ph: "587" },
      { key: "from", label: t("set.fldFrom"), ph: "ansible-ui@example.com" },
      { key: "to", label: t("set.fldTo"), ph: "ops@example.com, oncall@example.com" },
      { key: "username", label: t("common.username"), ph: "(optional)" },
      { key: "password", label: t("common.password"), ph: "(optional)" },
    ],
    discord: [{ key: "url", label: t("set.fldWebhookUrl"), ph: "https://discord.com/api/webhooks/…" }],
    teams: [{ key: "url", label: t("set.fldWebhookUrl"), ph: "https://outlook.office.com/webhook/…" }],
    rocketchat: [{ key: "url", label: t("set.fldWebhookUrl"), ph: "https://chat.example.com/hooks/…" }],
    googlechat: [{ key: "url", label: t("set.fldWebhookUrl"), ph: "https://chat.googleapis.com/v1/spaces/…" }],
    dingtalk: [{ key: "url", label: t("set.fldWebhookUrl"), ph: "https://oapi.dingtalk.com/robot/send?access_token=…" }],
    gotify: [
      { key: "url", label: t("set.fldServerUrl"), ph: "https://gotify.example.com" },
      { key: "token", label: t("set.fldToken"), ph: "Axxxxxxxxxxxx" },
    ],
    ntfy: [
      { key: "url", label: t("set.fldTopicUrl"), ph: "https://ntfy.sh/my-topic" },
      { key: "token", label: t("set.fldToken") + " (" + t("common.optional") + ")", ph: "tk_…" },
    ],
    pushover: [
      { key: "token", label: t("set.fldAppToken"), ph: "axxxxxxxxxxxx" },
      { key: "user", label: t("set.fldUserKey"), ph: "uxxxxxxxxxxxx" },
    ],
    pagerduty: [
      { key: "routingKey", label: t("set.fldRoutingKey"), ph: "R0xxxxxxxxxxxxxxxxxxxx" },
      { key: "endpoint", label: t("set.fldEndpoint"), ph: "https://events.pagerduty.com/v2/enqueue" },
    ],
    opsgenie: [
      { key: "apiKey", label: t("set.fldApiKey"), ph: "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" },
      { key: "endpoint", label: t("set.fldEndpoint"), ph: "https://api.opsgenie.com/v2/alerts" },
    ],
  };

  return (
    <div className="card mb-4">
      <div className="flex items-center justify-between border-b border-border px-4 py-3">
        <div>
          <h2 className="flex items-center gap-1.5 text-sm font-semibold text-ink"><Bell size={15} /> {t("set.notifications")}</h2>
          <p className="text-xs text-ink-faint">{t("set.notificationsHint")}</p>
        </div>
        <button className="btn-outline" onClick={() => setCreating(true)}><Plus size={15} /> {t("set.addChannel")}</button>
      </div>
      {channels.data?.length ? (
        <div className="divide-y divide-border">
          {channels.data.map((c) => (
            <div key={c.id} className="flex items-center gap-3 px-4 py-3">
              <Bell size={16} className={c.enabled ? "text-accent" : "text-ink-faint"} />
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <span className="truncate text-sm text-ink">{c.name || c.type}</span>
                  <span className="chip text-[10px] uppercase">{c.type}</span>
                  <span className="chip text-[10px]">{c.projectId ? projName(c.projectId) : t("set.allProjects")}</span>
                </div>
                <div className="truncate text-xs text-ink-faint">
                  {c.events.length ? c.events.join(", ") : t("set.allTerminal")}
                </div>
              </div>
              <button className="btn-ghost p-1.5" title={t("set.sendTest")} onClick={() => test(c.id)} disabled={testing === c.id}>
                {testing === c.id ? <Spinner /> : <Send size={15} />}
              </button>
              <button className="btn-ghost p-1.5" title={t("common.delete")} onClick={() => confirm(t("set.deleteChannel")).then((ok) => ok && del.mutate(c.id))}>
                <Trash2 size={15} />
              </button>
            </div>
          ))}
        </div>
      ) : (
        <div className="px-4 py-6">
          <EmptyState icon={<Bell size={22} />} title={t("set.noChannels")} description={t("set.noChannelsHint")} />
        </div>
      )}
      {creating && (
        <Modal
          open
          onClose={reset}
          title={t("set.addChannelTitle")}
          footer={
            <>
              <button className="btn-outline" onClick={reset}>{t("common.cancel")}</button>
              <button className="btn-primary" onClick={() => { setError(""); create.mutate(); }} disabled={create.isPending}>
                {create.isPending ? <Spinner /> : null} {t("common.create")}
              </button>
            </>
          }
        >
          <div className="space-y-4">
            <div className="grid grid-cols-2 gap-4">
              <Field label={t("common.type")}>
                <Select
                  value={type}
                  onChange={(v) => { setType(v as NotifyType); setConfig({}); }}
                  options={[
                    { label: "Telegram", value: "telegram" },
                    { label: "Slack", value: "slack" },
                    { label: "Microsoft Teams", value: "teams" },
                    { label: "Discord", value: "discord" },
                    { label: "Rocket.Chat", value: "rocketchat" },
                    { label: "Google Chat", value: "googlechat" },
                    { label: "Gotify", value: "gotify" },
                    { label: "ntfy", value: "ntfy" },
                    { label: "Pushover", value: "pushover" },
                    { label: "DingTalk", value: "dingtalk" },
                    { label: "PagerDuty", value: "pagerduty" },
                    { label: "Opsgenie", value: "opsgenie" },
                    { label: "Webhook", value: "webhook" },
                    { label: "Email (SMTP)", value: "email" },
                  ]}
                />
              </Field>
              <Field label={t("common.name")} hint={t("common.optional")}>
                <input className="input" value={name} onChange={(e) => setName(e.target.value)} placeholder="ops-alerts" />
              </Field>
            </div>
            {fields[type].map((f) => (
              <Field key={f.key} label={f.label}>
                <input className="input font-mono text-xs" value={config[f.key] ?? ""} placeholder={f.ph}
                  onChange={(e) => setConfig((c) => ({ ...c, [f.key]: e.target.value }))} />
              </Field>
            ))}
            <Field label={t("set.notifyOn")} hint={t("set.notifyOnHint")}>
              <div className="flex gap-4">
                {NOTIFY_EVENTS.map((ev) => (
                  <Checkbox
                    key={ev}
                    checked={events.includes(ev)}
                    onChange={(v) => setEvents((cur) => (v ? [...cur, ev] : cur.filter((x) => x !== ev)))}
                    label={ev}
                  />
                ))}
              </div>
            </Field>
            <Field label={t("set.notifyProject")} hint={t("set.notifyProjectHint")}>
              <Select
                value={projectId}
                onChange={setProjectId}
                options={[
                  { label: t("set.allProjects"), value: "" },
                  ...(projects.data ?? []).map((p) => ({ label: p.name, value: p.id })),
                ]}
              />
            </Field>
            <Field label={t("set.msgTemplate")} hint={t("set.msgTemplateHint")}>
              <textarea
                className="input min-h-[64px] font-mono text-xs"
                value={template}
                onChange={(e) => setTemplate(e.target.value)}
                placeholder={"{{run}} → {{status}} ({{project}})"}
              />
            </Field>
            {error && <ErrorText>{error}</ErrorText>}
          </div>
        </Modal>
      )}
    </div>
  );
}

function NotificationLogCard() {
  const { t } = usePrefs();
  // Live: poll the delivery log so new dispatches appear without a refresh.
  const logs = useQuery({
    queryKey: ["notification-logs"],
    queryFn: api.listNotificationLogs,
    refetchInterval: 10000,
  });
  return (
    <div className="card mb-4">
      <div className="border-b border-border px-4 py-3">
        <h2 className="flex items-center gap-1.5 text-sm font-semibold text-ink"><Send size={15} /> {t("set.notifyLog")}</h2>
        <p className="text-xs text-ink-faint">{t("set.notifyLogHint")}</p>
      </div>
      {logs.data?.length ? (
        <div className="divide-y divide-border">
          {logs.data.map((l) => (
            <div key={l.id} className="flex items-center gap-3 px-4 py-2.5">
              <StatusBadge status={l.ok ? "success" : "failed"} />
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <span className="truncate text-sm text-ink">{l.channelName || l.channelType}</span>
                  <span className="chip text-[10px] uppercase">{l.channelType}</span>
                  <span className="chip text-[10px]">{l.event}</span>
                </div>
                <div className="truncate text-xs text-ink-faint">
                  {l.runName || l.runId}
                  {l.error ? <span className="text-danger"> · {l.error}</span> : null}
                </div>
              </div>
              <span className="whitespace-nowrap text-xs text-ink-faint">{fmtRelative(l.createdAt)}</span>
            </div>
          ))}
        </div>
      ) : (
        <div className="px-4 py-6">
          <EmptyState icon={<Send size={22} />} title={t("set.noNotifyLog")} description={t("set.noNotifyLogHint")} />
        </div>
      )}
    </div>
  );
}

const PROJECT_CAPS = ["view", "run", "edit", "manage"] as const;

function CustomRolesCard() {
  const { t } = usePrefs();
  const toast = useToast();
  const confirm = useConfirm();
  const qc = useQueryClient();
  const roles = useQuery({ queryKey: ["roles"], queryFn: api.listRoles });
  const [name, setName] = useState("");
  const [perms, setPerms] = useState<string[]>(["view"]);
  const reset = () => { setName(""); setPerms(["view"]); };
  const create = useMutation({
    mutationFn: () => api.createRole({ name: name.trim(), permissions: perms }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ["roles"] }); reset(); toast.success(t("set.saved")); },
    onError: (e) => toast.error((e as Error).message),
  });
  const del = useMutation({
    mutationFn: (id: string) => api.deleteRole(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["roles"] }),
  });
  const toggle = (c: string, on: boolean) => setPerms((cur) => (on ? [...new Set([...cur, c])] : cur.filter((x) => x !== c)));
  return (
    <div className="card mb-4 p-4">
      <h2 className="mb-1 flex items-center gap-2 text-sm font-semibold text-ink"><Shield size={15} /> {t("set.customRoles")}</h2>
      <p className="mb-3 text-xs text-ink-faint">{t("set.customRolesHint")}</p>
      {roles.data?.length ? (
        <div className="mb-3 divide-y divide-border rounded-lg border border-border">
          {roles.data.map((rl) => (
            <div key={rl.id} className="flex items-center gap-3 px-3 py-2">
              <Shield size={14} className="text-accent" />
              <span className="text-sm text-ink">{rl.name}</span>
              <div className="flex flex-wrap gap-1">
                {rl.permissions.map((p) => <span key={p} className="chip text-[10px] uppercase">{p}</span>)}
              </div>
              <button className="btn-ghost ml-auto p-1 hover:text-danger" title={t("common.delete")}
                onClick={() => confirm({ title: t("set.deleteRole"), body: rl.name }).then((ok) => ok && del.mutate(rl.id))}>
                <Trash2 size={14} />
              </button>
            </div>
          ))}
        </div>
      ) : (
        <p className="mb-3 text-xs text-ink-faint">{t("set.noCustomRoles")}</p>
      )}
      <div className="flex flex-wrap items-end gap-3">
        <Field label={t("common.name")}>
          <input className="input w-44" value={name} onChange={(e) => setName(e.target.value)} placeholder="operator" />
        </Field>
        <div>
          <span className="label">{t("set.permissions")}</span>
          <div className="flex gap-3">
            {PROJECT_CAPS.map((c) => (
              <Checkbox key={c} checked={perms.includes(c)} onChange={(v) => toggle(c, v)} label={t(`set.cap.${c}`)} />
            ))}
          </div>
        </div>
        <button className="btn-primary" disabled={!name.trim() || perms.length === 0 || create.isPending} onClick={() => create.mutate()}>
          {create.isPending ? <Spinner /> : <Plus size={15} />} {t("set.addRole")}
        </button>
      </div>
    </div>
  );
}

function SecretAccessLogCard() {
  const { t } = usePrefs();
  // Live: poll the secret-access trail so new accesses surface without a refresh.
  const logs = useQuery({
    queryKey: ["secret-access-logs"],
    queryFn: api.listSecretAccessLogs,
    refetchInterval: 10000,
  });
  const actionLabel = (a: string) =>
    ({ "run-secrets": t("set.salReveal"), pubkey: t("set.salPubkey"), "credential-use": t("set.salUse") } as Record<string, string>)[a] ?? a;
  return (
    <div className="card mb-4">
      <div className="border-b border-border px-4 py-3">
        <h2 className="flex items-center gap-1.5 text-sm font-semibold text-ink"><KeyRound size={15} /> {t("set.secretAccess")}</h2>
        <p className="text-xs text-ink-faint">{t("set.secretAccessHint")}</p>
      </div>
      {logs.data?.length ? (
        <div className="divide-y divide-border">
          {logs.data.map((l) => (
            <div key={l.id} className="flex items-center gap-3 px-4 py-2.5">
              <KeyRound size={16} className="text-warn" />
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <span className="chip text-[10px] uppercase">{actionLabel(l.action)}</span>
                  <span className="truncate text-sm text-ink">{l.actor || t("common.system")}</span>
                  {l.credentialName ? <span className="chip text-[10px]">{l.credentialName}</span> : null}
                </div>
                {l.detail ? <div className="truncate text-xs text-ink-faint">{l.detail}</div> : null}
              </div>
              <span className="whitespace-nowrap text-xs text-ink-faint">{fmtRelative(l.createdAt)}</span>
            </div>
          ))}
        </div>
      ) : (
        <div className="px-4 py-6">
          <EmptyState icon={<KeyRound size={22} />} title={t("set.noSecretAccess")} description={t("set.noSecretAccessHint")} />
        </div>
      )}
    </div>
  );
}

function TwoFactorCard() {
  const { t } = usePrefs();
  const { user, refresh } = useAuth();
  const toast = useToast();
  const [setup, setSetup] = useState<{ secret: string; otpauthUrl: string; qrDataUri?: string } | null>(null);
  const [code, setCode] = useState("");
  const [busy, setBusy] = useState(false);
  const enabled = user?.twoFactorEnabled;
  const onCode = (v: string) => setCode(v.replace(/\D/g, "").slice(0, 6));

  const run = async (fn: () => Promise<unknown>, ok?: string) => {
    setBusy(true);
    try {
      await fn();
      if (ok) toast.success(ok);
    } catch (e) {
      toast.error((e as Error).message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="card mb-4">
      <div className="border-b border-border px-4 py-3">
        <h2 className="flex items-center gap-1.5 text-sm font-semibold text-ink"><Shield size={15} /> {t("set.2fa")}</h2>
        <p className="text-xs text-ink-faint">{t("set.2faHint")}</p>
      </div>
      <div className="space-y-3 p-4">
        {enabled ? (
          <>
            <div className="flex items-center gap-1.5 text-sm text-success"><Shield size={16} /> {t("set.2faEnabled")}</div>
            <div className="flex flex-wrap items-end gap-2">
              <Field label={t("set.2faCode")}>
                <input className="input w-36 text-center font-mono tracking-[0.3em]" value={code} onChange={(e) => onCode(e.target.value)} placeholder="000000" inputMode="numeric" />
              </Field>
              <button className="btn-outline text-danger" disabled={busy || code.length !== 6}
                onClick={() => run(async () => { await api.twoFactorDisable(code); setCode(""); await refresh(); }, t("set.2faOff"))}>
                {t("set.2faDisable")}
              </button>
            </div>
          </>
        ) : setup ? (
          <>
            <p className="text-xs text-ink-dim">{t("set.2faScan")}</p>
            {setup.qrDataUri && (
              <img src={setup.qrDataUri} alt="2FA QR code" width={200} height={200}
                className="rounded-lg border border-border bg-white p-2" />
            )}
            <div className="rounded-lg border border-border bg-surface p-3 text-xs">
              <div className="mb-1 text-ink-faint">{t("set.2faSecretManual")}</div>
              <div className="select-all break-all font-mono text-base tracking-wider text-ink">{setup.secret}</div>
            </div>
            <div className="flex flex-wrap items-end gap-2">
              <Field label={t("set.2faCode")} hint={t("set.2faCodeHint")}>
                <input className="input w-36 text-center font-mono tracking-[0.3em]" value={code} onChange={(e) => onCode(e.target.value)} placeholder="000000" inputMode="numeric" autoFocus />
              </Field>
              <button className="btn-primary" disabled={busy || code.length !== 6}
                onClick={() => run(async () => { await api.twoFactorEnable(code); setSetup(null); setCode(""); await refresh(); }, t("set.2faOn"))}>
                {t("set.2faVerify")}
              </button>
              <button className="btn-ghost text-sm" onClick={() => { setSetup(null); setCode(""); }}>{t("common.cancel")}</button>
            </div>
          </>
        ) : (
          <button className="btn-primary" disabled={busy} onClick={() => run(async () => setSetup(await api.twoFactorSetup()))}>
            {busy ? <Spinner /> : <Shield size={15} />} {t("set.2faEnable")}
          </button>
        )}

        <div className="border-t border-border pt-3">
          <div className="mb-1 text-sm text-ink">{t("set.emailOtp")}</div>
          <p className="mb-2 text-xs text-ink-faint">{t("set.emailOtpHint")}</p>
          {user?.emailOtpEnabled ? (
            <button className="btn-outline text-danger" disabled={busy}
              onClick={() => run(async () => { await api.disableEmailOTP(); await refresh(); }, t("set.emailOtpOff"))}>
              {t("set.emailOtpDisable")}
            </button>
          ) : (
            <button className="btn-outline" disabled={busy}
              onClick={() => run(async () => { await api.enableEmailOTP(); await refresh(); }, t("set.emailOtpOn"))}>
              {t("set.emailOtpEnable")}
            </button>
          )}
        </div>
      </div>
    </div>
  );
}

function RetentionCard() {
  const { t } = usePrefs();
  const toast = useToast();
  const qc = useQueryClient();
  const ret = useQuery({ queryKey: ["retention"], queryFn: api.getRetention });
  const [runsDays, setRunsDays] = useState("0");
  const [auditDays, setAuditDays] = useState("0");
  useEffect(() => {
    if (ret.data) {
      setRunsDays(String(ret.data.runsDays));
      setAuditDays(String(ret.data.auditDays));
    }
  }, [ret.data]);
  const save = useMutation({
    mutationFn: () => api.setRetention({ runsDays: parseInt(runsDays) || 0, auditDays: parseInt(auditDays) || 0 }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ["retention"] }); toast.success(t("set.retentionSaved")); },
    onError: (e) => toast.error((e as Error).message),
  });
  return (
    <div className="card mb-4">
      <div className="border-b border-border px-4 py-3">
        <h2 className="flex items-center gap-1.5 text-sm font-semibold text-ink"><Database size={15} /> {t("set.retention")}</h2>
        <p className="text-xs text-ink-faint">{t("set.retentionHint")}</p>
      </div>
      <div className="space-y-4 p-4">
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label={t("set.retentionRuns")} hint={t("set.retentionZero")}>
            <input className="input" type="number" min={0} value={runsDays} onChange={(e) => setRunsDays(e.target.value)} placeholder="0" />
          </Field>
          <Field label={t("set.retentionAudit")} hint={t("set.retentionZero")}>
            <input className="input" type="number" min={0} value={auditDays} onChange={(e) => setAuditDays(e.target.value)} placeholder="0" />
          </Field>
        </div>
        <button className="btn-primary" onClick={() => save.mutate()} disabled={save.isPending}>
          {save.isPending ? <Spinner /> : null} {t("common.save")}
        </button>
        <div className="border-t border-border pt-4">
          <div className="label mb-1">{t("set.export")}</div>
          <p className="mb-2 text-xs text-ink-faint">{t("set.exportHint")}</p>
          <div className="flex flex-wrap gap-2">
            <a href={api.auditExportURL()} className="btn-outline gap-1.5 text-sm"><Download size={14} /> {t("set.exportAudit")}</a>
            <a href={api.runsExportURL()} className="btn-outline gap-1.5 text-sm"><Download size={14} /> {t("set.exportRuns")}</a>
            <a href={api.complianceReportURL()} className="btn-outline gap-1.5 text-sm"><Shield size={14} /> {t("set.exportCompliance")}</a>
          </div>
        </div>
      </div>
    </div>
  );
}

function DeleteUser({ id, name }: { id: string; name: string }) {
  const { t } = usePrefs();
  const confirm = useConfirm();
  const qc = useQueryClient();
  const del = useMutation({
    mutationFn: () => api.deleteUser(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["users"] }),
  });
  return (
    <button
      className="btn-ghost p-1.5"
      title={t("set.deleteUser")}
      onClick={() => confirm({ title: t("set.deleteUser"), body: name }).then((ok) => ok && del.mutate())}
    >
      <Trash2 size={15} />
    </button>
  );
}

function Reset2FA({ id, name }: { id: string; name: string }) {
  const { t } = usePrefs();
  const confirm = useConfirm();
  const toast = useToast();
  const qc = useQueryClient();
  const reset = useMutation({
    mutationFn: () => api.resetUser2FA(id),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ["users"] }); toast.success(t("set.2faResetDone")); },
    onError: (e) => toast.error((e as Error).message),
  });
  return (
    <button
      className="btn-ghost p-1.5"
      title={t("set.2faReset")}
      onClick={() => confirm({ title: t("set.2faReset"), body: t("set.2faResetBody").replace("{user}", name) }).then((ok) => ok && reset.mutate())}
    >
      <ShieldOff size={15} />
    </button>
  );
}

function CreateUser({ onClose }: { onClose: () => void }) {
  const { t } = usePrefs();
  const qc = useQueryClient();
  const [username, setUsername] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [role, setRole] = useState("user");
  const [error, setError] = useState("");

  const create = useMutation({
    mutationFn: () => api.createUser({ username, email, password, role }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["users"] });
      onClose();
    },
    onError: (e) => setError((e as Error).message),
  });

  return (
    <Modal
      open
      onClose={onClose}
      title={t("set.addUser")}
      footer={
        <>
          <button className="btn-outline" onClick={onClose}>{t("common.cancel")}</button>
          <button className="btn-primary" onClick={() => { setError(""); create.mutate(); }} disabled={!username || password.length < 6 || create.isPending}>
            {create.isPending ? <Spinner /> : null} {t("common.create")}
          </button>
        </>
      }
    >
      <div className="space-y-4">
        <Field label={t("common.username")}>
          <input className="input" value={username} onChange={(e) => setUsername(e.target.value)} autoFocus />
        </Field>
        <Field label={t("common.email")} hint={t("common.optional")}>
          <input className="input" type="email" value={email} onChange={(e) => setEmail(e.target.value)} />
        </Field>
        <Field label={t("common.password")} hint={t("set.passwordHint")}>
          <input className="input" type="password" value={password} onChange={(e) => setPassword(e.target.value)} />
        </Field>
        <Field label={t("set.role")}>
          <Select
            value={role}
            onChange={setRole}
            options={[{ label: t("set.roleUser"), value: "user" }, { label: t("set.roleAdmin"), value: "admin" }]}
          />
        </Field>
        {error && <ErrorText>{error}</ErrorText>}
      </div>
    </Modal>
  );
}

function ApiTokens() {
  const { t, lang } = usePrefs();
  const tokens = useQuery({ queryKey: ["tokens"], queryFn: api.listTokens });
  const [creating, setCreating] = useState(false);
  return (
    <div className="card mb-4">
      <div className="flex items-center justify-between border-b border-border px-4 py-3">
        <div>
          <h2 className="text-sm font-semibold text-ink">{t("set.apiTokens")}</h2>
          <p className="text-xs text-ink-faint">{t("set.apiTokensHint")}</p>
        </div>
        <button className="btn-outline" onClick={() => setCreating(true)}>
          <Plus size={15} /> {t("set.newToken")}
        </button>
      </div>
      {tokens.isLoading ? (
        <div className="flex justify-center py-8 text-ink-faint"><Spinner /></div>
      ) : tokens.data?.length ? (
        <div className="divide-y divide-border">
          {tokens.data.map((tok) => (
            <div key={tok.id} className="flex items-center gap-3 px-4 py-3">
              <KeyRound size={17} className="text-accent" />
              <div className="min-w-0 flex-1">
                <div className="truncate text-sm text-ink">{tok.name || t("set.tokenFallback")}</div>
                <div className="truncate font-mono text-xs text-ink-faint">
                  {tok.prefix}… · {tok.lastUsedAt ? `${t("set.lastUsed")} ${fmtRelative(tok.lastUsedAt, lang)}` : t("set.neverUsed")}
                  {tok.expiresAt ? ` · ${t("set.expires")} ${fmtRelative(tok.expiresAt, lang)}` : ""}
                </div>
              </div>
              <DeleteToken id={tok.id} />
            </div>
          ))}
        </div>
      ) : (
        <div className="px-4 py-6">
          <EmptyState icon={<KeyRound size={22} />} title={t("set.noTokens")} description={t("set.noTokensHint")} />
        </div>
      )}
      {creating && <CreateTokenDialog onClose={() => setCreating(false)} />}
    </div>
  );
}

function DeleteToken({ id }: { id: string }) {
  const { t } = usePrefs();
  const confirm = useConfirm();
  const qc = useQueryClient();
  const del = useMutation({
    mutationFn: () => api.deleteToken(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["tokens"] }),
  });
  return (
    <button className="btn-ghost p-1.5" title={t("common.revoke")} onClick={() => confirm({ title: t("set.revokeToken"), confirmLabel: t("common.revoke") }).then((ok) => ok && del.mutate())}>
      <Trash2 size={15} />
    </button>
  );
}

function CreateTokenDialog({ onClose }: { onClose: () => void }) {
  const { t } = usePrefs();
  const qc = useQueryClient();
  const [name, setName] = useState("");
  const [expiresDays, setExpiresDays] = useState(0);
  const [created, setCreated] = useState<string | null>(null);
  const [error, setError] = useState("");
  const create = useMutation({
    mutationFn: () => api.createToken({ name, expiresDays: expiresDays || undefined }),
    onSuccess: (t) => {
      setCreated(t.token ?? "");
      qc.invalidateQueries({ queryKey: ["tokens"] });
    },
    onError: (e) => setError((e as Error).message),
  });

  return (
    <Modal
      open
      onClose={onClose}
      title={t("set.newTokenTitle")}
      footer={
        created ? (
          <button className="btn-primary" onClick={onClose}>{t("common.done")}</button>
        ) : (
          <>
            <button className="btn-outline" onClick={onClose}>{t("common.cancel")}</button>
            <button className="btn-primary" onClick={() => { setError(""); create.mutate(); }} disabled={!name || create.isPending}>
              {create.isPending ? <Spinner /> : null} {t("common.create")}
            </button>
          </>
        )
      }
    >
      {created ? (
        <div className="space-y-3">
          <p className="text-sm text-ink-dim">{t("set.tokenCopyHint")}</p>
          <div className="flex items-center gap-2 rounded-lg border border-border bg-surface p-2.5">
            <code className="flex-1 truncate font-mono text-xs text-accent">{created}</code>
            <button className="btn-ghost p-1.5" title={t("common.copy")} onClick={() => navigator.clipboard?.writeText(created)}>
              <Copy size={15} />
            </button>
          </div>
          <p className="text-xs text-ink-faint">
            {t("set.tokenUseHint")} <code className="font-mono">Authorization: Bearer &lt;token&gt;</code>
          </p>
        </div>
      ) : (
        <div className="space-y-4">
          <Field label={t("common.name")}>
            <input className="input" value={name} onChange={(e) => setName(e.target.value)} placeholder="ci-pipeline" autoFocus />
          </Field>
          <Field label={t("set.expiresInDays")} hint={t("set.expiresHint")}>
            <input className="input" type="number" value={expiresDays} onChange={(e) => setExpiresDays(Number(e.target.value))} />
          </Field>
          {error && <ErrorText>{error}</ErrorText>}
        </div>
      )}
    </Modal>
  );
}
