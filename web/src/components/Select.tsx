import { useEffect, useRef, useState } from "react";
import clsx from "clsx";
import { Check, ChevronDown } from "lucide-react";

export interface Option {
  label: string;
  value: string;
  hint?: string;
}

/**
 * Styled dropdown — replaces native <select> everywhere so the option list
 * matches the app theme (native selects render an un-themable OS popup).
 * Drop-in-ish: pass value + options + onChange(value).
 */
export function Select({
  value,
  options,
  onChange,
  placeholder = "Select…",
  disabled,
  className,
}: {
  value: string;
  options: Option[];
  onChange: (value: string) => void;
  placeholder?: string;
  disabled?: boolean;
  className?: string;
}) {
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(0);
  const ref = useRef<HTMLDivElement>(null);

  const selected = options.find((o) => o.value === value);

  useEffect(() => {
    if (!open) return;
    const onDoc = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  }, [open]);

  useEffect(() => {
    if (open) setActive(Math.max(0, options.findIndex((o) => o.value === value)));
  }, [open, options, value]);

  const pick = (v: string) => {
    onChange(v);
    setOpen(false);
  };

  const onKey = (e: React.KeyboardEvent) => {
    if (disabled) return;
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      if (open && options[active]) pick(options[active].value);
      else setOpen(true);
    } else if (e.key === "Escape") {
      setOpen(false);
    } else if (e.key === "ArrowDown") {
      e.preventDefault();
      setOpen(true);
      setActive((a) => Math.min(options.length - 1, a + 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActive((a) => Math.max(0, a - 1));
    }
  };

  return (
    <div ref={ref} className={clsx("relative", className)}>
      <button
        type="button"
        disabled={disabled}
        onClick={() => !disabled && setOpen((o) => !o)}
        onKeyDown={onKey}
        className={clsx(
          "input flex w-full items-center justify-between gap-2 text-left",
          disabled && "cursor-not-allowed opacity-50",
        )}
        aria-haspopup="listbox"
        aria-expanded={open}
      >
        <span className={clsx("truncate", !selected && "text-ink-faint")}>
          {selected ? selected.label : placeholder}
        </span>
        <ChevronDown size={15} className={clsx("shrink-0 text-ink-faint transition-transform", open && "rotate-180")} />
      </button>

      {open && (
        <div
          role="listbox"
          className="absolute z-50 mt-1 max-h-72 w-full min-w-max overflow-auto rounded-lg border border-border-strong bg-panel2 p-1 shadow-soft animate-fadeIn"
        >
          {options.length === 0 && <div className="px-2.5 py-2 text-xs text-ink-faint">No options</div>}
          {options.map((o, i) => {
            const isSel = o.value === value;
            return (
              <button
                key={o.value + i}
                type="button"
                onClick={() => pick(o.value)}
                onMouseEnter={() => setActive(i)}
                className={clsx(
                  "flex w-full items-center gap-2 rounded-md px-2.5 py-1.5 text-left text-sm transition-colors",
                  i === active ? "bg-accent/15 text-ink" : "text-ink-dim",
                  isSel && "text-accent",
                )}
                role="option"
                aria-selected={isSel}
              >
                <Check size={14} className={clsx("shrink-0", isSel ? "opacity-100 text-accent" : "opacity-0")} />
                <span className="min-w-0 flex-1 truncate">{o.label}</span>
                {o.hint && <span className="shrink-0 text-xs text-ink-faint">{o.hint}</span>}
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}
