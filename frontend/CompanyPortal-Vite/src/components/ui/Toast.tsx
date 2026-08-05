import {
  AUTO_DISMISS_MS,
  type Toast as ToastData,
  type ToastType,
  useToastStore,
} from "@/lib/hooks/useToast";
import { cn } from "@/lib/utils/cn";
import { AlertTriangle, CheckCircle, Info, X, XCircle } from "lucide-react";
import { useEffect, useRef } from "react";

// ─── Icon + Style Mapping ─────────────────────────────────────────────────────

const TOAST_CONFIG: Record<
  ToastType,
  { icon: typeof CheckCircle; containerClass: string; iconClass: string; label: string }
> = {
  success: {
    icon: CheckCircle,
    containerClass: "bg-[var(--success-bg)] border-[var(--success-solid)]",
    iconClass: "text-[var(--success-fg)]",
    label: "Berhasil",
  },
  error: {
    icon: XCircle,
    containerClass: "bg-[var(--danger-bg)] border-[var(--danger-fg)]",
    iconClass: "text-[var(--danger-fg)]",
    label: "Gagal",
  },
  warning: {
    icon: AlertTriangle,
    containerClass: "bg-[var(--warning-bg)] border-[var(--warning-solid)]",
    iconClass: "text-[var(--warning-fg)]",
    label: "Peringatan",
  },
  info: {
    icon: Info,
    containerClass: "bg-[var(--info-bg)] border-[var(--info-solid)]",
    iconClass: "text-[var(--info-fg)]",
    label: "Informasi",
  },
};

// ─── Single Toast ─────────────────────────────────────────────────────────────

interface ToastItemProps {
  toast: ToastData;
  onDismiss: (id: string) => void;
}

function ToastItem({ toast, onDismiss }: ToastItemProps) {
  const config = TOAST_CONFIG[toast.type];
  const Icon = config.icon;
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    // Success, warning, and info toasts auto-dismiss after 5s
    // Error toasts persist until manually dismissed
    if (toast.type !== "error") {
      timerRef.current = setTimeout(() => {
        onDismiss(toast.id);
      }, AUTO_DISMISS_MS);
    }

    return () => {
      if (timerRef.current) {
        clearTimeout(timerRef.current);
      }
    };
  }, [toast.id, toast.type, onDismiss]);

  return (
    <div
      role="alert"
      className={cn(
        "pointer-events-auto flex w-80 items-start gap-3 rounded-[var(--radius-lg)] border p-4 shadow-[var(--shadow-md)]",
        "animate-[toast-enter_300ms_cubic-bezier(0.22,1,0.36,1)_forwards]",
        config.containerClass,
      )}
    >
      <Icon className={cn("mt-0.5 size-5 shrink-0", config.iconClass)} aria-hidden="true" />
      <div className="flex-1 min-w-0">
        <p className="text-sm font-medium text-[var(--n-900)]">{config.label}</p>
        <p className="mt-0.5 text-sm text-[var(--n-700)] break-words">{toast.message}</p>
      </div>
      <button
        type="button"
        onClick={() => onDismiss(toast.id)}
        className="shrink-0 rounded-[var(--radius-sm)] p-1 text-[var(--n-500)] hover:text-[var(--n-700)] hover:bg-[var(--n-100)] transition-colors duration-150"
        aria-label="Tutup notifikasi"
      >
        <X className="size-4" />
      </button>
    </div>
  );
}

// ─── Toast Container ──────────────────────────────────────────────────────────

export function ToastContainer() {
  const toasts = useToastStore((s) => s.toasts);
  const dismiss = useToastStore((s) => s.dismiss);

  return (
    <div
      aria-live="polite"
      aria-atomic="false"
      className="pointer-events-none fixed inset-0 z-[9999] flex flex-col items-end gap-3 p-4 pt-4"
    >
      {toasts.map((t) => (
        <ToastItem key={t.id} toast={t} onDismiss={dismiss} />
      ))}
    </div>
  );
}
