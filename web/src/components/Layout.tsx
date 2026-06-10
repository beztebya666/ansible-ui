import { Link, NavLink, Outlet, useNavigate } from "react-router-dom";
import clsx from "clsx";
import {
  BarChart3,
  Blocks,
  Boxes,
  CalendarClock,
  FolderGit2,
  GitBranch,
  GitMerge,
  Network,
  History,
  Braces,
  KeyRound,
  LayoutDashboard,
  ListChecks,
  LogOut,
  HardDrive,
  type LucideIcon,
  Moon,
  Server,
  Settings as SettingsIcon,
  SquareTerminal,
  Sun,
  Terminal,
  Webhook,
} from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { LaunchButton } from "./Launcher";
import { Select } from "./Select";
import { useConnected } from "../lib/events";
import { useAuth } from "../lib/auth";
import { usePrefs } from "../lib/prefs";
import { useProject } from "../lib/project";
import { api } from "../lib/api";

const nav: { to: string; key: string; icon: LucideIcon; end?: boolean }[] = [
  { to: "/", key: "nav.dashboard", icon: LayoutDashboard, end: true },
  { to: "/projects", key: "nav.projects", icon: FolderGit2 },
  { to: "/repositories", key: "nav.repositories", icon: GitBranch },
  { to: "/templates", key: "nav.templates", icon: ListChecks },
  { to: "/workflows", key: "nav.workflows", icon: GitMerge },
  { to: "/schedules", key: "nav.schedules", icon: CalendarClock },
  { to: "/integrations", key: "nav.integrations", icon: Webhook },
  { to: "/environments", key: "nav.environments", icon: Boxes },
  { to: "/credentials", key: "nav.credentials", icon: KeyRound },
  { to: "/runners", key: "nav.runners", icon: Server },
  { to: "/hosts", key: "nav.hosts", icon: HardDrive },
  { to: "/cluster", key: "nav.cluster", icon: Network },
  { to: "/runs", key: "nav.runs", icon: SquareTerminal },
  { to: "/insights", key: "nav.insights", icon: BarChart3 },
  { to: "/audit", key: "nav.audit", icon: History },
];

export function Layout() {
  const connected = useConnected();
  const { user, setUser, docsEnabled } = useAuth();
  const { t, theme, toggleTheme, lang, setLang } = usePrefs();
  const { currentId, setCurrent } = useProject();
  const projects = useQuery({ queryKey: ["projects"], queryFn: api.listProjects });
  const navigate = useNavigate();

  const logout = async () => {
    try {
      await api.logout();
    } finally {
      setUser(null);
      navigate("/");
    }
  };

  return (
    <div className="flex h-screen overflow-hidden">
      <aside className="hidden w-60 shrink-0 select-none flex-col border-r border-border bg-surface md:flex">
        {/* Brand → Dashboard. select-none on the whole sidebar: chrome isn't meant
            to be drag-selected (page content stays selectable). */}
        <Link to="/" className="flex h-14 items-center gap-2.5 px-5 transition-opacity hover:opacity-90">
          <div className="grid h-8 w-8 place-items-center rounded-lg bg-accent/15 text-accent">
            <Terminal size={18} />
          </div>
          <div className="leading-tight">
            <div className="text-sm font-semibold tracking-tight text-ink">ansible·ui</div>
            <div className="text-[10px] uppercase tracking-wider text-ink-faint">{t("common.controlPlane")}</div>
          </div>
        </Link>

        <div className="px-3 pb-1 pt-1">
          <Select
            value={currentId}
            onChange={setCurrent}
            placeholder={t("runs.allProjects")}
            options={[
              { label: t("runs.allProjects"), value: "" },
              ...(projects.data ?? []).map((p) => ({ label: p.name, value: p.id })),
            ]}
          />
        </div>

        <nav className="flex-1 space-y-1 px-3 py-3">
          {nav.map((n) => (
            <NavLink
              key={n.to}
              to={n.to}
              end={n.end}
              className={({ isActive }) => clsx("nav-link", isActive && "nav-link-active")}
            >
              <n.icon size={17} />
              {t(n.key)}
            </NavLink>
          ))}
          {user?.role === "admin" && (
            <NavLink to="/applications" className={({ isActive }) => clsx("nav-link", isActive && "nav-link-active")}>
              <Blocks size={17} />
              {t("nav.applications")}
            </NavLink>
          )}
          {docsEnabled && (
            <NavLink to="/api" className={({ isActive }) => clsx("nav-link", isActive && "nav-link-active")}>
              <Braces size={17} />
              {t("nav.api")}
            </NavLink>
          )}
        </nav>

        <div className="space-y-1 border-t border-border px-3 py-2">
          <div className="flex items-center gap-1 px-1 pb-1">
            <button onClick={toggleTheme} className="btn-ghost flex-1 justify-center py-1.5" title={t("settings.theme")}>
              {theme === "dark" ? <Sun size={16} /> : <Moon size={16} />}
            </button>
            <div className="flex flex-1 overflow-hidden rounded-lg border border-border">
              {(["en", "ru"] as const).map((l) => (
                <button
                  key={l}
                  onClick={() => setLang(l)}
                  className={clsx(
                    "flex-1 py-1.5 text-xs font-semibold uppercase transition-colors",
                    lang === l ? "bg-accent/15 text-accent" : "text-ink-faint hover:text-ink",
                  )}
                >
                  {l}
                </button>
              ))}
            </div>
          </div>
          <NavLink to="/settings" className={({ isActive }) => clsx("nav-link", isActive && "nav-link-active")}>
            <SettingsIcon size={17} />
            {t("nav.settings")}
          </NavLink>
          <div className="flex items-center justify-between rounded-lg px-3 py-2">
            <div className="min-w-0">
              <div className="truncate text-sm text-ink">{user?.username}</div>
              <div className="flex items-center gap-1.5 text-[11px] text-ink-faint">
                <span className={clsx("h-1.5 w-1.5 rounded-full", connected ? "bg-success" : "bg-ink-faint")} />
                {connected ? t("common.live") : t("common.reconnecting")}
              </div>
            </div>
            <button onClick={logout} className="btn-ghost p-1.5" title={t("common.signOut")}>
              <LogOut size={16} />
            </button>
          </div>
        </div>
      </aside>

      <div className="flex min-w-0 flex-1 flex-col">
        {/* Compact top bar only on mobile (sidebar is hidden there). Desktop
            pages own their own header — no duplicate actions. */}
        <header className="flex h-14 shrink-0 select-none items-center justify-between border-b border-border bg-surface px-4 md:hidden">
          <Link to="/" className="flex items-center gap-2 text-sm font-semibold text-ink">
            <Terminal size={16} className="text-accent" />
            ansible·ui
          </Link>
          <LaunchButton />
        </header>
        <main className="min-h-0 flex-1 overflow-y-auto">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
