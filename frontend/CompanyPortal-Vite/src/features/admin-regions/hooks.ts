/**
 * TanStack Query hooks for the admin regions screen. Same conventions as
 * admin-atms/hooks.ts: keepPreviousData on the list, mutations invalidate
 * this feature's query keys on success. No pending/master-data invalidation
 * (no maker-checker for regions).
 */

import type { ApiError } from "@/lib/api/client";
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  createRegion,
  disableRegion,
  enableRegion,
  getRegion,
  listRegions,
  updateRegionName,
} from "./api";
import type {
  AdminRegion,
  AdminRegionsListParams,
  AdminRegionsListResponse,
  CreateRegionPayload,
  UpdateRegionPayload,
} from "./types";

export const adminRegionsKeys = {
  all: ["admin-regions"] as const,
  list: (params: AdminRegionsListParams) => [...adminRegionsKeys.all, "list", params] as const,
  detail: (id: number) => [...adminRegionsKeys.all, "detail", id] as const,
};

export function useRegionsList(params: AdminRegionsListParams) {
  return useQuery<AdminRegionsListResponse, ApiError>({
    queryKey: adminRegionsKeys.list(params),
    queryFn: () => listRegions(params),
    placeholderData: keepPreviousData,
  });
}

export function useRegion(id: number | null) {
  return useQuery<AdminRegion, ApiError>({
    queryKey: adminRegionsKeys.detail(id ?? 0),
    queryFn: () => getRegion(id as number),
    enabled: id !== null,
  });
}

function useInvalidateList() {
  const queryClient = useQueryClient();
  return () => queryClient.invalidateQueries({ queryKey: adminRegionsKeys.all });
}

export function useCreateRegion() {
  const invalidate = useInvalidateList();
  return useMutation<AdminRegion, ApiError, CreateRegionPayload>({
    mutationFn: createRegion,
    onSuccess: invalidate,
  });
}

export function useUpdateRegion() {
  const invalidate = useInvalidateList();
  return useMutation<AdminRegion, ApiError, { id: number; payload: UpdateRegionPayload }>({
    mutationFn: ({ id, payload }) => updateRegionName(id, payload),
    onSuccess: invalidate,
  });
}

export function useDisableRegion() {
  const invalidate = useInvalidateList();
  return useMutation<AdminRegion, ApiError, number>({
    mutationFn: disableRegion,
    onSuccess: invalidate,
  });
}

export function useEnableRegion() {
  const invalidate = useInvalidateList();
  return useMutation<AdminRegion, ApiError, number>({
    mutationFn: enableRegion,
    onSuccess: invalidate,
  });
}
