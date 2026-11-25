/**
 * TanStack Query hooks for Role Management. Create/update mutations
 * invalidate roleKeys.roles() on success so the role list (with its embedded
 * permissions) refreshes (design.md "hooks/useRoleQueries.ts").
 */

import type { ApiError } from "@/lib/api/client";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createRole, getCatalog, getRoles, updateRolePermissions } from "../api";
import type {
  CatalogEntry,
  CatalogResponse,
  CreateRoleRequest,
  CreatedRole,
  Portal,
  RoleWithPermissions,
  RolesResponse,
  UpdateRolePermissionsResult,
} from "../types";

export const roleKeys = {
  all: ["role-mgmt"] as const,
  roles: () => [...roleKeys.all, "roles"] as const,
  catalog: () => [...roleKeys.all, "catalog"] as const,
};

// ─── Reads ─────────────────────────────────────────────────────────────────

export function useRoles() {
  return useQuery<RolesResponse, ApiError>({
    queryKey: roleKeys.roles(),
    queryFn: getRoles,
  });
}

export function useCatalog() {
  return useQuery<CatalogResponse, ApiError>({
    queryKey: roleKeys.catalog(),
    queryFn: getCatalog,
  });
}

// The API returns every portal in one response; split client-side so each tab
// loads only its own catalog (Req 2.4, 5.3). Shares the cache with useCatalog.
export function useCatalogForPortal(portal: Portal) {
  return useQuery<CatalogResponse, ApiError, CatalogEntry[]>({
    queryKey: roleKeys.catalog(),
    queryFn: getCatalog,
    select: (d) => d.catalog.filter((e) => e.portal === portal),
  });
}

/** Roles of one portal; `service` roles (EXT-CLIENT) never appear in a tab (Req 2.6). */
export function useRolesForPortal(portal: Portal) {
  return useQuery<RolesResponse, ApiError, RoleWithPermissions[]>({
    queryKey: roleKeys.roles(),
    queryFn: getRoles,
    select: (d) => d.roles.filter((r) => r.portal === portal),
  });
}

// ─── Writes ────────────────────────────────────────────────────────────────

export function useCreateRole() {
  const queryClient = useQueryClient();
  return useMutation<CreatedRole, ApiError, CreateRoleRequest>({
    mutationFn: createRole,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: roleKeys.roles() }),
  });
}

export function useUpdateRolePermissions() {
  const queryClient = useQueryClient();
  return useMutation<
    UpdateRolePermissionsResult,
    ApiError,
    { id: number; menuFeatureIds: number[] }
  >({
    mutationFn: ({ id, menuFeatureIds }) => updateRolePermissions(id, menuFeatureIds),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: roleKeys.roles() }),
  });
}
