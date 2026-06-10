// A drop-in WebSocket replacement for the demo build. For /ws/runs/{id} it reveals
// the run's log as binary PTY frames (so the terminal animates like the real live
// stream), then a {type:"end"} meta frame. For /ws/events it forwards demo events
// so lists + the dashboard refresh live with no F5.
import { getDB, onDemoEvent } from "./db";

type Cb = ((ev: { data: unknown }) => void) | null;

function chunkLines(s: string, per = 2): string[] {
  const lines = s.split(/(?<=\n)/); // keep the trailing newline on each line
  const out: string[] = [];
  for (let i = 0; i < lines.length; i += per) out.push(lines.slice(i, i + per).join(""));
  return out.filter((c) => c.length);
}

export class DemoWebSocket {
  static readonly CONNECTING = 0;
  static readonly OPEN = 1;
  static readonly CLOSING = 2;
  static readonly CLOSED = 3;
  readonly CONNECTING = 0;
  readonly OPEN = 1;
  readonly CLOSING = 2;
  readonly CLOSED = 3;

  url: string;
  readyState = 0;
  binaryType: "arraybuffer" | "blob" = "blob";
  onopen: Cb = null;
  onmessage: Cb = null;
  onclose: Cb = null;
  onerror: Cb = null;

  private timers: number[] = [];
  private unsub?: () => void;
  private closed = false;

  constructor(url: string) {
    this.url = url;
    setTimeout(() => this.start(), 0); // let the caller attach handlers first
  }

  private emit(data: unknown) {
    if (!this.closed) this.onmessage?.({ data });
  }

  private start() {
    if (this.closed) return;
    this.readyState = this.OPEN;
    this.onopen?.({ data: null });
    const m = /\/ws\/runs\/([^/?]+)/.exec(this.url);
    if (m) {
      this.streamRun(decodeURIComponent(m[1]));
    } else if (this.url.includes("/ws/events")) {
      this.unsub = onDemoEvent((type) => this.emit(JSON.stringify({ type })));
    }
  }

  private streamRun(runId: string) {
    const log = getDB().runLogs[runId] || "";
    if (!log) {
      this.timers.push(window.setTimeout(() => this.emit(JSON.stringify({ type: "end" })), 200));
      return;
    }
    const enc = new TextEncoder();
    let delay = 150;
    for (const part of chunkLines(log, 2)) {
      const id = window.setTimeout(() => this.emit(enc.encode(part).buffer), delay);
      this.timers.push(id);
      delay += 120 + Math.random() * 130;
    }
    this.timers.push(window.setTimeout(() => this.emit(JSON.stringify({ type: "end" })), delay + 250));
  }

  send() { /* the demo never sends */ }
  addEventListener() { /* handlers are set via on* props */ }
  removeEventListener() { /* no-op */ }

  close() {
    if (this.closed) return;
    this.closed = true;
    this.readyState = this.CLOSED;
    for (const t of this.timers) clearTimeout(t);
    this.unsub?.();
    this.onclose?.({ data: null });
  }
}
