/**
 * Admin Vendors API client (backend/internal/handler/admin_vendor_handler.go,
 * mounted at /api/v1/admin/vendors). Paths here are relative to
 * apiConfig.baseURL ("/api/v1"), same convention as admin-users/api.ts.
 */

import { api } from "@/lib/api/client";
import type {
  AdminVendor,
  AdminVendorsListParams,
  AdminVendorsListResponse,
  CreateVendorPayload,
  DisableVendorResponse,
  UpdateVendorPayload,
} from "./types";

const BASE = "/admin/vendors";

function toQueryString(params: AdminVendorsListParams): string {
  const q = new URLSearchParams();
  q.set("page", String(params.page));
  q.set("page_size", String(params.page_size));
  if (params.status) q.set("status", params.status);
  if (params.q) q.set("q", params.q);
  return q.toString();
}

export async function listVendors(
  params: AdminVendorsListParams,
): Promise<AdminVendorsListResponse> {
  const { data } = await api.get<AdminVendorsListResponse>(`${BASE}?${toQueryString(params)}`);
  return data;
}

export async function getVendor(id: number): Promise<AdminVendor> {
  const { data } = await api.get<AdminVendor>(`${BASE}/${id}`);
  return data;
}

export async function createVendor(payload: CreateVendorPayload): Promise<AdminVendor> {
  const { data } = await api.post<AdminVendor>(BASE, payload);
  return data;
}

export async function updateVendor(id: number, payload: UpdateVendorPayload): Promise<AdminVendor> {
  const { data } = await api.put<AdminVendor>(`${BASE}/${id}`, payload);
  return data;
}

export async function disableVendor(id: number): Promise<DisableVendorResponse> {
  const { data } = await api.post<DisableVendorResponse>(`${BASE}/${id}/disable`);
  return data;
}

export async function enableVendor(id: number): Promise<{ message: string }> {
  const { data } = await api.post<{ message: string }>(`${BASE}/${id}/enable`);
  return data;
}
