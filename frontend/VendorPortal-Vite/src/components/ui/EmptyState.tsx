import type { LucideIcon } from "lucide-react";
import { Inbox } from "lucide-react";

interface EmptyStateProps {
  readonly icon?: LucideIcon;
  readonly title: string;
  readonly description?: string;
}

/**
 * Empty state placeholder with centered icon, title, and optional description.
 * Used when data tables or lists have no results to display.
 */
export function EmptyState({ icon: Icon = Inbox, title, description }: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center justify-center gap-3 py-12 px-4 text-center">
      <Icon className="size-12 text-neutral-400" aria-hidden="true" />
      <h3 className="text-base font-semibold text-surface-text">{title}</h3>
      {description && <p className="text-sm text-neutral-500 max-w-sm">{description}</p>}
    </div>
  );
}
