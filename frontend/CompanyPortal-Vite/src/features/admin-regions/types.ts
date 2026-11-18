/**
 * Types for the admin regions management screen, matching the flat-JSON
 * shape returned by backend/internal/handler/admin_region_handler.go
 * (regionToResponse/listRegionRowToResponse/getRegionRowToResponse).
 * Mounted at /api/v1/admin/regions, ADMIN/ADMIN_PARAM-only.
 *
 * Unlike admin-atms/admin-vendors, region mutations apply immediately (no
 * maker-checker, design.md "Documented Deviation") -- create/update/disable/
 * enable return the region row directly, not a pending change request.
 */

import { z } from "zod";

export type RegionStatus = "active" | "inactive" | "all";

export interface AdminRegion {
  id: number;
  code: string;
  region: string | null;
  is_active: boolean;
  /** Active ATMs whose location is in this region (region-standardization Req 6.1). */
  atm_count: number;
  /** Active vendor branches whose region_code is this region's code (Req 6.1). */
  vendor_branch_count: number;
  created_at: string | null;
  updated_at: string | null;
  deleted_at: string | null;
}

export interface AdminRegionsListParams {
  page: number;
  page_size: number;
  status?: RegionStatus;
  q?: string;
}

export interface AdminRegionsListResponse {
  regions: AdminRegion[];
  page: number;
  page_size: number;
  total: number;
}

export interface CreateRegionPayload {
  code: string;
  region: string;
}

export interface UpdateRegionPayload {
  code?: string;
  region: string;
}

// code: 2-10 chars after trim, letters/digits only (mirrors regionCodeRe in
// backend/internal/service/region_admin.go, same rule as vendor branch
// region_code -- region-standardization Req 1.3; the backend upper-cases).
// region (display name): 1-100 chars after trim.
export const createRegionSchema = z.object({
  code: z
    .string()
    .trim()
    .min(1, "Kode wajib diisi")
    .min(2, "Minimal 2 karakter")
    .max(10, "Maksimal 10 karakter")
    .regex(/^[A-Za-z0-9]+$/, "Hanya huruf dan angka"),
  region: z.string().trim().min(1, "Nama wajib diisi").max(100, "Maksimal 100 karakter"),
});

// code is not editable (immutable after create, Req 3.2) -- the edit form
// disables the field and never submits a changed value.
export const updateRegionSchema = z.object({
  region: z.string().trim().min(1, "Nama wajib diisi").max(100, "Maksimal 100 karakter"),
});

export type CreateRegionFormValues = z.infer<typeof createRegionSchema>;
export type UpdateRegionFormValues = z.infer<typeof updateRegionSchema>;
