/**
 * TanStack Query hooks for the admin Users screen. List hook uses
 * keepPreviousData so pagination/filter changes don't flash a loading
 * state (same convention as vendor-request's useVendorRequests); mutations
 * invalidate only the list key on success (design.md convention, matching
 * rbac-settings/hooks/useRbacQueries.ts).
 */

import type { ApiError } from "@/lib/api/client";
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createUser, disableUser, enableUser, getUser, listUsers, updateUser } from "./api";
import type {
  AdminUser,
  AdminUsersListParams,
  AdminUsersListResponse,
  CreateUserPayload,
  UpdateUserPayload,
} from "./types";

export const adminUserKeys = {
  all: ["admin-users"] as const,
  list: (params: AdminUsersListParams) => [...adminUserKeys.all, "list", params] as const,
  detail: (id: number) => [...adminUserKeys.all, "detail", id] as const,
};

export function useUsersList(params: AdminUsersListParams) {
  return useQuery<AdminUsersListResponse, ApiError>({
    queryKey: adminUserKeys.list(params),
    queryFn: () => listUsers(params),
    placeholderData: keepPreviousData,
  });
}

export function useUser(id: number | null) {
  return useQuery<AdminUser, ApiError>({
    queryKey: adminUserKeys.detail(id ?? 0),
    queryFn: () => getUser(id as number),
    enabled: id !== null,
  });
}

function useInvalidateList() {
  const queryClient = useQueryClient();
  return () => queryClient.invalidateQueries({ queryKey: adminUserKeys.all });
}

export function useCreateUser() {
  const invalidate = useInvalidateList();
  return useMutation<AdminUser, ApiError, CreateUserPayload>({
    mutationFn: createUser,
    onSuccess: invalidate,
  });
}

export function useUpdateUser() {
  const invalidate = useInvalidateList();
  return useMutation<AdminUser, ApiError, { id: number; payload: UpdateUserPayload }>({
    mutationFn: ({ id, payload }) => updateUser(id, payload),
    onSuccess: invalidate,
  });
}

export function useDisableUser() {
  const invalidate = useInvalidateList();
  return useMutation<{ message: string }, ApiError, number>({
    mutationFn: disableUser,
    onSuccess: invalidate,
  });
}

export function useEnableUser() {
  const invalidate = useInvalidateList();
  return useMutation<{ message: string }, ApiError, number>({
    mutationFn: enableUser,
    onSuccess: invalidate,
  });
}
