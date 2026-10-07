import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  type VendorNotification,
  fetchNotifications,
  fetchUnreadCount,
  markAllNotificationsRead,
  markNotificationRead,
} from "./api";

// ponytail: the page shows the newest 100 only, no pagination UI; add paging
// when vendors actually accumulate more than that within the 90-day retention.
const PAGE_SIZE = 100;
const UNREAD_POLL_MS = 60_000;

const listKey = ["notifications", "list"] as const;
const unreadKey = ["notifications", "unread-count"] as const;

/** The newest notifications of the logged-in vendor user (server-scoped). */
export function useNotifications() {
  return useQuery({
    queryKey: listKey,
    queryFn: async () => (await fetchNotifications(1, PAGE_SIZE)).items,
  });
}

/** Unread count for the sidebar badge, polled every 60 s (spec decision 3). */
export function useUnreadCount() {
  return useQuery({
    queryKey: unreadKey,
    queryFn: fetchUnreadCount,
    refetchInterval: UNREAD_POLL_MS,
  });
}

// After a write the cache is updated in place, not refetched: list and count
// come from the read replica, which may lag the primary (spec FR4.5).

export function useMarkAsRead() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: markNotificationRead,
    onSuccess: (_data, id) => {
      const wasUnread =
        queryClient
          .getQueryData<VendorNotification[]>(listKey)
          ?.some((n) => n.id === id && !n.is_read) ?? false;
      queryClient.setQueryData<VendorNotification[]>(listKey, (old) =>
        old?.map((n) => (n.id === id ? { ...n, is_read: true } : n)),
      );
      if (wasUnread) {
        queryClient.setQueryData<number>(unreadKey, (old) => Math.max((old ?? 1) - 1, 0));
      }
    },
  });
}

export function useMarkAllAsRead() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: markAllNotificationsRead,
    onSuccess: () => {
      queryClient.setQueryData<VendorNotification[]>(listKey, (old) =>
        old?.map((n) => ({ ...n, is_read: true })),
      );
      queryClient.setQueryData<number>(unreadKey, 0);
    },
  });
}
