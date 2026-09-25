/**
 * Admin regions API client (backend/internal/handler/admin_region_handler.go,
 * mounted at /api/v1/admin/regions). Paths here are relative to
 * apiConfig.baseURL ("/api/v1"), same convention as admin-atms/api.ts.
 * Writes apply immediately and return the region row (200/201), not a
 * pending change request -- no maker-checker for this feature.
 */

import { api } from "@/lib/api/client";
import type {
  AdminRegion,
  AdminRegionsListParams,
  AdminRegionsListResponse,
  CreateRegionPayload,
  UpdateRegionPayload,
} from "./types";

const BASE = "/admin/regions";

function toQueryString(params: AdminRegionsListParams): string {
  const q = new URLSearchParams();
  q.set("page", String(params.page));
  q.set("page_size", String(params.page_size));
  if (params.status) q.set("status", params.status);
  if (params.q) q.set("q", params.q);
  return q.toString();
}

export async function listRegions(
  params: AdminRegionsListParams,
): Promise<AdminRegionsListResponse> {
  const { data } = await api.get<AdminRegionsListResponse>(`${BASE}?${toQueryString(params)}`);
  return data;
}

export async function getRegion(id: number): Promise<AdminRegion> {
  const { data } = await api.get<AdminRegion>(`${BASE}/${id}`);
  return data;
}

export async function createRegion(payload: CreateRegionPayload): Promise<AdminRegion> {
  const { data } = await api.post<AdminRegion>(BASE, payload);
  return data;
}

export async function updateRegionName(
  id: number,
  payload: UpdateRegionPayload,
): Promise<AdminRegion> {
  const { data } = await api.put<AdminRegion>(`${BASE}/${id}`, payload);
  return data;
}

export async function disableRegion(id: number): Promise<AdminRegion> {
  const { data } = await api.post<AdminRegion>(`${BASE}/${id}/disable`);
  return data;
}

export async function enableRegion(id: number): Promise<AdminRegion> {
  const { data } = await api.post<AdminRegion>(`${BASE}/${id}/enable`);
  return data;
}
