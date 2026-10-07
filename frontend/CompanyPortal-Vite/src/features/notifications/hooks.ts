/**
 * Notification hooks. Unread count polls every 60 s (spec decision 3). After
 * mark read / read all the cache is updated in place instead of refetching:
 * list + count are served from the read replica, which may lag the primary
 * write (spec FR4.5).
 */

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  type NotificationListResponse,
  fetchNotifications,
  fetchUnreadCount,
  markAllNotificationsRead,
  markNotificationRead,
} from "./api";

export const UNREAD_POLL_MS = 60_000;
export const RECENT_PAGE_SIZE = 10;

export const notificationKeys = {
  all: ["notifications"] as const,
  unreadCount: ["notifications", "unread-count"] as const,
  recent: ["notifications", "recent"] as const,
};

export function useUnreadCount() {
  return useQuery({
    queryKey: notificationKeys.unreadCount,
    queryFn: fetchUnreadCount,
    refetchInterval: UNREAD_POLL_MS,
  });
}

export function useRecentNotifications(enabled: boolean) {
  return useQuery({
    queryKey: notificationKeys.recent,
    queryFn: () => fetchNotifications(1, RECENT_PAGE_SIZE),
    enabled,
  });
}

export function useMarkRead() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: markNotificationRead,
    onSuccess: (_data, id) => {
      const list = queryClient.getQueryData<NotificationListResponse>(notificationKeys.recent);
      const wasUnread = list?.items.some((n) => n.id === id && !n.is_read) ?? false;
      queryClient.setQueryData<NotificationListResponse>(notificationKeys.recent, (old) =>
        old
          ? { ...old, items: old.items.map((n) => (n.id === id ? { ...n, is_read: true } : n)) }
          : old,
      );
      if (wasUnread) {
        queryClient.setQueryData<number>(notificationKeys.unreadCount, (old) =>
          Math.max((old ?? 1) - 1, 0),
        );
      }
    },
  });
}

export function useMarkAllRead() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: markAllNotificationsRead,
    onSuccess: () => {
      queryClient.setQueryData<NotificationListResponse>(notificationKeys.recent, (old) =>
        old ? { ...old, items: old.items.map((n) => ({ ...n, is_read: true })) } : old,
      );
      queryClient.setQueryData<number>(notificationKeys.unreadCount, 0);
    },
  });
}
