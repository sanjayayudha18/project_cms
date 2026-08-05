import type { LucideIcon } from 'lucide-react';

type NoticeBannerVariant = 'warning' | 'danger' | 'info';

interface NoticeBannerProps {
  icon: LucideIcon;
  title: string;
  description: string;
  variant?: NoticeBannerVariant;
}

const variantStyles: Record<NoticeBannerVariant, { container: string; icon: string }> = {
  warning: {
    container: 'bg-warning-bg text-warning-fg',
    icon: 'text-warning-fg',
  },
  danger: {
    container: 'bg-danger-bg text-danger-fg',
    icon: 'text-danger-fg',
  },
  info: {
    container: 'bg-info-bg text-info-fg',
    icon: 'text-info-fg',
  },
};

/**
 * A semantic notice banner for displaying warnings, errors, or informational messages.
 * Renders an icon, bold title, and description with variant-based coloring.
 *
 * @validates Requirements 7.2
 */
export function NoticeBanner({ icon: Icon, title, description, variant = 'warning' }: NoticeBannerProps) {
  const styles = variantStyles[variant];

  return (
    <div className={`rounded-lg p-4 ${styles.container}`} role="note">
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
