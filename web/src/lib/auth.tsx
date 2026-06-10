import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from "react";
import { api, type AuthStatus, type User } from "./api";

interface Providers {
  local: boolean;
  ldap: boolean;
  oidc: boolean;
  saml?: boolean;
  github?: boolean;
  bitbucket?: boolean;
}

interface AuthCtx {
  user: User | null;
  needsSetup: boolean;
  loading: boolean;
  providers: Providers;
  oidcAutoLogin: boolean;
  docsEnabled: boolean;
  refresh: () => Promise<void>;
  setUser: (u: User | null) => void;
}

const Ctx = createContext<AuthCtx>({
  user: null,
  needsSetup: false,
  loading: true,
  providers: { local: true, ldap: false, oidc: false },
  oidcAutoLogin: false,
  docsEnabled: true,
  refresh: async () => {},
  setUser: () => {},
});

export const useAuth = () => useContext(Ctx);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [needsSetup, setNeedsSetup] = useState(false);
  const [loading, setLoading] = useState(true);
  const [providers, setProviders] = useState<Providers>({ local: true, ldap: false, oidc: false });
  const [oidcAutoLogin, setOidcAutoLogin] = useState(false);
  const [docsEnabled, setDocsEnabled] = useState(true);

  const refresh = useCallback(async () => {
    try {
      const s: AuthStatus = await api.authStatus();
      setNeedsSetup(s.needsSetup);
      setUser(s.user);
      if (s.providers) setProviders(s.providers);
      setOidcAutoLogin(!!s.oidcAutoLogin);
      setDocsEnabled(s.docsEnabled !== false);
    } catch {
      setUser(null);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
    const onUnauth = () => setUser(null);
    window.addEventListener("auth:unauthorized", onUnauth);
    return () => window.removeEventListener("auth:unauthorized", onUnauth);
  }, [refresh]);

  return (
    <Ctx.Provider value={{ user, needsSetup, loading, providers, oidcAutoLogin, docsEnabled, refresh, setUser }}>
      {children}
    </Ctx.Provider>
  );
}
