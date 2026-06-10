// Demo bootstrap: when VITE_DEMO is set we swap fetch + WebSocket for the in-browser
// mock, so the whole SPA runs with zero backend (great for GitHub Pages). Each
// browser is its own sandbox (localStorage); "Reset Demo" rebuilds it.
import { demoFetch } from "./server";
import { DemoWebSocket } from "./socket";
import { getDB } from "./db";

export { resetDemo } from "./db";

export function isDemo(): boolean {
  try {
    const env = (import.meta as unknown as { env?: Record<string, string> }).env;
    if (env && env.VITE_DEMO === "1") return true;
  } catch { /* ignore */ }
  return typeof window !== "undefined" && (window as unknown as { __AUI_DEMO__?: boolean }).__AUI_DEMO__ === true;
}

let installed = false;
export function installDemo() {
  if (installed) return;
  installed = true;
  (window as unknown as { __AUI_DEMO__?: boolean }).__AUI_DEMO__ = true;
  window.fetch = ((input: RequestInfo | URL, init?: RequestInit) => demoFetch(input, init)) as typeof window.fetch;
  (window as unknown as { WebSocket: unknown }).WebSocket = DemoWebSocket;
  try {
    if (!localStorage.getItem("aui.project")) localStorage.setItem("aui.project", "prj_prod");
  } catch { /* ignore */ }
  getDB(); // seed on first visit
}
