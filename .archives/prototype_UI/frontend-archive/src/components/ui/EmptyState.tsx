import { Inbox, type LucideIcon } from 'lucide-react';

interface EmptyStateProps {
  icon?: LucideIcon;
  message: string;
}

/**
 * Empty state placeholder with icon and message.
 * Rendered when filters yield no results or data is unavailable.
 */
export function EmptyState({ icon: Icon = Inbox, message }: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center justify-center py-12 text-center">
      <Icon className="h-12 w-12 text-n-300 mb-3" aria-hidden="true" />
      <p className="text-n-500 text-sm">{message}</p>
    </div>
  );
}
