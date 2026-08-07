import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useAuth } from '@/features/auth/useAuth';
import notificationsData from '@/data/notifications.json';
import type { Notification } from '@/lib/types';

export function useNotifications() {
  const { state } = useAuth();
  const vendorId = state.user?.vendorId;

  const query = useQuery({
    queryKey: ['notifications', vendorId],
    queryFn: () => {
      const allNotifications = notificationsData as Notification[];
      return allNotifications.filter((n) => n.vendorId === String(vendorId));
    },
    enabled: !!vendorId,
  });

  const unreadCount =
    query.data?.filter((n) => !n.isRead).length ?? 0;

  return { ...query, unreadCount };
}

export function useMarkAsRead() {
  const queryClient = useQueryClient();
  const { state } = useAuth();
  const vendorId = state.user?.vendorId;

  return useMutation({
    mutationFn: async (notificationId: string) => {
      // Simulate marking as read
      await Promise.resolve();
      return { notificationId, success: true };
    },
    onSuccess: ({ notificationId }) => {
      queryClient.setQueryData<Notification[]>(
        ['notifications', vendorId],
        (old) =>
          old?.map((n) =>
            n.id === notificationId ? { ...n, isRead: true } : n,
          ),
      );
    },
  });
}

export function useMarkAllAsRead() {
  const queryClient = useQueryClient();
  const { state } = useAuth();
  const vendorId = state.user?.vendorId;

  return useMutation({
    mutationFn: async () => {
      // Simulate marking all as read
      await Promise.resolve();
      return { success: true };
    },
    onSuccess: () => {
      queryClient.setQueryData<Notification[]>(
        ['notifications', vendorId],
        (old) => old?.map((n) => ({ ...n, isRead: true })),
      );
    },
  });
}
