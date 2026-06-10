import { forwardRef, useEffect, useImperativeHandle, useRef } from "react";
import { Terminal as XTerm, type ITheme } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import { WebLinksAddon } from "@xterm/addon-web-links";
import { WebglAddon } from "@xterm/addon-webgl";
import "@xterm/xterm/css/xterm.css";
import { xtermTheme } from "../lib/ansi";

// The 16 ANSI colours come from the single shared palette (lib/ansi) so the live
// terminal and the structured log view never drift apart.
const theme: ITheme = {
  background: "#0a0c10",
  foreground: "#c8d0dc",
  cursor: "#34d399",
  cursorAccent: "#0a0c10",
  selectionBackground: "rgba(52,211,153,0.25)",
  ...xtermTheme(),
};

export interface TermHandle {
  write(data: Uint8Array | string): void;
  clear(): void;
  fit(): void;
  focus(): void;
  scrollToBottom(): void;
  size(): { cols: number; rows: number };
}

interface Props {
  onResize?: (cols: number, rows: number) => void;
}

export const TerminalView = forwardRef<TermHandle, Props>(({ onResize }, ref) => {
  const elRef = useRef<HTMLDivElement>(null);
  const termRef = useRef<XTerm | null>(null);
  const fitRef = useRef<FitAddon | null>(null);
  const resizeCb = useRef(onResize);
  resizeCb.current = onResize;

  useEffect(() => {
    if (!elRef.current) return;
    const term = new XTerm({
      theme,
      fontFamily:
        "'JetBrains Mono', ui-monospace, SFMono-Regular, Menlo, Consolas, monospace",
      fontSize: 13,
      lineHeight: 1.25,
      letterSpacing: 0,
      cursorBlink: false,
      cursorStyle: "bar",
      scrollback: 20000,
      convertEol: false,
      allowProposedApi: true,
      smoothScrollDuration: 0,
    });
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.loadAddon(new WebLinksAddon());
    term.open(elRef.current);

    // Ctrl+C / Cmd+C copies the selection (instead of sending SIGINT) when text
    // is selected; otherwise the key passes through to the PTY. Cmd/Ctrl+V is
    // left to the browser so paste works normally.
    term.attachCustomKeyEventHandler((e) => {
      if (e.type !== "keydown") return true;
      const mod = e.ctrlKey || e.metaKey;
      if (mod && (e.key === "c" || e.key === "C")) {
        const sel = term.getSelection();
        if (sel && sel.length > 0) {
          navigator.clipboard?.writeText(sel).catch(() => {});
          return false; // handled — don't forward ^C
        }
      }
      return true;
    });
    try {
      const webgl = new WebglAddon();
      webgl.onContextLoss(() => webgl.dispose());
      term.loadAddon(webgl);
    } catch {
      /* webgl unavailable — fall back to the default renderer */
    }
    try {
      fit.fit();
    } catch {
      /* element not measured yet */
    }
    termRef.current = term;
    fitRef.current = fit;
    resizeCb.current?.(term.cols, term.rows);

    const ro = new ResizeObserver(() => {
      try {
        fit.fit();
        resizeCb.current?.(term.cols, term.rows);
      } catch {
        /* ignore transient measure errors */
      }
    });
    ro.observe(elRef.current);

    return () => {
      ro.disconnect();
      term.dispose();
      termRef.current = null;
      fitRef.current = null;
    };
  }, []);

  useImperativeHandle(ref, () => ({
    write: (d) => termRef.current?.write(d),
    clear: () => termRef.current?.clear(),
    fit: () => {
      try {
        fitRef.current?.fit();
      } catch {
        /* ignore */
      }
    },
    focus: () => termRef.current?.focus(),
    scrollToBottom: () => termRef.current?.scrollToBottom(),
    size: () => ({ cols: termRef.current?.cols ?? 80, rows: termRef.current?.rows ?? 24 }),
  }));

  return <div ref={elRef} className="xterm-host" />;
});

TerminalView.displayName = "TerminalView";
