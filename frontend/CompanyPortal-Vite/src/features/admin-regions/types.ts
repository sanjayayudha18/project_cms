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
  location_count: number;
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

// code: 1-20 chars after trim, alphanumeric only (mirrors regionCodeRe in
// backend/internal/service/region_admin.go). region (display name): 1-100
// chars after trim.
export const createRegionSchema = z.object({
  code: z
    .string()
    .trim()
    .min(1, "Kode wajib diisi")
    .max(20, "Maksimal 20 karakter")
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
