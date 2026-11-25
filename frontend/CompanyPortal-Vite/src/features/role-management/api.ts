/**
 * Role Management API client (backend/internal/handler/role_mgmt_handler.go).
 * All paths are relative to apiConfig.baseURL ("/api/v1"), so "/admin/roles/..."
 * here maps to "/api/v1/admin/roles/..." on the wire.
 */

import { api } from "@/lib/api/client";
import type {
  CatalogResponse,
  CreateRoleRequest,
  CreatedRole,
  RolesResponse,
  UpdateRolePermissionsResult,
} from "./types";

const BASE = "/admin/roles";

// ─── Reads ─────────────────────────────────────────────────────────────────

export async function getRoles(): Promise<RolesResponse> {
  const { data } = await api.get<RolesResponse>(BASE);
  return data;
}

export async function getCatalog(): Promise<CatalogResponse> {
  const { data } = await api.get<CatalogResponse>(`${BASE}/catalog`);
  return data;
}

// ─── Writes ────────────────────────────────────────────────────────────────

export async function createRole(values: CreateRoleRequest): Promise<CreatedRole> {
  const { data } = await api.post<CreatedRole>(BASE, { portal: "main", ...values });
  return data;
}

export async function updateRolePermissions(
  id: number,
  menuFeatureIds: number[],
): Promise<UpdateRolePermissionsResult> {
  const { data } = await api.put<UpdateRolePermissionsResult>(`${BASE}/${id}/permissions`, {
    menu_feature_ids: menuFeatureIds,
  });
  return data;
}
