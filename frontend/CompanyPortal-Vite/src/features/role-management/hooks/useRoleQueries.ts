/**
 * TanStack Query hooks for Role Management. Create/update mutations
 * invalidate roleKeys.roles() on success so the role list (with its embedded
 * permissions) refreshes (design.md "hooks/useRoleQueries.ts").
 */

import type { ApiError } from "@/lib/api/client";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createRole, getCatalog, getRoles, updateRolePermissions } from "../api";
import type {
  CatalogResponse,
  CreateRoleFormValues,
  CreatedRole,
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

// ─── Writes ────────────────────────────────────────────────────────────────

export function useCreateRole() {
  const queryClient = useQueryClient();
  return useMutation<CreatedRole, ApiError, CreateRoleFormValues>({
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
