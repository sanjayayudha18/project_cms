import { Button } from "@/components/ui/Button";
import { EmptyState } from "@/components/ui/EmptyState";
import { truncate } from "@/lib/formatters";
import type { Notification } from "@/lib/types";
import { useNavigate } from "@tanstack/react-router";
import { Bell, CheckCheck } from "lucide-react";
import { useMarkAllAsRead, useMarkAsRead, useNotifications } from "./useNotifications";

/**
 * Format ISO timestamp to "DD MMM YYYY HH:mm" in Indonesian locale.
 */
function formatTimestamp(iso: string): string {
  const date = new Date(iso);
  return date.toLocaleDateString("id-ID", {
    day: "2-digit",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function NotificationsPage() {
  const { data: notifications = [], unreadCount, isLoading } = useNotifications();
  const { mutate: markAsRead } = useMarkAsRead();
  const { mutate: markAllAsRead, isPending: isMarkingAll } = useMarkAllAsRead();
  const navigate = useNavigate();

  // Sort by timestamp descending (newest first)
  const sorted = [...notifications].sort(
    (a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime(),
  );

  function handleNotificationClick(notification: Notification) {
    if (!notification.isRead) {
      markAsRead(notification.id);
    }
    // relatedRoute is an arbitrary internal path; href resolves dynamic segments.
    void navigate({ href: notification.relatedRoute });
  }

  function handleMarkAllAsRead() {
    markAllAsRead();
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <div className="size-6 animate-spin rounded-full border-2 border-neutral-300 border-t-sidebar-active" />
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold text-surface-text">Notifications</h1>
        <Button
          variant="ghost"
          size="sm"
          disabled={unreadCount === 0}
          isLoading={isMarkingAll}
          onClick={handleMarkAllAsRead}
          aria-label="Mark all notifications as read"
        >
          <CheckCheck className="size-4" aria-hidden="true" />
          Mark All as Read
        </Button>
      </div>

      {/* Empty state */}
      {sorted.length === 0 ? (
        <EmptyState
          icon={Bell}
          title="Tidak ada notifikasi"
          description="Belum ada notifikasi untuk ditampilkan."
        />
      ) : (
        /* Notification list table */
        <div className="overflow-x-auto rounded-lg border border-neutral-200">
          <table className="w-full text-left text-sm">
            <thead>
              <tr className="border-b border-neutral-200 bg-neutral-50">
                <th className="px-4 py-3 text-xs font-medium uppercase tracking-wider text-neutral-500">
                  Timestamp
                </th>
                <th className="px-4 py-3 text-xs font-medium uppercase tracking-wider text-neutral-500">
                  Type
                </th>
                <th className="px-4 py-3 text-xs font-medium uppercase tracking-wider text-neutral-500">
                  Message
                </th>
                <th className="px-4 py-3 text-xs font-medium uppercase tracking-wider text-neutral-500">
                  Status
                </th>
              </tr>
            </thead>
            <tbody>
              {sorted.map((notification) => (
                <tr
                  key={notification.id}
                  onClick={() => handleNotificationClick(notification)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" || e.key === " ") {
                      e.preventDefault();
                      handleNotificationClick(notification);
                    }
                  }}
                  tabIndex={0}
                  aria-label={`${notification.type}: ${notification.message}`}
                  className={[
                    "cursor-pointer border-b border-neutral-100 transition-colors duration-150",
                    "hover:bg-neutral-50 focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-sidebar-active",
                    notification.isRead ? "bg-white" : "bg-info-bg/30 font-bold",
                  ].join(" ")}
                >
                  <td className="whitespace-nowrap px-4 py-3 text-neutral-600">
                    {formatTimestamp(notification.timestamp)}
                  </td>
                  <td className="whitespace-nowrap px-4 py-3 text-neutral-700">
                    {notification.type}
                  </td>
                  <td className="px-4 py-3 text-surface-text">
                    {truncate(notification.message, 120)}
                  </td>
                  <td className="whitespace-nowrap px-4 py-3">
                    {notification.isRead ? (
                      <span className="inline-flex items-center gap-1 text-xs text-neutral-500">
                        Read
                      </span>
                    ) : (
                      <span className="inline-flex items-center gap-1 rounded-full bg-info-bg px-2 py-0.5 text-xs font-medium text-info-fg">
                        Unread
                      </span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
