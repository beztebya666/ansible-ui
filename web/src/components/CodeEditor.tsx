import { useEffect, useRef } from "react";
import { Annotation, EditorState, type Extension } from "@codemirror/state";
import {
  EditorView,
  keymap,
  lineNumbers,
  highlightActiveLine,
  highlightActiveLineGutter,
  drawSelection,
  placeholder as cmPlaceholder,
} from "@codemirror/view";
import { defaultKeymap, history, historyKeymap, indentWithTab } from "@codemirror/commands";
import {
  syntaxHighlighting,
  HighlightStyle,
  indentOnInput,
  bracketMatching,
  foldGutter,
  StreamLanguage,
} from "@codemirror/language";
import { yaml } from "@codemirror/lang-yaml";
import { json } from "@codemirror/lang-json";
import { properties } from "@codemirror/legacy-modes/mode/properties";
import { shell } from "@codemirror/legacy-modes/mode/shell";
import { python } from "@codemirror/legacy-modes/mode/python";
import { powerShell } from "@codemirror/legacy-modes/mode/powershell";
import { tags as t } from "@lezer/highlight";

// CodeMirror has no bundled HCL mode; this minimal stream parser colours the
// Terraform/Terragrunt essentials (blocks, strings, numbers, comments).
const hclMode = StreamLanguage.define<{ inString: boolean }>({
  startState: () => ({ inString: false }),
  token(stream) {
    if (stream.eatSpace()) return null;
    if (stream.match(/^(#|\/\/).*/)) return "comment";
    if (stream.match(/^"(?:[^"\\]|\\.)*"?/)) return "string";
    if (stream.match(/^-?\d+(?:\.\d+)?/)) return "number";
    if (stream.match(/^(true|false|null)\b/)) return "atom";
    if (
      stream.match(
        /^(resource|variable|output|module|data|provider|terraform|locals|for_each|count|depends_on|dynamic|if|for|in)\b/,
      )
    )
      return "keyword";
    if (stream.match(/^\$\{[^}]*\}?/)) return "variable-2";
    if (stream.match(/^[A-Za-z_][\w-]*/)) return "variable";
    stream.next();
    return null;
  },
});

// VSCode-grade token colours, tuned to the app palette.
const highlight = HighlightStyle.define([
  { tag: [t.keyword, t.moduleKeyword, t.definitionKeyword], color: "#c084fc" },
  { tag: [t.atom, t.bool, t.special(t.variableName)], color: "#fbbf24" },
  { tag: [t.number, t.integer, t.float], color: "#fbbf24" },
  { tag: [t.string, t.special(t.string), t.docString], color: "#6ee7b7" },
  { tag: [t.propertyName, t.attributeName], color: "#7aa2f7" },
  { tag: [t.comment, t.lineComment, t.blockComment], color: "#5b6677", fontStyle: "italic" },
  { tag: [t.variableName, t.name], color: "#e6e9ef" },
  { tag: [t.typeName, t.className, t.namespace], color: "#22d3ee" },
  { tag: [t.operator, t.punctuation, t.separator], color: "#9aa4b2" },
  { tag: [t.bracket, t.brace, t.squareBracket, t.paren], color: "#9aa4b2" },
  { tag: [t.meta, t.documentMeta], color: "#22d3ee" },
  { tag: t.invalid, color: "#f87171" },
  { tag: [t.heading, t.strong], fontWeight: "bold", color: "#e6e9ef" },
  { tag: t.link, color: "#7aa2f7", textDecoration: "underline" },
]);

const theme = EditorView.theme(
  {
    // maxHeight:inherit lets a max-h-* host (e.g. the API Explorer response)
    // bound the editor so .cm-scroller actually scrolls instead of overflowing.
    "&": { backgroundColor: "transparent", color: "#c8d0dc", height: "100%", maxHeight: "inherit", fontSize: "13px" },
    ".cm-scroller": {
      fontFamily: "'JetBrains Mono', ui-monospace, SFMono-Regular, Menlo, Consolas, monospace",
      lineHeight: "1.6",
      overflow: "auto",
    },
    ".cm-content": { caretColor: "#34d399", padding: "10px 0" },
    ".cm-gutters": { backgroundColor: "transparent", color: "#3b4452", border: "none" },
    ".cm-activeLineGutter": { backgroundColor: "transparent", color: "#5b6677" },
    ".cm-activeLine": { backgroundColor: "rgba(255,255,255,0.025)" },
    "&.cm-focused": { outline: "none" },
    ".cm-cursor": { borderLeftColor: "#34d399" },
    ".cm-selectionBackground, .cm-content ::selection": { backgroundColor: "rgba(52,211,153,0.22)" },
    "&.cm-focused .cm-selectionBackground": { backgroundColor: "rgba(52,211,153,0.28)" },
    ".cm-matchingBracket": { backgroundColor: "transparent", outline: "none", color: "inherit" },
    ".cm-foldGutter span": { color: "#3b4452" },
  },
  { dark: true },
);

function languageExt(lang: string): Extension[] {
  switch (lang) {
    case "yaml":
      return [yaml()];
    case "json":
      return [json()];
    case "ini":
      return [StreamLanguage.define(properties)];
    case "hcl":
      return [hclMode];
    case "shell":
      return [StreamLanguage.define(shell)];
    case "python":
      return [StreamLanguage.define(python)];
    case "powershell":
      return [StreamLanguage.define(powerShell)];
    default:
      return [];
  }
}

// Marks a programmatic value sync (loading a different file) so the update
// listener can tell it apart from a real user edit — otherwise just opening a
// file would fire onChange and flag it "dirty" (the phantom unsaved ● dot).
const syncAnnotation = Annotation.define<boolean>();

export type CodeLang = "yaml" | "json" | "ini" | "hcl" | "shell" | "python" | "powershell" | "text";

export function langFromPath(path: string): CodeLang {
  const p = path.toLowerCase();
  if (p.endsWith(".yml") || p.endsWith(".yaml") || p.endsWith(".j2") || p.endsWith(".cfg")) return "yaml";
  if (p.endsWith(".json")) return "json";
  if (p.endsWith(".tf") || p.endsWith(".tfvars") || p.endsWith(".hcl")) return "hcl";
  if (p.endsWith(".sh") || p.endsWith(".bash")) return "shell";
  if (p.endsWith(".py")) return "python";
  if (p.endsWith(".ps1") || p.endsWith(".psm1")) return "powershell";
  if (p.endsWith(".ini") || p.includes("inventory") || p.endsWith("hosts")) return "ini";
  return "text";
}

interface Props {
  value: string;
  onChange?: (v: string) => void;
  language?: CodeLang;
  readOnly?: boolean;
  placeholder?: string;
  className?: string;
}

export function CodeEditor({ value, onChange, language = "yaml", readOnly = false, placeholder, className }: Props) {
  const host = useRef<HTMLDivElement>(null);
  const view = useRef<EditorView | null>(null);
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;

  // Rebuild the editor when language/readOnly changes.
  useEffect(() => {
    if (!host.current) return;
    const exts: Extension[] = [
      lineNumbers(),
      foldGutter(),
      drawSelection(),
      history(),
      indentOnInput(),
      syntaxHighlighting(highlight),
      // Active-line + matching-bracket highlights only while editing (keeps the
      // read-only JSON viewer clean — no green braces / highlighted line).
      ...(readOnly ? [] : [highlightActiveLine(), highlightActiveLineGutter(), bracketMatching()]),
      keymap.of([...defaultKeymap, ...historyKeymap, indentWithTab]),
      theme,
      EditorView.lineWrapping,
      EditorState.tabSize.of(2),
      ...languageExt(language),
      EditorState.readOnly.of(readOnly),
      EditorView.editable.of(!readOnly),
      EditorView.updateListener.of((u) => {
        // Ignore doc changes we made ourselves to load a file (annotated as a
        // sync) — only user edits should bubble up and mark the buffer dirty.
        if (u.docChanged && !u.transactions.some((tr) => tr.annotation(syncAnnotation))) {
          onChangeRef.current?.(u.state.doc.toString());
        }
      }),
    ];
    if (placeholder) exts.push(cmPlaceholder(placeholder));

    const state = EditorState.create({ doc: value, extensions: exts });
    const v = new EditorView({ state, parent: host.current });
    view.current = v;
    return () => {
      v.destroy();
      view.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [language, readOnly]);

  // Sync external value changes (e.g. loading a different file).
  useEffect(() => {
    const v = view.current;
    if (!v) return;
    const current = v.state.doc.toString();
    if (value !== current) {
      v.dispatch({
        changes: { from: 0, to: current.length, insert: value },
        annotations: syncAnnotation.of(true),
      });
    }
  }, [value]);

  return <div ref={host} className={className} />;
}
