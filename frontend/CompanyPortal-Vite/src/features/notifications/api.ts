/**
 * Notification API (backend /api/v1/notifications, .claude/sdlc/notification
 * spec FR4). The caller is always the logged-in user — no user parameter.
 */

import { api } from "@/lib/api/client";

export interface NotificationItem {
  readonly id: number;
  readonly type: string;
  readonly title: string;
  readonly body: string;
  readonly link: string | null;
  readonly is_read: boolean;
  readonly created_at: string; // RFC 3339, UTC
}

export interface NotificationListResponse {
  readonly items: NotificationItem[];
  readonly total: number;
  readonly page: number;
  readonly page_size: number;
}

const BASE = "/notifications";

export async function fetchNotifications(
  page = 1,
  pageSize = 10,
): Promise<NotificationListResponse> {
  const search = new URLSearchParams({ page: String(page), page_size: String(pageSize) });
  const { data } = await api.get<NotificationListResponse>(`${BASE}?${search}`);
  return data;
}

export async function fetchUnreadCount(): Promise<number> {
  const { data } = await api.get<{ unread_count: number }>(`${BASE}/unread-count`);
  return data.unread_count;
}

export async function markNotificationRead(id: number): Promise<void> {
  await api.post(`${BASE}/${id}/read`);
}

export async function markAllNotificationsRead(): Promise<number> {
  const { data } = await api.post<{ updated: number }>(`${BASE}/read-all`);
  return data.updated;
}
