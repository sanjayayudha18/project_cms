import type { LucideIcon } from 'lucide-react';
import { AlertTriangle, CheckCircle, Circle, Info, XCircle } from 'lucide-react';

export interface BadgeProps {
  readonly variant: 'info' | 'warning' | 'success' | 'danger' | 'neutral';
  readonly icon?: LucideIcon;
  readonly children: React.ReactNode;
}

const defaultIcons: Record<BadgeProps['variant'], LucideIcon> = {
  info: Info,
  warning: AlertTriangle,
  success: CheckCircle,
  danger: XCircle,
  neutral: Circle,
};

const variantClasses: Record<BadgeProps['variant'], string> = {
  info: 'bg-info-bg text-info-fg',
  warning: 'bg-warning-bg text-warning-fg',
  success: 'bg-success-bg text-success-fg',
  danger: 'bg-danger-bg text-danger-fg',
  neutral: 'bg-neutral-100 text-neutral-600',
};

export function Badge({ variant, icon, children }: BadgeProps) {
  const Icon = icon ?? defaultIcons[variant];

  return (
    <span
      className={`inline-flex items-center gap-1 rounded-md px-2 py-0.5 text-xs font-medium ${variantClasses[variant]}`}
    >
      <Icon className="size-3.5 shrink-0" aria-hidden="true" />
      <span>{children}</span>
    </span>
  );
}
