import { create } from "zustand";

// ─── Types ────────────────────────────────────────────────────────────────────

export type ToastType = "success" | "error" | "warning" | "info";

export interface Toast {
  id: string;
  type: ToastType;
  message: string;
  createdAt: number;
}

export interface ToastOptions {
  type: ToastType;
  message: string;
}

interface ToastState {
  toasts: Toast[];
}

interface ToastActions {
  toast: (options: ToastOptions) => string;
  dismiss: (id: string) => void;
  dismissAll: () => void;
}

// ─── Constants ────────────────────────────────────────────────────────────────

const MAX_VISIBLE = 5;
const AUTO_DISMISS_MS = 5000;

// ─── Store ────────────────────────────────────────────────────────────────────

export type ToastStore = ToastState & ToastActions;

export const useToastStore = create<ToastStore>((set) => ({
  toasts: [],

  toast: (options) => {
    const id = crypto.randomUUID();
    const newToast: Toast = {
      id,
      type: options.type,
      message: options.message,
      createdAt: Date.now(),
    };

    set((state) => {
      const updated = [...state.toasts, newToast];
      // Enforce max visible — remove oldest when exceeded
      if (updated.length > MAX_VISIBLE) {
        return { toasts: updated.slice(updated.length - MAX_VISIBLE) };
      }
      return { toasts: updated };
    });

    return id;
  },

  dismiss: (id) => {
    set((state) => ({
      toasts: state.toasts.filter((t) => t.id !== id),
    }));
  },

  dismissAll: () => {
    set({ toasts: [] });
  },
}));

// ─── Hook ─────────────────────────────────────────────────────────────────────

export function useToast() {
  const toast = useToastStore((s) => s.toast);
  const dismiss = useToastStore((s) => s.dismiss);
  const dismissAll = useToastStore((s) => s.dismissAll);
  const toasts = useToastStore((s) => s.toasts);

  return { toast, dismiss, dismissAll, toasts };
}

export { AUTO_DISMISS_MS, MAX_VISIBLE };
