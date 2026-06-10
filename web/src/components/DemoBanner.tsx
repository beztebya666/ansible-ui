import { useState } from "react";
import { resetDemo } from "../lib/demo";

/**
 * Floating demo control (demo build only). Tells the visitor this is a private,
 * in-browser sandbox and lets them reset it to the seeded state at any time.
 */
export function DemoBanner() {
  const [busy, setBusy] = useState(false);
  const reset = () => {
    setBusy(true);
    resetDemo();
    setTimeout(() => location.reload(), 150);
  };
  return (
    <div className="fixed bottom-4 right-4 z-[60] flex items-center gap-3 rounded-xl border border-zinc-700/70 bg-zinc-900/95 px-4 py-2.5 text-sm text-zinc-100 shadow-2xl ring-1 ring-black/30">
      <span className="flex items-center gap-2">
        <span className="relative flex h-2.5 w-2.5">
          <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-60" />
          <span className="relative inline-flex h-2.5 w-2.5 rounded-full bg-emerald-400" />
        </span>
        <strong className="font-semibold">Live demo</strong>
        <span className="hidden text-zinc-400 sm:inline">· your private in-browser sandbox</span>
      </span>
      <button
        onClick={reset}
        disabled={busy}
        title="Wipe your changes and restore the seeded demo data"
        className="rounded-lg bg-emerald-500 px-3 py-1.5 font-medium text-zinc-950 transition hover:bg-emerald-400 disabled:opacity-60"
      >
        {busy ? "Resetting…" : "Reset demo"}
      </button>
      <a
        href="https://github.com/beztebya666/ansible-ui"
        target="_blank"
        rel="noreferrer"
        className="text-zinc-400 underline-offset-2 hover:text-zinc-100 hover:underline"
      >
        GitHub ↗
      </a>
    </div>
  );
}
