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
  createVendorBranch,
  createVendorPackage,
  createVendorPackagePrice,
  createVendorPic,
  createVendorVault,
  disableVendor,
  disableVendorBranch,
  disableVendorPackage,
  disableVendorPackagePrice,
  disableVendorPic,
  disableVendorVault,
  enableVendor,
  enableVendorBranch,
  enableVendorPackage,
  enableVendorPic,
  enableVendorVault,
  getVendor,
  getVendorBranch,
  listVendorBranches,
  listVendorChildren,
  listVendorPackagePrices,
  listVendorPackages,
  listVendorPics,
  listVendorVaults,
  listVendors,
  updateVendor,
  updateVendorBranch,
  updateVendorPackagePrice,
  updateVendorPic,
  updateVendorVault,
} from "./api";
import type {
  AdminVendor,
  AdminVendorBranch,
  AdminVendorBranchesListParams,
  AdminVendorBranchesListResponse,
  AdminVendorPackagePricesListParams,
  AdminVendorPackagePricesListResponse,
  AdminVendorPackagesListParams,
  AdminVendorPackagesListResponse,
  AdminVendorPicsListParams,
  AdminVendorPicsListResponse,
  AdminVendorVaultsListParams,
  AdminVendorVaultsListResponse,
  AdminVendorsListParams,
  AdminVendorsListResponse,
  CreateVendorBranchPayload,
  CreateVendorPackagePayload,
  CreateVendorPackagePricePayload,
  CreateVendorPayload,
  CreateVendorPicPayload,
  CreateVendorVaultPayload,
  DisableVendorResponse,
  UpdateVendorBranchPayload,
  UpdateVendorPackagePricePayload,
  UpdateVendorPayload,
  UpdateVendorPicPayload,
  UpdateVendorVaultPayload,
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
  return () => {
    queryClient.invalidateQueries({ queryKey: adminVendorKeys.all });
    // a submitted change is pending: refresh the row badges too
    queryClient.invalidateQueries({ queryKey: ["master-data", "pending"] });
  };
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

// ─── Branches, vaults, PICs, packages: typed list + CRUD mutation hooks ───────
// Mutations reuse useInvalidateList: its ["admin-vendors"] prefix already
// covers every child query key below (TanStack Query matches by key prefix),
// and it also refreshes the master-data "pending" badge set.

export function useVendorBranch(vendorId: number, branchId: number) {
  return useQuery<AdminVendorBranch, ApiError>({
    queryKey: ["admin-vendors", "children", vendorId, "branches", "detail", branchId],
    queryFn: () => getVendorBranch(vendorId, branchId),
  });
}

export function useVendorBranches(vendorId: number, params: AdminVendorBranchesListParams) {
  return useQuery<AdminVendorBranchesListResponse, ApiError>({
    queryKey: ["admin-vendors", "children", vendorId, "branches", params],
    queryFn: () => listVendorBranches(vendorId, params),
    placeholderData: keepPreviousData,
  });
}

export function useCreateVendorBranch(vendorId: number) {
  const invalidate = useInvalidateList();
  return useMutation<ChangeRequestAccepted, ApiError, CreateVendorBranchPayload>({
    mutationFn: (payload) => createVendorBranch(vendorId, payload),
    onSuccess: invalidate,
  });
}

export function useUpdateVendorBranch(vendorId: number) {
  const invalidate = useInvalidateList();
  return useMutation<
    ChangeRequestAccepted,
    ApiError,
    { branchId: number; payload: UpdateVendorBranchPayload }
  >({
    mutationFn: ({ branchId, payload }) => updateVendorBranch(vendorId, branchId, payload),
    onSuccess: invalidate,
  });
}

export function useDisableVendorBranch(vendorId: number) {
  const invalidate = useInvalidateList();
  return useMutation<ChangeRequestAccepted, ApiError, number>({
    mutationFn: (branchId) => disableVendorBranch(vendorId, branchId),
    onSuccess: invalidate,
  });
}

export function useEnableVendorBranch(vendorId: number) {
  const invalidate = useInvalidateList();
  return useMutation<ChangeRequestAccepted, ApiError, number>({
    mutationFn: (branchId) => enableVendorBranch(vendorId, branchId),
    onSuccess: invalidate,
  });
}

export function useVendorVaults(vendorId: number, params: AdminVendorVaultsListParams) {
  return useQuery<AdminVendorVaultsListResponse, ApiError>({
    queryKey: ["admin-vendors", "children", vendorId, "vaults", params],
    queryFn: () => listVendorVaults(vendorId, params),
    placeholderData: keepPreviousData,
  });
}

export function useCreateVendorVault(vendorId: number) {
  const invalidate = useInvalidateList();
  return useMutation<ChangeRequestAccepted, ApiError, CreateVendorVaultPayload>({
    mutationFn: (payload) => createVendorVault(vendorId, payload),
    onSuccess: invalidate,
  });
}

export function useUpdateVendorVault(vendorId: number) {
  const invalidate = useInvalidateList();
  return useMutation<
    ChangeRequestAccepted,
    ApiError,
    { vaultId: number; payload: UpdateVendorVaultPayload }
  >({
    mutationFn: ({ vaultId, payload }) => updateVendorVault(vendorId, vaultId, payload),
    onSuccess: invalidate,
  });
}

export function useDisableVendorVault(vendorId: number) {
  const invalidate = useInvalidateList();
  return useMutation<ChangeRequestAccepted, ApiError, number>({
    mutationFn: (vaultId) => disableVendorVault(vendorId, vaultId),
    onSuccess: invalidate,
  });
}

export function useEnableVendorVault(vendorId: number) {
  const invalidate = useInvalidateList();
  return useMutation<ChangeRequestAccepted, ApiError, number>({
    mutationFn: (vaultId) => enableVendorVault(vendorId, vaultId),
    onSuccess: invalidate,
  });
}

export function useVendorPics(vendorId: number, params: AdminVendorPicsListParams) {
  return useQuery<AdminVendorPicsListResponse, ApiError>({
    queryKey: ["admin-vendors", "children", vendorId, "pics", params],
    queryFn: () => listVendorPics(vendorId, params),
    placeholderData: keepPreviousData,
  });
}

export function useCreateVendorPic(vendorId: number) {
  const invalidate = useInvalidateList();
  return useMutation<ChangeRequestAccepted, ApiError, CreateVendorPicPayload>({
    mutationFn: (payload) => createVendorPic(vendorId, payload),
    onSuccess: invalidate,
  });
}

export function useUpdateVendorPic(vendorId: number) {
  const invalidate = useInvalidateList();
  return useMutation<
    ChangeRequestAccepted,
    ApiError,
    { picId: number; payload: UpdateVendorPicPayload }
  >({
    mutationFn: ({ picId, payload }) => updateVendorPic(vendorId, picId, payload),
    onSuccess: invalidate,
  });
}

export function useDisableVendorPic(vendorId: number) {
  const invalidate = useInvalidateList();
  return useMutation<ChangeRequestAccepted, ApiError, number>({
    mutationFn: (picId) => disableVendorPic(vendorId, picId),
    onSuccess: invalidate,
  });
}

export function useEnableVendorPic(vendorId: number) {
  const invalidate = useInvalidateList();
  return useMutation<ChangeRequestAccepted, ApiError, number>({
    mutationFn: (picId) => enableVendorPic(vendorId, picId),
    onSuccess: invalidate,
  });
}

export function useVendorPackages(vendorId: number, params: AdminVendorPackagesListParams) {
  return useQuery<AdminVendorPackagesListResponse, ApiError>({
    queryKey: ["admin-vendors", "children", vendorId, "packages", params],
    queryFn: () => listVendorPackages(vendorId, params),
    placeholderData: keepPreviousData,
  });
}

export function useCreateVendorPackage(vendorId: number) {
  const invalidate = useInvalidateList();
  return useMutation<ChangeRequestAccepted, ApiError, CreateVendorPackagePayload>({
    mutationFn: (payload) => createVendorPackage(vendorId, payload),
    onSuccess: invalidate,
  });
}

export function useDisableVendorPackage(vendorId: number) {
  const invalidate = useInvalidateList();
  return useMutation<ChangeRequestAccepted, ApiError, number>({
    mutationFn: (packageId) => disableVendorPackage(vendorId, packageId),
    onSuccess: invalidate,
  });
}

export function useEnableVendorPackage(vendorId: number) {
  const invalidate = useInvalidateList();
  return useMutation<ChangeRequestAccepted, ApiError, number>({
    mutationFn: (packageId) => enableVendorPackage(vendorId, packageId),
    onSuccess: invalidate,
  });
}

// ─── Vendor package prices: no Enable (effective-dated history, not a toggle) ─

export function useVendorPackagePrices(
  vendorId: number,
  params: AdminVendorPackagePricesListParams,
) {
  return useQuery<AdminVendorPackagePricesListResponse, ApiError>({
    queryKey: ["admin-vendors", "children", vendorId, "package-prices", params],
    queryFn: () => listVendorPackagePrices(vendorId, params),
    placeholderData: keepPreviousData,
  });
}

export function useCreateVendorPackagePrice(vendorId: number) {
  const invalidate = useInvalidateList();
  return useMutation<ChangeRequestAccepted, ApiError, CreateVendorPackagePricePayload>({
    mutationFn: (payload) => createVendorPackagePrice(vendorId, payload),
    onSuccess: invalidate,
  });
}

export function useUpdateVendorPackagePrice(vendorId: number) {
  const invalidate = useInvalidateList();
  return useMutation<
    ChangeRequestAccepted,
    ApiError,
    { priceId: number; payload: UpdateVendorPackagePricePayload }
  >({
    mutationFn: ({ priceId, payload }) => updateVendorPackagePrice(vendorId, priceId, payload),
    onSuccess: invalidate,
  });
}

export function useDisableVendorPackagePrice(vendorId: number) {
  const invalidate = useInvalidateList();
  return useMutation<ChangeRequestAccepted, ApiError, number>({
    mutationFn: (priceId) => disableVendorPackagePrice(vendorId, priceId),
    onSuccess: invalidate,
  });
}
