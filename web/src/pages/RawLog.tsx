import { useEffect, useState } from "react";
import { useParams } from "react-router-dom";
import { api } from "../lib/api";
import { Spinner } from "../components/ui";

// Strip ANSI escape / OSC sequences so the raw log is clean, plain text.
function stripAnsi(s: string): string {
  return s
    // eslint-disable-next-line no-control-regex
    .replace(/\x1b\[[0-9;?]*[ -/]*[@-~]/g, "")
    // eslint-disable-next-line no-control-regex
    .replace(/\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)/g, "")
    // eslint-disable-next-line no-control-regex
    .replace(/[\x00-\x08\x0b\x0c\x0e-\x1f]/g, "");
}

/** A bare, copy-friendly plain-text view of a run's full log (opened in a new
 *  tab from the console's "Raw log" button) — like Semaphore's /raw_output. */
export function RawLog() {
  const { id = "" } = useParams();
  const [text, setText] = useState<string | null>(null);

  useEffect(() => {
    let alive = true;
    document.title = `raw log · ${id.replace(/^run_/, "")}`;
    fetch(api.runOutputURL(id), { credentials: "same-origin" })
      .then((r) => r.text())
      .then((t) => alive && setText(stripAnsi(t)))
      .catch(() => alive && setText("Failed to load log."));
    return () => {
      alive = false;
    };
  }, [id]);

  if (text === null) {
    return (
      <div className="flex h-screen items-center justify-center bg-bg text-ink-faint">
        <Spinner />
      </div>
    );
  }
  return (
    <pre className="min-h-screen whitespace-pre-wrap break-words bg-bg p-4 font-mono text-xs leading-relaxed text-ink-dim">
      {text || "— empty —"}
    </pre>
  );
}
