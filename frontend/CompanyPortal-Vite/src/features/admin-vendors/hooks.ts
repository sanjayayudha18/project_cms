/**
 * TanStack Query hooks for the admin Vendors screen. Same conventions as
 * admin-users/hooks.ts: keepPreviousData on the list, mutations invalidate
 * only this feature's query keys on success.
 */

import type { ApiError } from "@/lib/api/client";
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { ChangeRequestAccepted } from "../master-data/changeRequest";
import {
  type VendorChildKind,
  createVendor,
  disableVendor,
  enableVendor,
  getVendor,
  listVendorChildren,
  listVendors,
  updateVendor,
} from "./api";
import type {
  AdminVendor,
  AdminVendorsListParams,
  AdminVendorsListResponse,
  CreateVendorPayload,
  DisableVendorResponse,
  UpdateVendorPayload,
} from "./types";

export const adminVendorKeys = {
  all: ["admin-vendors"] as const,
  list: (params: AdminVendorsListParams) => [...adminVendorKeys.all, "list", params] as const,
  detail: (id: number) => [...adminVendorKeys.all, "detail", id] as const,
};

export function useVendorsList(params: AdminVendorsListParams) {
  return useQuery<AdminVendorsListResponse, ApiError>({
    queryKey: adminVendorKeys.list(params),
    queryFn: () => listVendors(params),
    placeholderData: keepPreviousData,
  });
}

export function useVendor(id: number | null) {
  return useQuery<AdminVendor, ApiError>({
    queryKey: adminVendorKeys.detail(id ?? 0),
    queryFn: () => getVendor(id as number),
    enabled: id !== null,
  });
}

function useInvalidateList() {
  const queryClient = useQueryClient();
  return () => queryClient.invalidateQueries({ queryKey: adminVendorKeys.all });
}

export function useCreateVendor() {
  const invalidate = useInvalidateList();
  return useMutation<ChangeRequestAccepted, ApiError, CreateVendorPayload>({
    mutationFn: createVendor,
    onSuccess: invalidate,
  });
}

export function useUpdateVendor() {
  const invalidate = useInvalidateList();
  return useMutation<ChangeRequestAccepted, ApiError, { id: number; payload: UpdateVendorPayload }>(
    {
      mutationFn: ({ id, payload }) => updateVendor(id, payload),
      onSuccess: invalidate,
    },
  );
}

export function useDisableVendor() {
  const invalidate = useInvalidateList();
  return useMutation<DisableVendorResponse, ApiError, number>({
    mutationFn: disableVendor,
    onSuccess: invalidate,
  });
}

export function useEnableVendor() {
  const invalidate = useInvalidateList();
  return useMutation<ChangeRequestAccepted, ApiError, number>({
    mutationFn: enableVendor,
    onSuccess: invalidate,
  });
}

export function useVendorChildren(vendorId: number, kind: VendorChildKind) {
  return useQuery({
    queryKey: ["admin-vendors", "children", vendorId, kind],
    queryFn: () => listVendorChildren(vendorId, kind),
  });
}
