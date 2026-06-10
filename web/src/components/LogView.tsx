import { useEffect, useMemo, useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { ansiToSpans, bannerHead } from "../lib/ansi";
import { api, wsURL } from "../lib/api";
import { usePrefs } from "../lib/prefs";
import { Spinner } from "./ui";

function clockMs(ms: number, hour12: boolean): string {
  return new Date(ms).toLocaleTimeString(undefined, {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12,
  });
}

// A generous run of '*' for banner fills; the row clips whatever doesn't fit, so
// every banner reaches the right edge on a single line (Semaphore-style).
const STARS = "*".repeat(260);

/**
 * The run console: a Semaphore-style structured log. Per-line timestamp gutter,
 * ANSI colours from the single shared palette, banners that fill the width and
 * clip (one line, no wrap, no horizontal scroll), and long real lines that wrap.
 *
 * While the run is live it streams the PTY over /ws/runs/{id} and renders line by
 * line as bytes arrive — smooth and immediate, not a chunky poll. Finished runs
 * (and the reconcile after a stream ends) replay the stored output + server-side
 * per-line timestamps. A slow stored-log poll is kept only as a socket-down
 * fallback so the log is never stuck (the live-everywhere bar).
 */
export function LogView({ runId, active }: { runId: string; active: boolean }) {
  const { clock, t } = usePrefs();
  const hour12 = clock === "12h";
  const qc = useQueryClient();
  const q = useQuery({
    queryKey: ["runlog", runId],
    queryFn: () => api.runLog(runId),
    refetchInterval: active ? 2000 : false, // fallback only; the WS does the live work
  });

  // Live PTY stream. We accumulate decoded bytes into `live` and stamp each
  // completed (\n-terminated) line with its arrival time for the gutter, mirroring
  // how the backend stamps stored lines. Kept after the run ends so the finished
  // log shows instantly with no flicker; the stored-log query reconciles in the
  // background (and serves runs we never streamed).
  const [live, setLive] = useState<{ text: string; times: number[] } | null>(null);
  useEffect(() => void setLive(null), [runId]); // drop a previous run's buffer

  useEffect(() => {
    if (!active) return; // only stream while the run is live
    setLive((p) => p ?? { text: "", times: [] });
    let buf = "";
    const times: number[] = [];
    const decoder = new TextDecoder();
    let raf = 0;
    const paint = () => {
      raf = 0;
      setLive({ text: buf, times: times.slice() });
    };
    let ws: WebSocket | null = null;
    try {
      ws = new WebSocket(wsURL(`/ws/runs/${runId}`));
      ws.binaryType = "arraybuffer";
      ws.onmessage = (ev) => {
        if (typeof ev.data === "string") {
          // Lifecycle meta: when the stream ends, let the stored-log query
          // reconcile (authoritative final bytes + server timestamps).
          try {
            if ((JSON.parse(ev.data) as { type?: string }).type === "end")
              qc.invalidateQueries({ queryKey: ["runlog", runId] });
          } catch { /* ignore non-JSON */ }
          return;
        }
        const chunk = decoder.decode(new Uint8Array(ev.data as ArrayBuffer), { stream: true });
        const now = Date.now();
        for (let k = 0; k < chunk.length; k++) if (chunk.charCodeAt(k) === 10) times.push(now);
        buf += chunk;
        if (!raf) raf = requestAnimationFrame(paint); // coalesce bursts to one paint/frame
      };
    } catch {
      /* WebSocket unavailable — the slow stored-log poll keeps the log updating */
    }
    return () => {
      if (raf) cancelAnimationFrame(raf);
      ws?.close();
    };
  }, [active, runId, qc]);

  // Prefer the live buffer whenever it carries output; fall back to the stored log
  // (never-streamed finished runs, or a socket that failed before any bytes).
  const useLive = !!live && live.text.length > 0;
  const output = useLive ? live!.text : q.data?.output ?? "";
  const times = useLive ? live!.times : q.data?.times ?? [];

  const lines = useMemo(() => {
    const ls = output.split("\n"); // keep \r in-line — the emulator needs it
    if (ls.length && ls[ls.length - 1] === "") ls.pop();
    return ls;
  }, [output]);

  // Auto-scroll to the bottom on new output while the run is live, unless the
  // user has scrolled up to read something.
  const scrollRef = useRef<HTMLDivElement>(null);
  const atBottom = useRef(true);
  const onScroll = () => {
    const el = scrollRef.current;
    if (el) atBottom.current = el.scrollHeight - el.scrollTop - el.clientHeight < 48;
  };
  useEffect(() => {
    const el = scrollRef.current;
    if (el && active && atBottom.current) el.scrollTop = el.scrollHeight;
  }, [lines, active]);

  if (q.isLoading && !useLive) {
    return (
      <div className="flex h-full items-center justify-center bg-bg text-ink-faint">
        <Spinner />
      </div>
    );
  }

  if (!lines.length) {
    return (
      <div className="flex h-full items-center justify-center bg-bg text-sm text-ink-faint">
        {active ? t("run.waitingOutput") : t("run.noOutput")}
      </div>
    );
  }

  return (
    <div
      ref={scrollRef}
      onScroll={onScroll}
      className="h-full overflow-y-auto overflow-x-hidden bg-bg font-mono text-xs leading-normal"
    >
      {lines.map((raw, i) => {
        const head = bannerHead(raw);
        return (
          <div key={i} className="flex items-start hover:bg-white/[0.03]">
            <span className="sticky left-0 z-10 flex-none select-none whitespace-nowrap border-r border-white/[0.04] bg-bg px-3 py-px text-right tabular-nums text-ink-dim">
              {times[i] ? clockMs(times[i], hour12) : " "}
            </span>
            {head !== null ? (
              <div className="flex min-w-0 flex-1 items-baseline py-px pl-3 pr-4 text-ink">
                <span className="whitespace-pre">{ansiToSpans(head, String(i))} </span>
                <span className="flex-1 overflow-hidden whitespace-nowrap text-ink-dim/60" aria-hidden>
                  {STARS}
                </span>
              </div>
            ) : (
              <div className="min-w-0 flex-1 whitespace-pre-wrap break-words py-px pl-3 pr-4 text-ink">
                {ansiToSpans(raw, String(i))}
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}
