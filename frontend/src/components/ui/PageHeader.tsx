import type { ReactNode } from "react";

interface PageHeaderProps {
  eyebrow: string;
  title: string;
  description?: string;
  actions?: ReactNode;
}

/**
 * Consistent page-level header used across all screens.
 * Renders eyebrow label, title, optional description, and action buttons.
 */
export function PageHeader({ eyebrow, title, description, actions }: PageHeaderProps) {
  return (
    <header className="flex flex-col gap-1 mb-8 max-[759px]:mb-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="flex flex-col gap-1">
          <span className="text-[0.75rem] font-[800] uppercase tracking-[0.09em] text-[var(--red-600)]">
            {eyebrow}
          </span>
          <h1
            className="text-[clamp(1.85rem,3vw,2.55rem)] font-bold tracking-[-0.045em] text-[var(--n-900)]"
            style={{ textWrap: "balance" }}
          >
            {title}
          </h1>
          {description && (
            <p className="text-[var(--n-500)] max-w-[62ch] text-sm mt-1">{description}</p>
          )}
        </div>

        {actions && (
          <div className="flex items-center gap-3 max-[759px]:w-full max-[759px]:flex-col max-[759px]:items-stretch max-[759px]:gap-2">
            {actions}
          </div>
        )}
      </div>
    </header>
  );
}
