import { CheckCircle2, Info, AlertTriangle } from 'lucide-react';
import type { LucideIcon } from 'lucide-react';

export interface ToastMessage {
  id: string;
  text: string;
  icon?: 'success' | 'info' | 'warning';
}

interface ToastProps {
  message: ToastMessage;
  visible: boolean;
}

const iconMap: Record<NonNullable<ToastMessage['icon']>, LucideIcon> = {
  success: CheckCircle2,
  info: Info,
  warning: AlertTriangle,
};

export function Toast({ message, visible }: ToastProps) {
  const Icon = message.icon ? iconMap[message.icon] : null;

  return (
    <div
      role="status"
      aria-live="polite"
      className={`
        fixed z-50 flex items-center gap-3 rounded-[var(--radius-lg)] px-4 py-3
        text-[var(--n-0)] shadow-[var(--shadow-md)]
        transition-all duration-[220ms] ease-out
        right-4 bottom-4
        max-[759px]:right-auto max-[759px]:bottom-4 max-[759px]:left-4 max-[759px]:w-[calc(100%-32px)]
        ${visible ? 'translate-y-0 opacity-100' : 'translate-y-3 opacity-0'}
      `}
      style={{ backgroundColor: 'var(--n-900)' }}
    >
      {Icon ? (
        <Icon className="h-5 w-5 shrink-0" aria-hidden="true" />
      ) : null}
      <span className="text-sm font-medium">{message.text}</span>
    </div>
  );
}
