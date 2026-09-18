/**
 * Role Management API types (design.md API contract) and form schemas.
 *
 * All endpoints live under /admin/roles (backend/internal/handler
 * role_mgmt_handler.go, ATM backend flat JSON — not pkg/response's envelope).
 */

import { z } from "zod";

// ─── Entities ──────────────────────────────────────────────────────────────

export interface CatalogEntry {
  id: number;
  parent_id: number | null;
  key: string;
  label: string;
  kind: "menu" | "feature";
  sort_order: number;
}

/** One granted catalog entry, as embedded in RoleWithPermissions.permissions. */
export interface GrantedPermission {
  menu_feature_id: number;
  parent_id: number | null;
  key: string;
  label: string;
  kind: "menu" | "feature";
}

export interface RoleWithPermissions {
  id: number;
  role: string;
  description: string | null;
  permissions: GrantedPermission[];
}

export interface RolesResponse {
  roles: RoleWithPermissions[];
}

export interface CatalogResponse {
  catalog: CatalogEntry[];
}

export interface CreatedRole {
  id: number;
  role: string;
  description: string | null;
}

export interface UpdateRolePermissionsResult {
  role_id: number;
  permissions: number[];
}

// ─── Form schemas ────────────────────────────────────────────────────────────

/**
 * Mirrors backend/internal/rolemgmt/service.go's roleNameRe exactly
 * (^[A-Z0-9-]+$) so client-side rejection matches the server (Req 7.3).
 */
export const createRoleSchema = z.object({
  role: z
    .string()
    .min(1, "Nama peran wajib diisi")
    .regex(/^[A-Z0-9-]+$/, "Gunakan huruf kapital, angka, dan tanda hubung"),
  description: z.string().optional(),
});

export type CreateRoleFormValues = z.infer<typeof createRoleSchema>;

export const permissionsSchema = z.object({
  menu_feature_ids: z.array(z.number().int().positive()),
});

export type PermissionsFormValues = z.infer<typeof permissionsSchema>;
