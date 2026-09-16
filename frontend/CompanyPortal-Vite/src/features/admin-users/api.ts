/**
 * Admin Users API client (backend/internal/handler/admin_user_handler.go,
 * mounted at /api/v1/admin/users). Paths here are relative to
 * apiConfig.baseURL ("/api/v1"), same convention as rbac-settings/api.ts.
 */

import { api } from "@/lib/api/client";
import type {
  AdminUser,
  AdminUsersListParams,
  AdminUsersListResponse,
  CreateUserPayload,
  UpdateUserPayload,
} from "./types";

const BASE = "/admin/users";

function toQueryString(params: AdminUsersListParams): string {
  const q = new URLSearchParams();
  q.set("page", String(params.page));
  q.set("page_size", String(params.page_size));
  if (params.status) q.set("status", params.status);
  if (params.vendor_id !== undefined) q.set("vendor_id", String(params.vendor_id));
  if (params.role) q.set("role", params.role);
  if (params.q) q.set("q", params.q);
  return q.toString();
}

export async function listUsers(params: AdminUsersListParams): Promise<AdminUsersListResponse> {
  const { data } = await api.get<AdminUsersListResponse>(`${BASE}?${toQueryString(params)}`);
  return data;
}

export async function getUser(id: number): Promise<AdminUser> {
  const { data } = await api.get<AdminUser>(`${BASE}/${id}`);
  return data;
}

export async function createUser(payload: CreateUserPayload): Promise<AdminUser> {
  const { data } = await api.post<AdminUser>(BASE, payload);
  return data;
}

export async function updateUser(id: number, payload: UpdateUserPayload): Promise<AdminUser> {
  const { data } = await api.put<AdminUser>(`${BASE}/${id}`, payload);
  return data;
}

export async function disableUser(id: number): Promise<{ message: string }> {
  const { data } = await api.post<{ message: string }>(`${BASE}/${id}/disable`);
  return data;
}

export async function enableUser(id: number): Promise<{ message: string }> {
  const { data } = await api.post<{ message: string }>(`${BASE}/${id}/enable`);
  return data;
}
