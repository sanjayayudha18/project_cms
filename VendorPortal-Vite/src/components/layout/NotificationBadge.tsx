import { formatBadgeCount } from '@/lib/formatters';

interface NotificationBadgeProps {
  readonly count: number;
}

/**
 * Notification badge pill that displays the unread count on the Notifications nav item.
 * Shows numeric 1–99, "99+" for >99, and renders nothing for 0.
 */
export function NotificationBadge({ count }: NotificationBadgeProps) {
  const display = formatBadgeCount(count);

  if (display === null) {
    return null;
  }

  return (
    <span
      aria-label={`${count} unread notifications`}
      className="absolute -top-1 -right-1 inline-flex items-center justify-center
                 min-w-[18px] h-[18px] px-1 rounded-full
                 bg-sidebar-active text-white
                 text-[10px] font-semibold leading-none"
    >
      {display}
    </span>
  );
}
