import type { LucideIcon } from "lucide-react";

export type BadgeVariant = "success" | "warning" | "danger" | "info" | "neutral";

interface BadgeProps {
  variant: BadgeVariant;
  icon?: LucideIcon;
  label: string;
}

const variantStyles: Record<BadgeVariant, string> = {
  success: "bg-[var(--success-bg)] text-[var(--success-fg)]",
  warning: "bg-[var(--warning-bg)] text-[var(--warning-fg)]",
  danger: "bg-[var(--danger-bg)] text-[var(--danger-fg)]",
  info: "bg-[var(--info-bg)] text-[var(--info-fg)]",
  neutral: "bg-[var(--n-100)] text-[var(--n-600)]",
};

export function Badge({ variant, icon: Icon, label }: BadgeProps) {
  return (
    <span
      className={`inline-flex items-center gap-1 rounded-[var(--radius-sm)] px-2 py-0.5 text-xs font-medium ${variantStyles[variant]}`}
    >
      {Icon ? <Icon className="h-3.5 w-3.5 shrink-0" aria-hidden="true" /> : null}
      {label}
    </span>
  );
}

export type { BadgeProps };
