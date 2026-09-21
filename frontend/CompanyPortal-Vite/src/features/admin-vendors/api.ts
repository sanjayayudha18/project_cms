/**
 * Admin Vendors API client (backend/internal/handler/admin_vendor_handler.go,
 * mounted at /api/v1/admin/vendors). Paths here are relative to
 * apiConfig.baseURL ("/api/v1"), same convention as admin-users/api.ts.
 */

import { api } from "@/lib/api/client";
import type { ChangeRequestAccepted } from "../master-data/changeRequest";
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

// Writes are maker-checker (plan.md D1): they answer 202 with the staged change
// request, not the saved vendor. The list only changes once it is approved.
export async function createVendor(payload: CreateVendorPayload): Promise<ChangeRequestAccepted> {
  const { data } = await api.post<ChangeRequestAccepted>(BASE, payload);
  return data;
}

export async function updateVendor(
  id: number,
  payload: UpdateVendorPayload,
): Promise<ChangeRequestAccepted> {
  const { data } = await api.put<ChangeRequestAccepted>(`${BASE}/${id}`, payload);
  return data;
}

export async function disableVendor(id: number): Promise<DisableVendorResponse> {
  const { data } = await api.post<DisableVendorResponse>(`${BASE}/${id}/disable`);
  return data;
}

export async function enableVendor(id: number): Promise<ChangeRequestAccepted> {
  const { data } = await api.post<ChangeRequestAccepted>(`${BASE}/${id}/enable`);
  return data;
}

// Read-only child lists of a vendor (backend admin_vendor_{branch,vault,pic,package}_handler.go).
export type VendorChildKind = "branches" | "vaults" | "pics" | "packages";

export async function listVendorChildren(
  vendorId: number,
  kind: VendorChildKind,
): Promise<Record<string, unknown>[]> {
  const { data } = await api.get<Record<string, unknown>>(
    `${BASE}/${vendorId}/${kind}?page=1&page_size=100&status=all`,
  );
  return (data[kind] as Record<string, unknown>[]) ?? [];
}
