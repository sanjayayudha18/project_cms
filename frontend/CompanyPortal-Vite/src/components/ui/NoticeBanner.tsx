import type { LucideIcon } from "lucide-react";

type NoticeBannerVariant = "warning" | "danger" | "info";

interface NoticeBannerProps {
  icon: LucideIcon;
  title: string;
  description: string;
  variant?: NoticeBannerVariant;
}

const variantStyles: Record<NoticeBannerVariant, { container: string; icon: string }> = {
  warning: {
    container: "bg-[var(--warning-bg)] text-[var(--warning-fg)]",
    icon: "text-[var(--warning-fg)]",
  },
  danger: {
    container: "bg-[var(--danger-bg)] text-[var(--danger-fg)]",
    icon: "text-[var(--danger-fg)]",
  },
  info: {
    container: "bg-[var(--info-bg)] text-[var(--info-fg)]",
    icon: "text-[var(--info-fg)]",
  },
};

/**
 * A semantic notice banner for displaying warnings, errors, or informational messages.
 * Renders an icon, bold title, and description with variant-based coloring.
 */
export function NoticeBanner({
  icon: Icon,
  title,
  description,
  variant = "warning",
}: NoticeBannerProps) {
  const styles = variantStyles[variant];

  return (
    <div className={`rounded-[var(--radius-lg)] p-4 ${styles.container}`} role="note">
      <div className="flex items-start gap-3">
        <Icon className={`h-5 w-5 shrink-0 mt-0.5 ${styles.icon}`} aria-hidden="true" />
        <div className="min-w-0">
          <p className="font-bold text-sm">{title}</p>
          <p className="text-sm mt-0.5">{description}</p>
        </div>
      </div>
    </div>
  );
}

export type { NoticeBannerVariant, NoticeBannerProps };
