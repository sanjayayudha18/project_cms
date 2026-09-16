import { cn } from "@/lib/utils/cn";
import { X } from "lucide-react";
import { type ReactNode, useEffect, useRef } from "react";
import { createPortal } from "react-dom";

interface DialogProps {
  open: boolean;
  onClose: () => void;
  title: string;
  children: ReactNode;
  className?: string;
}

const FOCUSABLE_SELECTOR =
  'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])';

/**
 * Accessible modal dialog (design-system Sec 9): role="dialog" + aria-modal,
 * focus moves in on open and traps Tab/Shift+Tab inside, Escape or a click
 * on the overlay closes, focus returns to the trigger that opened it.
 * Transition is transform+opacity only (200ms ease-out enter / 150ms exit)
 * via Tailwind's `animate-` utilities defined in styles/index.css.
 */
export function Dialog({ open, onClose, title, children, className = "" }: DialogProps) {
  const panelRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<Element | null>(null);

  useEffect(() => {
    if (!open) return;

    triggerRef.current = document.activeElement;
    const panel = panelRef.current;
    const focusable = panel?.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR);
    (focusable?.[0] ?? panel)?.focus();

    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") {
        onClose();
        return;
      }
      if (e.key !== "Tab" || !panel) return;

      const items = Array.from(panel.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR));
      const first = items[0];
      const last = items[items.length - 1];
      if (!first || !last) return;

      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault();
        first.focus();
      }
    }

    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("keydown", handleKeyDown);
      if (triggerRef.current instanceof HTMLElement) {
        triggerRef.current.focus();
      }
    };
  }, [open, onClose]);

  if (!open) return null;

  return createPortal(
    <div className="fixed inset-0 z-[9998] flex items-center justify-center p-4">
      <button
        type="button"
        aria-label="Tutup dialog"
        onClick={onClose}
        className="absolute inset-0 bg-black/40 animate-[dialog-overlay-enter_200ms_ease-out_forwards]"
      />
      <div
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby="dialog-title"
        tabIndex={-1}
        className={cn(
          "relative w-full max-w-lg rounded-[var(--radius-lg)] bg-[var(--n-0)] p-6 shadow-[var(--shadow-md)] outline-none",
          "animate-[dialog-panel-enter_200ms_ease-out_forwards]",
          className,
        )}
      >
        <div className="flex items-start justify-between gap-4">
          <h2 id="dialog-title" className="text-lg font-semibold text-[var(--n-900)]">
            {title}
          </h2>
          <button
            type="button"
            onClick={onClose}
            aria-label="Tutup"
            className="shrink-0 rounded-[var(--radius-sm)] p-1 text-[var(--n-500)] hover:bg-[var(--n-100)] hover:text-[var(--n-700)] transition-colors duration-150"
          >
            <X className="size-4" />
          </button>
        </div>
        <div className="mt-4">{children}</div>
        <div aria-live="polite" className="sr-only">
          {title} dibuka
        </div>
      </div>
    </div>,
    document.body,
  );
}
