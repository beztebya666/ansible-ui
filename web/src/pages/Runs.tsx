import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { SquareTerminal, Trash2 } from "lucide-react";
import { Page, PageHeader } from "../components/PageHeader";
import { RunRow } from "../components/RunRow";
import { ConfirmDialog, EmptyState, Segmented, Spinner } from "../components/ui";
import { Select } from "../components/Select";
import { LaunchButton } from "../components/Launcher";
import { api } from "../lib/api";
import { useAuth } from "../lib/auth";
import { usePrefs } from "../lib/prefs";

export function Runs() {
  const { t } = usePrefs();
  const { user } = useAuth();
  const qc = useQueryClient();
  const [clearing, setClearing] = useState(false);
  const [clearBusy, setClearBusy] = useState(false);

  const clearAll = async () => {
    setClearBusy(true);
    try {
      await api.clearRuns();
      qc.invalidateQueries({ queryKey: ["runs"] });
      qc.invalidateQueries({ queryKey: ["stats"] });
      setClearing(false);
    } finally {
      setClearBusy(false);
    }
  };
  const filters = [
    { label: t("common.all"), value: "" },
    { label: t("status.running"), value: "running" },
    { label: t("status.success"), value: "success" },
    { label: t("status.failed"), value: "failed" },
  ];
  const [status, setStatus] = useState("");
  const [projectId, setProjectId] = useState("");
  const projects = useQuery({ queryKey: ["projects"], queryFn: api.listProjects });
  const runsAll = useQuery({
    queryKey: ["runs", { status, projectId }],
    queryFn: () => api.listRuns({ status: status || undefined, projectId: projectId || undefined, limit: 200 }),
    refetchInterval: 2000,
  });
  const [appFilter, setAppFilter] = useState("");
  const runs = {
    ...runsAll,
    data: appFilter ? runsAll.data?.filter((r) => (r.app || "ansible") === appFilter) : runsAll.data,
  };

  return (
    <>
      <PageHeader
        title={t("runs.title")}
        subtitle={t("runs.subtitle")}
        actions={
          <>
            {user?.role === "admin" && (
              <button className="btn-outline" onClick={() => setClearing(true)} title={t("runs.clearHistory")}>
                <Trash2 size={15} /> {t("runs.clearHistory")}
              </button>
            )}
            <LaunchButton />
          </>
        }
      />
      <Page>
        <div className="mb-4 flex flex-wrap items-center gap-3">
          <Segmented value={status} onChange={setStatus} options={filters} />
          <Select
            className="w-48"
            value={projectId}
            onChange={setProjectId}
            options={[
              { label: t("runs.allProjects"), value: "" },
              ...(projects.data ?? []).map((p) => ({ label: p.name, value: p.id })),
            ]}
          />
          <Select
            className="w-40"
            value={appFilter}
            onChange={setAppFilter}
            options={[
              { label: t("runs.allApps"), value: "" },
              { label: "Ansible", value: "ansible" },
              { label: "Terraform", value: "terraform" },
              { label: "OpenTofu", value: "tofu" },
              { label: "Terragrunt", value: "terragrunt" },
              { label: "Bash", value: "bash" },
              { label: "PowerShell", value: "powershell" },
              { label: "Python", value: "python" },
            ]}
          />
          {(status || projectId || appFilter) && (
            <button
              className="btn-ghost text-xs"
              onClick={() => { setStatus(""); setProjectId(""); setAppFilter(""); }}
            >
              {t("common.clearFilters")}
            </button>
          )}
        </div>

        <div className="card overflow-hidden">
          {runs.isLoading ? (
            <div className="flex items-center justify-center py-16 text-ink-faint">
              <Spinner />
            </div>
          ) : runs.data?.length ? (
            <div className="divide-y divide-border">
              {runs.data.map((r) => (
                <RunRow key={r.id} run={r} />
              ))}
            </div>
          ) : (
            <div className="p-6">
              <EmptyState
                icon={<SquareTerminal size={28} />}
                title={t("runs.none")}
                description={t("runs.noneHint")}
                action={<LaunchButton />}
              />
            </div>
          )}
        </div>
      </Page>

      <ConfirmDialog
        open={clearing}
        title={t("runs.clearTitle")}
        body={t("runs.clearBody")}
        confirmLabel={t("runs.clearHistory")}
        cancelLabel={t("common.cancel")}
        busy={clearBusy}
        onConfirm={clearAll}
        onClose={() => setClearing(false)}
      />
    </>
  );
}
