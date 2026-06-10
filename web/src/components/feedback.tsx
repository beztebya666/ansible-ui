// Styled, in-app feedback — a toaster and a promise-based confirm dialog — so we
// never use the browser's native alert()/confirm(). See memory: no-native-html-ui.
import { createContext, useCallback, useContext, useRef, useState, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { CheckCircle2, Info, X, XCircle } from "lucide-react";
import { ConfirmDialog, Modal } from "./ui";
import { usePrefs } from "../lib/prefs";

type ToastKind = "success" | "error" | "info";
interface Toast {
  id: number;
  kind: ToastKind;
  msg: string;
}

interface ConfirmOpts {
  title: string;
  body?: ReactNode;
  confirmLabel?: string;
  cancelLabel?: string;
  danger?: boolean;
}

interface PromptOpts {
  title: string;
  placeholder?: string;
  defaultValue?: string;
  confirmLabel?: string;
}

interface FeedbackCtx {
  toast: {
    success: (msg: string) => void;
    error: (msg: string) => void;
    info: (msg: string) => void;
  };
  confirm: (opts: string | ConfirmOpts) => Promise<boolean>;
  prompt: (opts: string | PromptOpts) => Promise<string | null>;
}

const Ctx = createContext<FeedbackCtx | null>(null);

export function useToast() {
  return useFeedback().toast;
}
export function useConfirm() {
  return useFeedback().confirm;
}
export function usePrompt() {
  return useFeedback().prompt;
}
function useFeedback(): FeedbackCtx {
  const ctx = useContext(Ctx);
  if (!ctx) throw new Error("useFeedback must be used within FeedbackProvider");
  return ctx;
}

const toastTone: Record<ToastKind, { icon: typeof Info; cls: string }> = {
  success: { icon: CheckCircle2, cls: "border-success/40 text-success" },
  error: { icon: XCircle, cls: "border-danger/40 text-danger" },
  info: { icon: Info, cls: "border-info/40 text-info" },
};

export function FeedbackProvider({ children }: { children: ReactNode }) {
  const { t } = usePrefs();
  const [toasts, setToasts] = useState<Toast[]>([]);
  const seq = useRef(0);
  const remove = useCallback((id: number) => setToasts((t) => t.filter((x) => x.id !== id)), []);
  const push = useCallback(
    (kind: ToastKind, msg: string) => {
      const id = ++seq.current;
      setToasts((t) => [...t, { id, kind, msg }]);
      window.setTimeout(() => remove(id), kind === "error" ? 6000 : 3500);
    },
    [remove],
  );
  const toast = useRef({
    success: (m: string) => push("success", m),
    error: (m: string) => push("error", m),
    info: (m: string) => push("info", m),
  }).current;

  // Promise-based confirm.
  const [confirmState, setConfirmState] = useState<(ConfirmOpts & { resolve: (v: boolean) => void }) | null>(null);
  const confirm = useCallback(
    (opts: string | ConfirmOpts) =>
      new Promise<boolean>((resolve) =>
        setConfirmState({ ...(typeof opts === "string" ? { title: opts } : opts), resolve }),
      ),
    [],
  );
  const settle = (v: boolean) => {
    confirmState?.resolve(v);
    setConfirmState(null);
  };

  // Promise-based text prompt (replaces native prompt()).
  const [promptState, setPromptState] = useState<(PromptOpts & { resolve: (v: string | null) => void }) | null>(null);
  const [promptValue, setPromptValue] = useState("");
  const prompt = useCallback(
    (opts: string | PromptOpts) =>
      new Promise<string | null>((resolve) => {
        const o = typeof opts === "string" ? { title: opts } : opts;
        setPromptValue(o.defaultValue ?? "");
        setPromptState({ ...o, resolve });
      }),
    [],
  );
  const settlePrompt = (v: string | null) => {
    promptState?.resolve(v);
    setPromptState(null);
  };

  return (
    <Ctx.Provider value={{ toast, confirm, prompt }}>
      {children}
      {createPortal(
        <div className="pointer-events-none fixed bottom-4 right-4 z-[60] flex w-full max-w-sm flex-col gap-2">
          {toasts.map((t) => {
            const tone = toastTone[t.kind];
            const Icon = tone.icon;
            return (
              <div
                key={t.id}
                className={`pointer-events-auto flex items-start gap-2.5 rounded-xl border bg-panel px-3.5 py-3 text-sm text-ink shadow-soft animate-fadeIn ${tone.cls}`}
              >
                <Icon size={16} className="mt-0.5 shrink-0" />
                <span className="min-w-0 flex-1 break-words text-ink-dim">{t.msg}</span>
                <button onClick={() => remove(t.id)} className="btn-ghost -mr-1 -mt-1 p-1 text-ink-faint">
                  <X size={14} />
                </button>
              </div>
            );
          })}
        </div>,
        document.body,
      )}
      <ConfirmDialog
        open={!!confirmState}
        title={confirmState?.title ?? ""}
        body={confirmState?.body}
        confirmLabel={confirmState?.confirmLabel ?? t("common.delete")}
        cancelLabel={confirmState?.cancelLabel ?? t("common.cancel")}
        danger={confirmState?.danger ?? true}
        onConfirm={() => settle(true)}
        onClose={() => settle(false)}
      />
      <Modal
        open={!!promptState}
        onClose={() => settlePrompt(null)}
        title={promptState?.title ?? ""}
        footer={
          <>
            <button className="btn-outline" onClick={() => settlePrompt(null)}>{t("common.cancel")}</button>
            <button className="btn-primary" onClick={() => settlePrompt(promptValue)} disabled={!promptValue.trim()}>
              {promptState?.confirmLabel ?? t("common.save")}
            </button>
          </>
        }
      >
        <input
          className="input"
          autoFocus
          placeholder={promptState?.placeholder}
          value={promptValue}
          onChange={(e) => setPromptValue(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" && promptValue.trim()) settlePrompt(promptValue);
          }}
        />
      </Modal>
    </Ctx.Provider>
  );
}
