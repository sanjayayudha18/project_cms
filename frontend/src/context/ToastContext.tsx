import { createContext, useCallback, useContext, useMemo, useRef, useState } from 'react';
import type { ReactNode } from 'react';

import { Toast } from '@/components/ui/Toast';
import type { ToastMessage } from '@/components/ui/Toast';

interface ToastContextValue {
  showToast: (text: string, icon?: ToastMessage['icon']) => void;
}

const ToastContext = createContext<ToastContextValue>({
  showToast: () => {},
});

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toast, setToast] = useState<ToastMessage | null>(null);
  const [visible, setVisible] = useState(false);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const animateOutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const clearTimers = useCallback(() => {
    if (timerRef.current) {
      clearTimeout(timerRef.current);
      timerRef.current = null;
    }
    if (animateOutRef.current) {
      clearTimeout(animateOutRef.current);
      animateOutRef.current = null;
    }
  }, []);

  const dismiss = useCallback(() => {
    setVisible(false);
    // Remove toast from DOM after animate-out completes (220ms)
    animateOutRef.current = setTimeout(() => {
      setToast(null);
    }, 220);
  }, []);

  const showToast = useCallback(
    (text: string, icon?: ToastMessage['icon']) => {
      clearTimers();

      const id = `${Date.now()}-${Math.random().toString(36).slice(2, 9)}`;
      setToast({ id, text, icon });
      setVisible(true);

      // Auto-dismiss after 4 seconds
      timerRef.current = setTimeout(() => {
        dismiss();
      }, 4000);
    },
    [clearTimers, dismiss],
  );

  const value = useMemo<ToastContextValue>(() => ({ showToast }), [showToast]);

  return (
    <ToastContext.Provider value={value}>
      {children}
      {toast ? <Toast message={toast} visible={visible} /> : null}
    </ToastContext.Provider>
  );
}

export function useToast(): ToastContextValue {
  return useContext(ToastContext);
}

export { ToastContext };
export type { ToastContextValue };
