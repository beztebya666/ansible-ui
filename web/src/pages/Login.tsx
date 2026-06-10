import { useEffect, useState } from "react";
import { Terminal } from "lucide-react";
import { api } from "../lib/api";
import { useAuth } from "../lib/auth";
import { usePrefs } from "../lib/prefs";
import { ErrorText, Field, Spinner } from "../components/ui";

export function Login() {
  const { t } = usePrefs();
  const { needsSetup, setUser, providers, oidcAutoLogin } = useAuth();
  // SSO auto-login: send unauthenticated users straight to the IdP. The ?local=1
  // escape hatch (or an sso_error) shows the form instead of looping.
  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    if (oidcAutoLogin && providers.oidc && !needsSetup && !params.has("local") && !params.has("sso_error")) {
      window.location.href = "/api/auth/oidc/login";
    }
  }, [oidcAutoLogin, providers.oidc, needsSetup]);
  const [username, setUsername] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [code, setCode] = useState("");
  const [twoFactor, setTwoFactor] = useState(false);
  const [emailOtp, setEmailOtp] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setBusy(true);
    try {
      if (needsSetup) {
        setUser(await api.setup({ username, email, password }));
        return;
      }
      const res = await api.login(username, password, twoFactor || emailOtp ? code : undefined);
      if (res && "twoFactorRequired" in res) {
        setTwoFactor(true);
        setBusy(false);
        return;
      }
      if (res && "emailOtpRequired" in res) {
        setEmailOtp(true);
        setBusy(false);
        return;
      }
      setUser(res);
    } catch (err) {
      setError((err as Error).message);
      setBusy(false);
    }
  };

  return (
    <div className="flex min-h-screen items-center justify-center bg-bg px-4">
      <div className="w-full max-w-sm">
        <div className="mb-6 flex flex-col items-center text-center">
          <div className="mb-3 grid h-12 w-12 place-items-center rounded-xl bg-accent/15 text-accent">
            <Terminal size={24} />
          </div>
          <h1 className="text-xl font-semibold tracking-tight text-ink">ansible·ui</h1>
          <p className="mt-1 text-sm text-ink-dim">
            {needsSetup ? t("login.setupSub") : t("login.signinSub")}
          </p>
        </div>

        <form onSubmit={submit} className="card space-y-4 p-6">
          <Field label={t("common.username")}>
            <input
              className="input"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              autoFocus
              autoComplete="username"
            />
          </Field>
          {needsSetup && (
            <Field label={t("common.email")} hint={t("common.optional")}>
              <input
                className="input"
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                autoComplete="email"
              />
            </Field>
          )}
          <Field label={t("common.password")}>
            <input
              className="input"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete={needsSetup ? "new-password" : "current-password"}
            />
          </Field>
          {(twoFactor || emailOtp) && (
            <Field label={t("login.twoFactorCode")} hint={emailOtp ? t("login.emailOtpHint") : t("login.twoFactorHint")}>
              <input
                className="input text-center font-mono tracking-[0.4em]"
                value={code}
                onChange={(e) => setCode(e.target.value.replace(/\D/g, "").slice(0, 6))}
                inputMode="numeric"
                autoComplete="one-time-code"
                placeholder="000000"
                autoFocus
              />
            </Field>
          )}
          {error && <ErrorText>{error}</ErrorText>}
          <button className="btn-primary w-full" disabled={busy || !username || !password || ((twoFactor || emailOtp) && code.length !== 6)}>
            {busy ? <Spinner /> : null}
            {needsSetup ? t("login.createAccount") : twoFactor || emailOtp ? t("login.verify") : t("login.signin")}
          </button>

          {!needsSetup && (providers.oidc || providers.saml || providers.github || providers.bitbucket) && (
            <>
              <div className="relative py-1 text-center">
                <span className="relative bg-panel px-2 text-xs text-ink-faint">{t("login.or")}</span>
                <span className="absolute inset-x-0 top-1/2 -z-10 h-px bg-border" />
              </div>
              {providers.oidc && (
                <a href="/api/auth/oidc/login" className="btn-outline w-full justify-center">
                  {t("login.sso")}
                </a>
              )}
              {providers.saml && (
                <a href="/api/auth/saml/login" className="btn-outline w-full justify-center">
                  {t("login.saml")}
                </a>
              )}
              {providers.github && (
                <a href="/api/auth/github/login" className="btn-outline w-full justify-center">
                  {t("login.github")}
                </a>
              )}
              {providers.bitbucket && (
                <a href="/api/auth/bitbucket/login" className="btn-outline w-full justify-center">
                  {t("login.bitbucket")}
                </a>
              )}
            </>
          )}
          {!needsSetup && providers.ldap && (
            <p className="text-center text-xs text-ink-faint">{t("login.ldapNote")}</p>
          )}
        </form>
      </div>
    </div>
  );
}
