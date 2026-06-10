import { BrowserRouter, Route, Routes } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { Layout } from "./components/Layout";
import { FeedbackProvider } from "./components/feedback";
import { LauncherProvider } from "./components/Launcher";
import { EventsProvider } from "./lib/events";
import { AuthProvider, useAuth } from "./lib/auth";
import { PrefsProvider } from "./lib/prefs";
import { ProjectProvider } from "./lib/project";
import { Spinner } from "./components/ui";
import { Login } from "./pages/Login";
import { Dashboard } from "./pages/Dashboard";
import { Projects } from "./pages/Projects";
import { ProjectDetail } from "./pages/ProjectDetail";
import { Templates } from "./pages/Templates";
import { Repositories } from "./pages/Repositories";
import { Environments } from "./pages/Environments";
import { Runners } from "./pages/Runners";
import { Schedules } from "./pages/Schedules";
import { Integrations } from "./pages/Integrations";
import { Credentials } from "./pages/Credentials";
import { Settings } from "./pages/Settings";
import { Runs } from "./pages/Runs";
import { RunDetail } from "./pages/RunDetail";
import { RawLog } from "./pages/RawLog";
import { ApiExplorer } from "./pages/ApiExplorer";
import { Applications } from "./pages/Applications";
import { Audit } from "./pages/Audit";
import { Insights } from "./pages/Insights";
import { Workflows } from "./pages/Workflows";
import { Cluster } from "./pages/Cluster";
import { Hosts } from "./pages/Hosts";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: { retry: 1, refetchOnWindowFocus: false, staleTime: 5_000 },
  },
});

function Gate() {
  const { user, loading } = useAuth();

  if (loading) {
    return (
      <div className="flex h-screen items-center justify-center bg-bg text-ink-faint">
        <Spinner className="h-6 w-6" />
      </div>
    );
  }
  if (!user) return <Login />;

  return (
    <EventsProvider>
      <LauncherProvider>
        <Routes>
          {/* Bare full-page raw log (opened in a new tab) — no app chrome. */}
          <Route path="runs/:id/raw" element={<RawLog />} />
          <Route element={<Layout />}>
            <Route index element={<Dashboard />} />
            <Route path="projects" element={<Projects />} />
            <Route path="projects/:id" element={<ProjectDetail />} />
            <Route path="repositories" element={<Repositories />} />
            <Route path="templates" element={<Templates />} />
            <Route path="workflows" element={<Workflows />} />
            <Route path="cluster" element={<Cluster />} />
            <Route path="environments" element={<Environments />} />
            <Route path="schedules" element={<Schedules />} />
            <Route path="integrations" element={<Integrations />} />
            <Route path="credentials" element={<Credentials />} />
            <Route path="runners" element={<Runners />} />
            <Route path="hosts" element={<Hosts />} />
            <Route path="runs" element={<Runs />} />
            <Route path="runs/:id" element={<RunDetail />} />
            <Route path="api" element={<ApiExplorer />} />
            <Route path="applications" element={<Applications />} />
            <Route path="audit" element={<Audit />} />
            <Route path="insights" element={<Insights />} />
            <Route path="settings" element={<Settings />} />
          </Route>
        </Routes>
      </LauncherProvider>
    </EventsProvider>
  );
}

export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <ProjectProvider>
        <PrefsProvider>
          <FeedbackProvider>
            <BrowserRouter>
              <AuthProvider>
                <Gate />
              </AuthProvider>
            </BrowserRouter>
          </FeedbackProvider>
        </PrefsProvider>
      </ProjectProvider>
    </QueryClientProvider>
  );
}
