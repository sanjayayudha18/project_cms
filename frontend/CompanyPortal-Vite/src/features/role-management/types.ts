/**
 * Role Management API types (design.md API contract) and form schemas.
 *
 * All endpoints live under /admin/roles (backend/internal/handler
 * role_mgmt_handler.go, ATM backend flat JSON — not pkg/response's envelope).
 */

import { z } from "zod";

// ─── Entities ──────────────────────────────────────────────────────────────

/** Portal a role/catalog entry belongs to (tabs: main = Portal Utama, vendor = Portal Cabang). */
export type Portal = "main" | "vendor";
export type PartyScope = "replenish" | "vault";

export interface CatalogEntry {
  id: number;
  portal: Portal;
  parent_id: number | null;
  key: string;
  label: string;
  kind: "menu" | "feature";
  sort_order: number;
  /** Roles allowed to hold this key; null/absent = every internal role (migration 046). */
  assignable_roles?: string[] | null;
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
  portal: Portal;
  /** Non-empty for vendor roles, null for main roles. */
  party_scope: PartyScope[] | null;
  active_user_count: number;
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
  portal?: Portal;
  party_scope?: PartyScope[] | null;
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

/**
 * POST /admin/roles body. `portal` defaults to "main" in api.createRole so the
 * pre-revamp dialog keeps working until task 6.3; vendor roles must pass
 * `party_scope` (non-empty subset of replenish/vault), main roles must not.
 */
export type CreateRoleRequest = CreateRoleFormValues & {
  portal?: Portal;
  party_scope?: PartyScope[];
};

export const permissionsSchema = z.object({
  menu_feature_ids: z.array(z.number().int().positive()),
});

export type PermissionsFormValues = z.infer<typeof permissionsSchema>;
