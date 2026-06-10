export interface ParseResult {
  vars: Record<string, unknown>;
  error?: string;
}

/** parseExtraVars accepts a JSON object or `key: value` / `key=value` lines. */
export function parseExtraVars(text: string): ParseResult {
  const t = text.trim();
  if (!t) return { vars: {} };
  if (t.startsWith("{")) {
    try {
      return { vars: JSON.parse(t) };
    } catch {
      return { vars: {}, error: "Extra vars: invalid JSON" };
    }
  }
  const vars: Record<string, unknown> = {};
  for (const raw of t.split("\n")) {
    const line = raw.trim();
    if (!line || line.startsWith("#")) continue;
    const m = line.match(/^([\w.\-]+)\s*[:=]\s*(.*)$/);
    if (!m) return { vars: {}, error: `Extra vars: cannot parse "${line}"` };
    vars[m[1]] = coerce(m[2].trim());
  }
  return { vars };
}

function coerce(v: string): unknown {
  if (v === "true") return true;
  if (v === "false") return false;
  if (v !== "" && !Number.isNaN(Number(v))) return Number(v);
  return v.replace(/^["']|["']$/g, "");
}

/** extraVarsToText renders an extra-vars object back to editable text. */
export function extraVarsToText(vars?: Record<string, unknown>): string {
  if (!vars || Object.keys(vars).length === 0) return "";
  return JSON.stringify(vars, null, 2);
}

/** parseEnvVars parses `KEY=value` / `KEY: value` lines into a string map. */
export function parseEnvVars(text: string): { vars: Record<string, string>; error?: string } {
  const vars: Record<string, string> = {};
  for (const raw of text.split("\n")) {
    const line = raw.trim();
    if (!line || line.startsWith("#")) continue;
    const m = line.match(/^([\w.\-]+)\s*[:=]\s*(.*)$/);
    if (!m) return { vars: {}, error: `Env vars: cannot parse "${line}"` };
    vars[m[1]] = m[2].trim();
  }
  return { vars };
}

export function envVarsToText(vars?: Record<string, string>): string {
  if (!vars) return "";
  return Object.entries(vars)
    .map(([k, v]) => `${k}=${v}`)
    .join("\n");
}
