import type { CSSProperties, ReactNode } from "react";

/**
 * SINGLE SOURCE OF TRUTH for terminal colours.
 *
 * `ANSI16` is the 16-colour base palette (0–7 normal, 8–15 bright), used by BOTH
 * the live xterm terminal (see Terminal.tsx, via `xtermTheme()`) and the
 * structured log view (via `ansiToSpans`). Change a colour here → it changes
 * everywhere. No per-place / per-case duplication.
 */
export const ANSI16 = [
  "#0a0c10", // 0 black
  "#f87171", // 1 red
  "#34d399", // 2 green
  "#fbbf24", // 3 yellow
  "#7aa2f7", // 4 blue
  "#c084fc", // 5 magenta
  "#22d3ee", // 6 cyan
  "#c8d0dc", // 7 white
  "#5b6677", // 8 bright black
  "#fca5a5", // 9 bright red
  "#6ee7b7", // 10 bright green
  "#fcd34d", // 11 bright yellow
  "#93c5fd", // 12 bright blue
  "#d8b4fe", // 13 bright magenta
  "#67e8f9", // 14 bright cyan
  "#e6e9ef", // 15 bright white
] as const;

const DEFAULT_FG = "#c8d0dc"; // ink-dim — terminal foreground
const DEFAULT_BG = "#0a0c10"; // terminal background

/** The 16 named colours an xterm ITheme needs, derived from the single palette. */
export function xtermTheme() {
  return {
    black: ANSI16[0], red: ANSI16[1], green: ANSI16[2], yellow: ANSI16[3],
    blue: ANSI16[4], magenta: ANSI16[5], cyan: ANSI16[6], white: ANSI16[7],
    brightBlack: ANSI16[8], brightRed: ANSI16[9], brightGreen: ANSI16[10], brightYellow: ANSI16[11],
    brightBlue: ANSI16[12], brightMagenta: ANSI16[13], brightCyan: ANSI16[14], brightWhite: ANSI16[15],
  };
}

// Resolve an xterm 256-colour index → CSS colour. 0–15 use the base palette;
// 16–231 are the 6×6×6 cube; 232–255 are the grayscale ramp (computed, not a
// per-value table).
function color256(n: number): string {
  if (n < 16) return ANSI16[n];
  if (n >= 232) {
    const v = 8 + (n - 232) * 10;
    return `rgb(${v},${v},${v})`;
  }
  const i = n - 16;
  const ch = (x: number) => (x === 0 ? 0 : 55 + x * 40);
  return `rgb(${ch(Math.floor(i / 36))},${ch(Math.floor((i % 36) / 6))},${ch(i % 6)})`;
}

interface Style {
  fg?: string;
  bg?: string;
  bold?: boolean;
  dim?: boolean;
  italic?: boolean;
  underline?: boolean;
  strike?: boolean;
  reverse?: boolean;
  hidden?: boolean;
}

// Apply one SGR (Select Graphic Rendition) escape's codes to the running style.
// Covers the full common spec so any playbook's output renders, not just ours.
function applySGR(s: Style, codes: number[]): Style {
  const out = { ...s };
  for (let i = 0; i < codes.length; i++) {
    const c = codes[i];
    if (c === 0) { out.fg = out.bg = undefined; out.bold = out.dim = out.italic = out.underline = out.strike = out.reverse = out.hidden = false; }
    else if (c === 1) out.bold = true;
    else if (c === 2) out.dim = true;
    else if (c === 3) out.italic = true;
    else if (c === 4) out.underline = true;
    else if (c === 7) out.reverse = true;
    else if (c === 8) out.hidden = true;
    else if (c === 9) out.strike = true;
    else if (c === 21 || c === 24) out.underline = false;
    else if (c === 22) { out.bold = false; out.dim = false; }
    else if (c === 23) out.italic = false;
    else if (c === 27) out.reverse = false;
    else if (c === 28) out.hidden = false;
    else if (c === 29) out.strike = false;
    else if (c >= 30 && c <= 37) out.fg = ANSI16[c - 30];
    else if (c === 39) out.fg = undefined;
    else if (c >= 40 && c <= 47) out.bg = ANSI16[c - 40];
    else if (c === 49) out.bg = undefined;
    else if (c >= 90 && c <= 97) out.fg = ANSI16[c - 90 + 8];
    else if (c >= 100 && c <= 107) out.bg = ANSI16[c - 100 + 8];
    else if (c === 38 || c === 48) {
      const key = c === 38 ? "fg" : "bg";
      if (codes[i + 1] === 5) { out[key] = color256(codes[i + 2] ?? 0); i += 2; }
      else if (codes[i + 1] === 2) { out[key] = `rgb(${codes[i + 2] ?? 0},${codes[i + 3] ?? 0},${codes[i + 4] ?? 0})`; i += 4; }
    }
  }
  return out;
}

function styleToCSS(s: Style): CSSProperties {
  let fg = s.fg ?? DEFAULT_FG;
  // Bold brightens a standard (0–7) colour to its bright (8–15) variant, exactly
  // like a real terminal (xterm's drawBoldTextInBrightColors). Without this, SGR
  // 1;30 — bold black, which ansible uses for the grey "task path:" lines at -vv —
  // renders black on the near-black background and vanishes.
  if (s.bold && s.fg) {
    const i = (ANSI16 as readonly string[]).indexOf(s.fg);
    if (i >= 0 && i < 8) fg = ANSI16[i + 8];
  }
  let bg = s.bg;
  if (s.reverse) { const t = fg; fg = bg ?? DEFAULT_BG; bg = t; } // swap on reverse-video
  return {
    color: s.hidden ? "transparent" : fg !== DEFAULT_FG || s.reverse ? fg : undefined,
    backgroundColor: bg,
    fontWeight: s.bold ? 600 : undefined,
    opacity: s.dim ? 0.7 : undefined,
    fontStyle: s.italic ? "italic" : undefined,
    textDecoration: [s.underline ? "underline" : "", s.strike ? "line-through" : ""].join(" ").trim() || undefined,
  };
}

// Match: SGR (…m) | OSC | any other CSI/escape | lone control chars. NOTE the
// dropped-control class deliberately EXCLUDES \b (0x08), \t (0x09) and \r (0x0d)
// so the cursor emulator below can act on them (back-space / tab / carriage
// return) — that's what collapses ansible-galaxy spinners and progress redraws
// to their final state, exactly like a real terminal does.
// eslint-disable-next-line no-control-regex
const TOKEN = /\x1b\[([0-9;]*)m|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)|\x1b[\[\]][0-9;?]*[ -/]*[@-~]|\x1b.|[\x00-\x07\x0b\x0c\x0e-\x1f]/g;

function sameStyle(a: Style, b: Style): boolean {
  return (
    a.fg === b.fg && a.bg === b.bg && !!a.bold === !!b.bold && !!a.dim === !!b.dim &&
    !!a.italic === !!b.italic && !!a.underline === !!b.underline && !!a.strike === !!b.strike &&
    !!a.reverse === !!b.reverse && !!a.hidden === !!b.hidden
  );
}

/**
 * Render one line of terminal output into styled React spans. It is a tiny
 * terminal emulator: it lays characters into a cell buffer and honours the
 * cursor controls a log actually uses — carriage return (col 0), back-space,
 * tab, and erase-in-line (CSI K) — so spinners / progress bars collapse to their
 * final frame instead of leaking `|/-\|/-\…` garbage. Colours come from the full
 * SGR set (16 / bright / 256 / truecolor, bold/dim/italic/underline/reverse/
 * strike, fg+bg, all resets) via the single shared palette. Every other escape
 * and control char is dropped, so a raw `[0;32m` never reaches the UI.
 */
export function ansiToSpans(line: string, keyPrefix: string): ReactNode[] {
  const cells: { ch: string; style: Style }[] = [];
  let cursor = 0;
  let style: Style = {};
  let last = 0;
  let m: RegExpExecArray | null;
  TOKEN.lastIndex = 0;

  const writeText = (text: string) => {
    for (const ch of text) {
      if (ch === "\r") { cursor = 0; continue; }
      if (ch === "\b") { if (cursor > 0) cursor--; continue; }
      if (ch === "\t") {
        const stop = (Math.floor(cursor / 8) + 1) * 8;
        while (cursor < stop) cells[cursor++] = { ch: " ", style };
        continue;
      }
      cells[cursor++] = { ch, style };
    }
  };

  while ((m = TOKEN.exec(line))) {
    writeText(line.slice(last, m.index));
    last = m.index + m[0].length;
    if (m[1] !== undefined) {
      const codes = m[1] === "" ? [0] : m[1].split(";").map((x) => parseInt(x, 10) || 0);
      style = applySGR(style, codes);
    } else {
      const tok = m[0];
      if (tok.length >= 3 && tok[1] === "[" && tok[tok.length - 1] === "K") {
        const mode = parseInt(tok.slice(2, -1), 10) || 0;
        if (mode === 0) cells.length = cursor; // erase cursor→end of line
        else if (mode === 1) for (let i = 0; i < cursor; i++) cells[i] = { ch: " ", style };
        else { cells.length = 0; cursor = 0; } // erase whole line
      }
      // every other escape / control char is intentionally dropped
    }
  }
  writeText(line.slice(last));

  const out: ReactNode[] = [];
  let n = 0;
  for (let i = 0; i < cells.length; ) {
    const s = cells[i] ? cells[i].style : {};
    let text = "";
    while (i < cells.length && sameStyle(cells[i] ? cells[i].style : {}, s)) {
      text += cells[i] ? cells[i].ch : " ";
      i++;
    }
    const css = styleToCSS(s);
    const styled = Object.values(css).some((v) => v !== undefined);
    out.push(styled ? <span key={`${keyPrefix}-${n++}`} style={css}>{text}</span> : text);
  }
  return out.length ? out : [""];
}

/**
 * If `line` is an ansible banner (`PLAY [x] ****…`, `TASK […] ****…`,
 * `PLAY RECAP ****…`), return everything before the trailing run of `*` (the
 * label, ANSI preserved). The caller renders the label then fills the rest of
 * the row width with `*` and clips it — so banners sit on ONE line, edge-to-edge
 * like Semaphore, instead of overflowing into a horizontal scroll. Non-banners
 * return null.
 */
export function bannerHead(line: string): string | null {
  const m = line.match(/\*{6,}[ \t]*$/);
  if (!m || m.index === undefined) return null;
  const head = line.slice(0, m.index);
  if (head !== "" && !/\s$/.test(head)) return null; // stars must follow a space
  return head.replace(/[ \t]+$/, "");
}
