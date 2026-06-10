import { createContext, useContext, useEffect, useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { wsURL } from "./api";

const ConnCtx = createContext(false);

/** useConnected reports whether the live event socket is currently open. */
export function useConnected() {
  return useContext(ConnCtx);
}

/**
 * EventsProvider keeps a single /ws/events socket open and invalidates the
 * relevant React Query caches whenever a run or project changes, so every
 * list and the dashboard update live with zero manual refetching.
 */
export function EventsProvider({ children }: { children: React.ReactNode }) {
  const qc = useQueryClient();
  const [connected, setConnected] = useState(false);
  const retry = useRef(0);

  useEffect(() => {
    let ws: WebSocket | null = null;
    let timer: number | undefined;
    let closed = false;

    const connect = () => {
      ws = new WebSocket(wsURL("/ws/events"));
      ws.onopen = () => {
        retry.current = 0;
        setConnected(true);
      };
      ws.onmessage = (ev) => {
        let msg: { type?: string } = {};
        try {
          msg = JSON.parse(ev.data);
        } catch {
          return;
        }
        if (msg.type === "run.updated") {
          // Lists + the open run-detail page + dashboard, all live (no F5).
          qc.invalidateQueries({ queryKey: ["runs"] });
          qc.invalidateQueries({ queryKey: ["run"] });
          // The structured log stops polling once a run finishes; the final output
          // is flushed on finish (manager.finish persists it *before* publishing
          // this event), so refetch it here or it stays empty until a manual F5.
          qc.invalidateQueries({ queryKey: ["runlog"] });
          qc.invalidateQueries({ queryKey: ["artifacts"] }); // captured on finish
          qc.invalidateQueries({ queryKey: ["stats"] });
          qc.invalidateQueries({ queryKey: ["activity"] });
        } else if (msg.type === "activity") {
          qc.invalidateQueries({ queryKey: ["activity"] });
          qc.invalidateQueries({ queryKey: ["stats"] });
        } else if (msg.type === "project.created") {
          qc.invalidateQueries({ queryKey: ["projects"] });
          qc.invalidateQueries({ queryKey: ["stats"] });
        } else if (msg.type === "repository.updated") {
          qc.invalidateQueries({ queryKey: ["repositories"] });
        } else if (msg.type === "runners") {
          qc.invalidateQueries({ queryKey: ["runners"] });
        } else if (msg.type === "branches") {
          qc.invalidateQueries({ queryKey: ["branches"] });
        } else if (msg.type === "workflow") {
          qc.invalidateQueries({ queryKey: ["workflowRuns"] });
          qc.invalidateQueries({ queryKey: ["workflows"] });
        }
      };
      ws.onclose = () => {
        setConnected(false);
        if (closed) return;
        const delay = Math.min(1000 * 2 ** retry.current, 10000);
        retry.current += 1;
        timer = window.setTimeout(connect, delay);
      };
      ws.onerror = () => ws?.close();
    };
    connect();

    return () => {
      closed = true;
      if (timer) window.clearTimeout(timer);
      ws?.close();
    };
  }, [qc]);

  return <ConnCtx.Provider value={connected}>{children}</ConnCtx.Provider>;
}
