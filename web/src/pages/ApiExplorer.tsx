import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import clsx from "clsx";
import { Lock, Globe, Play, RotateCw, Shield } from "lucide-react";
import { Page, PageHeader } from "../components/PageHeader";
import { Spinner, EmptyState } from "../components/ui";
import { CodeEditor } from "../components/CodeEditor";
import { api, type ApiModelDoc, type EndpointDoc } from "../lib/api";
import { usePrefs } from "../lib/prefs";

const methodColor: Record<string, string> = {
  GET: "text-info border-info/40 bg-info/10",
  POST: "text-success border-success/40 bg-success/10",
  PUT: "text-warn border-warn/40 bg-warn/10",
  DELETE: "text-danger border-danger/40 bg-danger/10",
};

function pretty(s: string): string {
  if (!s) return "";
  try {
    return JSON.stringify(JSON.parse(s), null, 2);
  } catch {
    return s; // examples with // comments etc. — leave as-is
  }
}

export function ApiExplorer() {
  const { t } = usePrefs();
  const docs = useQuery({ queryKey: ["apidocs"], queryFn: api.apiDocs });
  const [selected, setSelected] = useState<EndpointDoc | null>(null);
  const [view, setView] = useState<"endpoints" | "models">("endpoints");

  const groups = useMemo(() => {
    const m = new Map<string, EndpointDoc[]>();
    for (const e of docs.data?.endpoints ?? []) {
      if (!m.has(e.group)) m.set(e.group, []);
      m.get(e.group)!.push(e);
    }
    return Array.from(m.entries());
  }, [docs.data]);

  const active = selected ?? docs.data?.endpoints?.[0] ?? null;

  return (
    <>
      <PageHeader title={t("set.docsTitle")} subtitle={t("api.subtitle")} />
      <Page>
        {docs.isLoading ? (
          <div className="flex justify-center py-20 text-ink-faint"><Spinner /></div>
        ) : docs.isError ? (
          <EmptyState icon={<Lock size={28} />} title={t("api.disabled")} description={t("api.disabledDesc")} />
        ) : (
          <>
          <div className="mb-4 flex gap-1 border-b border-border">
            {(["endpoints", "models"] as const).map((k) => (
              <button
                key={k}
                onClick={() => setView(k)}
                className={clsx("-mb-px border-b-2 px-3 py-2 text-sm", view === k ? "border-accent text-ink" : "border-transparent text-ink-faint hover:text-ink-dim")}
              >
                {t("api.tab" + (k === "endpoints" ? "Endpoints" : "Models"))}
                <span className="ml-1.5 text-xs text-ink-faint">{k === "endpoints" ? docs.data?.endpoints?.length : docs.data?.models?.length ?? 0}</span>
              </button>
            ))}
          </div>
          {view === "models" ? (
            <ModelsView models={docs.data?.models ?? []} />
          ) : (
          <div className="grid gap-4 lg:grid-cols-[320px_1fr]">
            {/* endpoint list */}
            <div className="card max-h-[74vh] overflow-y-auto p-2">
              {groups.map(([group, eps]) => (
                <div key={group} className="mb-2">
                  <div className="px-2 py-1 text-[11px] font-semibold uppercase tracking-wide text-ink-faint">{group}</div>
                  {eps.map((e) => (
                    <button
                      key={e.method + e.path}
                      onClick={() => setSelected(e)}
                      className={clsx(
                        "flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-xs transition-colors",
                        active === e ? "bg-accent/10" : "hover:bg-white/5",
                      )}
                    >
                      <span className={clsx("w-12 shrink-0 rounded border px-1 py-0.5 text-center text-[10px] font-bold", methodColor[e.method] ?? "text-ink-dim border-border")}>
                        {e.method}
                      </span>
                      <span className="min-w-0 flex-1 truncate font-mono text-ink-dim">{e.path}</span>
                      {e.auth ? (
                        <span className="shrink-0" title={t("api.authNote")}><Lock size={11} className="text-ink-faint" /></span>
                      ) : (
                        <span className="shrink-0" title={t("api.noAuth")}><Globe size={11} className="text-success" /></span>
                      )}
                    </button>
                  ))}
                </div>
              ))}
            </div>

            {/* detail (key resets the editable request when switching endpoints) */}
            <div>{active && <Detail key={active.method + active.path} ep={active} />}</div>
          </div>
          )}
          </>
        )}
      </Page>
    </>
  );
}

function Detail({ ep }: { ep: EndpointDoc }) {
  const { t } = usePrefs();
  const hasBody = ep.method === "POST" || ep.method === "PUT";
  const original = pretty(ep.request ?? "");
  const [body, setBody] = useState(original);
  const [vals, setVals] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<{ status: number; text: string } | null>(null);

  const pathParams = (ep.params ?? []).filter((p) => p.in === "path");
  const queryParams = (ep.params ?? []).filter((p) => p.in === "query");

  const send = async () => {
    let path = ep.path;
    for (const p of pathParams) path = path.replace(`{${p.name}}`, encodeURIComponent(vals[p.name] || ""));
    const qs = queryParams.filter((p) => vals[p.name]).map((p) => `${p.name}=${encodeURIComponent(vals[p.name])}`).join("&");
    if (qs) path += (path.includes("?") ? "&" : "?") + qs;

    setBusy(true);
    setResult(null);
    try {
      const proj = localStorage.getItem("aui.project") || "";
      const res = await fetch(path, {
        method: ep.method,
        credentials: "same-origin",
        headers: { ...(hasBody ? { "Content-Type": "application/json" } : {}), ...(proj ? { "X-Project-Id": proj } : {}) },
        body: hasBody && body ? body : undefined,
      });
      const text = await res.text();
      setResult({ status: res.status, text: pretty(text) });
    } catch (e) {
      setResult({ status: 0, text: String(e) });
    } finally {
      setBusy(false);
    }
  };

  const responseText = result ? result.text : pretty(ep.response ?? "");

  return (
    <div className="card space-y-3 p-4">
      {/* header: method · path · auth badge · refresh · Send (top-right) */}
      <div className="flex flex-wrap items-center gap-2">
        <span className={clsx("rounded border px-2 py-0.5 text-xs font-bold", methodColor[ep.method] ?? "text-ink-dim border-border")}>{ep.method}</span>
        <code className="font-mono text-sm text-ink">{ep.path}</code>
        {ep.auth ? (
          <span className="chip text-[11px] text-ink-dim" title={t("api.authNote")}><Lock size={11} /> {t("api.authBadge")}</span>
        ) : (
          <span className="chip text-[11px] text-success" title={t("api.noAuth")}><Globe size={11} /> {t("api.public")}</span>
        )}
        {ep.admin && <span className="chip text-[11px] text-warn"><Shield size={11} /> {t("api.adminBadge")}</span>}
        <div className="ml-auto flex items-center gap-1.5">
          <button className="btn-ghost p-1.5" title={t("api.reset")} onClick={() => { setBody(original); setResult(null); }}>
            <RotateCw size={15} />
          </button>
          <button className="btn-primary px-3 py-1.5" onClick={send} disabled={busy}>
            {busy ? <Spinner /> : <Play size={14} />} {t("api.send")}
          </button>
        </div>
      </div>

      <p className="text-sm text-ink-dim">{ep.summary}</p>

      {(pathParams.length > 0 || queryParams.length > 0) && (
        <div className="grid gap-2 sm:grid-cols-2">
          {[...pathParams, ...queryParams].map((p) => (
            <label key={p.name} className="block">
              <span className="label">{p.name} <span className="text-ink-faint">({p.in})</span></span>
              <input className="input font-mono text-xs" value={vals[p.name] ?? ""} onChange={(e) => setVals((v) => ({ ...v, [p.name]: e.target.value }))} placeholder={p.desc} />
            </label>
          ))}
        </div>
      )}

      <div>
        <div className="label">{t("api.request")} {!hasBody && <span className="text-ink-faint">— {ep.method} {t("api.noBodyNote")}</span>}</div>
        <div className="overflow-hidden rounded-lg border border-border bg-surface">
          <CodeEditor
            value={body}
            language="json"
            onChange={setBody}
            readOnly={!hasBody}
            placeholder={hasBody ? "{}" : t("api.noReqBody")}
            className="max-h-72 min-h-[80px]"
          />
        </div>
      </div>

      {result && (
        <div>
          <div className="mb-1 flex items-center gap-2">
            <span className="label mb-0">{t("api.response")}</span>
            {result && (
              <span className={clsx("rounded px-1.5 py-0.5 font-mono text-xs font-bold", result.status >= 200 && result.status < 300 ? "bg-success/10 text-success" : "bg-danger/10 text-danger")}>
                {result.status || "ERR"}
              </span>
            )}
          </div>
          <div className="overflow-hidden rounded-lg border border-border bg-surface">
            <CodeEditor value={responseText} language="json" readOnly className="max-h-80 min-h-[80px]" />
          </div>
        </div>
      )}
    </div>
  );
}

function ModelsView({ models }: { models: ApiModelDoc[] }) {
  return (
    <div className="grid gap-4 lg:grid-cols-2">
      {models.map((m) => (
        <div key={m.name} className="card p-4">
          <div className="font-mono text-sm font-semibold text-ink">{m.name}</div>
          <p className="mb-3 mt-0.5 text-xs text-ink-faint">{m.description}</p>
          <div className="overflow-hidden rounded-lg border border-border">
            <table className="w-full text-xs">
              <tbody className="divide-y divide-border">
                {m.fields.map((f) => (
                  <tr key={f.name} className="hover:bg-surface/50">
                    <td className="px-3 py-1.5 font-mono text-ink-dim">{f.name}</td>
                    <td className="px-3 py-1.5 text-right font-mono text-accent">{f.type}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      ))}
    </div>
  );
}
