/**
 * Admin Vendors API client (backend/internal/handler/admin_vendor_handler.go,
 * mounted at /api/v1/admin/vendors). Paths here are relative to
 * apiConfig.baseURL ("/api/v1"), same convention as admin-users/api.ts.
 */

import { api } from "@/lib/api/client";
import type { ChangeRequestAccepted } from "../master-data/changeRequest";
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
  VendorChildStatus,
} from "./types";

const BASE = "/admin/vendors";

function toQueryString(params: AdminVendorsListParams): string {
  const q = new URLSearchParams();
  q.set("page", String(params.page));
  q.set("page_size", String(params.page_size));
  if (params.status) q.set("status", params.status);
  if (params.q) q.set("q", params.q);
  return q.toString();
}

export async function listVendors(
  params: AdminVendorsListParams,
): Promise<AdminVendorsListResponse> {
  const { data } = await api.get<AdminVendorsListResponse>(`${BASE}?${toQueryString(params)}`);
  return data;
}

export async function getVendor(id: number): Promise<AdminVendor> {
  const { data } = await api.get<AdminVendor>(`${BASE}/${id}`);
  return data;
}

// Writes are maker-checker (plan.md D1): they answer 202 with the staged change
// request, not the saved vendor. The list only changes once it is approved.
export async function createVendor(payload: CreateVendorPayload): Promise<ChangeRequestAccepted> {
  const { data } = await api.post<ChangeRequestAccepted>(BASE, payload);
  return data;
}

export async function updateVendor(
  id: number,
  payload: UpdateVendorPayload,
): Promise<ChangeRequestAccepted> {
  const { data } = await api.put<ChangeRequestAccepted>(`${BASE}/${id}`, payload);
  return data;
}

export async function disableVendor(id: number): Promise<DisableVendorResponse> {
  const { data } = await api.post<DisableVendorResponse>(`${BASE}/${id}/disable`);
  return data;
}

export async function enableVendor(id: number): Promise<ChangeRequestAccepted> {
  const { data } = await api.post<ChangeRequestAccepted>(`${BASE}/${id}/enable`);
  return data;
}

// Read-only child lists of a vendor (backend admin_vendor_{branch,vault,pic,package}_handler.go).
export type VendorChildKind = "branches" | "vaults" | "pics" | "packages";

export async function listVendorChildren(
  vendorId: number,
  kind: VendorChildKind,
): Promise<Record<string, unknown>[]> {
  const { data } = await api.get<Record<string, unknown>>(
    `${BASE}/${vendorId}/${kind}?page=1&page_size=100&status=all`,
  );
  return (data[kind] as Record<string, unknown>[]) ?? [];
}

// ─── Vendor branches, vaults, PICs, packages: typed CRUD ──────────────────────
// Mirrors the vendor-level CRUD above; every write is maker-checker (202 =
// staged change request). Mounted at /admin/vendors/{vendorId}/{kind}.

function appendCommonParams(
  q: URLSearchParams,
  params: { page: number; page_size: number; status?: VendorChildStatus; q?: string },
): void {
  q.set("page", String(params.page));
  q.set("page_size", String(params.page_size));
  if (params.status) q.set("status", params.status);
  if (params.q) q.set("q", params.q);
}

export async function getVendorBranch(
  vendorId: number,
  branchId: number,
): Promise<AdminVendorBranch> {
  const { data } = await api.get<AdminVendorBranch>(`${BASE}/${vendorId}/branches/${branchId}`);
  return data;
}

export async function listVendorBranches(
  vendorId: number,
  params: AdminVendorBranchesListParams,
): Promise<AdminVendorBranchesListResponse> {
  const q = new URLSearchParams();
  appendCommonParams(q, params);
  const { data } = await api.get<AdminVendorBranchesListResponse>(
    `${BASE}/${vendorId}/branches?${q.toString()}`,
  );
  return data;
}

export async function createVendorBranch(
  vendorId: number,
  payload: CreateVendorBranchPayload,
): Promise<ChangeRequestAccepted> {
  const { data } = await api.post<ChangeRequestAccepted>(`${BASE}/${vendorId}/branches`, payload);
  return data;
}

export async function updateVendorBranch(
  vendorId: number,
  branchId: number,
  payload: UpdateVendorBranchPayload,
): Promise<ChangeRequestAccepted> {
  const { data } = await api.put<ChangeRequestAccepted>(
    `${BASE}/${vendorId}/branches/${branchId}`,
    payload,
  );
  return data;
}

export async function disableVendorBranch(
  vendorId: number,
  branchId: number,
): Promise<ChangeRequestAccepted> {
  const { data } = await api.post<ChangeRequestAccepted>(
    `${BASE}/${vendorId}/branches/${branchId}/disable`,
  );
  return data;
}

export async function enableVendorBranch(
  vendorId: number,
  branchId: number,
): Promise<ChangeRequestAccepted> {
  const { data } = await api.post<ChangeRequestAccepted>(
    `${BASE}/${vendorId}/branches/${branchId}/enable`,
  );
  return data;
}

export async function listVendorVaults(
  vendorId: number,
  params: AdminVendorVaultsListParams,
): Promise<AdminVendorVaultsListResponse> {
  const q = new URLSearchParams();
  appendCommonParams(q, params);
  if (params.branch_id) q.set("branch_id", String(params.branch_id));
  const { data } = await api.get<AdminVendorVaultsListResponse>(
    `${BASE}/${vendorId}/vaults?${q.toString()}`,
  );
  return data;
}

export async function createVendorVault(
  vendorId: number,
  payload: CreateVendorVaultPayload,
): Promise<ChangeRequestAccepted> {
  const { data } = await api.post<ChangeRequestAccepted>(`${BASE}/${vendorId}/vaults`, payload);
  return data;
}

export async function updateVendorVault(
  vendorId: number,
  vaultId: number,
  payload: UpdateVendorVaultPayload,
): Promise<ChangeRequestAccepted> {
  const { data } = await api.put<ChangeRequestAccepted>(
    `${BASE}/${vendorId}/vaults/${vaultId}`,
    payload,
  );
  return data;
}

export async function disableVendorVault(
  vendorId: number,
  vaultId: number,
): Promise<ChangeRequestAccepted> {
  const { data } = await api.post<ChangeRequestAccepted>(
    `${BASE}/${vendorId}/vaults/${vaultId}/disable`,
  );
  return data;
}

export async function enableVendorVault(
  vendorId: number,
  vaultId: number,
): Promise<ChangeRequestAccepted> {
  const { data } = await api.post<ChangeRequestAccepted>(
    `${BASE}/${vendorId}/vaults/${vaultId}/enable`,
  );
  return data;
}

export async function listVendorPics(
  vendorId: number,
  params: AdminVendorPicsListParams,
): Promise<AdminVendorPicsListResponse> {
  const q = new URLSearchParams();
  appendCommonParams(q, params);
  if (params.branch_id) q.set("branch_id", String(params.branch_id));
  if (params.vendor_wide_only) q.set("vendor_wide_only", "true");
  const { data } = await api.get<AdminVendorPicsListResponse>(
    `${BASE}/${vendorId}/pics?${q.toString()}`,
  );
  return data;
}

export async function createVendorPic(
  vendorId: number,
  payload: CreateVendorPicPayload,
): Promise<ChangeRequestAccepted> {
  const { data } = await api.post<ChangeRequestAccepted>(`${BASE}/${vendorId}/pics`, payload);
  return data;
}

export async function updateVendorPic(
  vendorId: number,
  picId: number,
  payload: UpdateVendorPicPayload,
): Promise<ChangeRequestAccepted> {
  const { data } = await api.put<ChangeRequestAccepted>(
    `${BASE}/${vendorId}/pics/${picId}`,
    payload,
  );
  return data;
}

export async function disableVendorPic(
  vendorId: number,
  picId: number,
): Promise<ChangeRequestAccepted> {
  const { data } = await api.post<ChangeRequestAccepted>(
    `${BASE}/${vendorId}/pics/${picId}/disable`,
  );
  return data;
}

export async function enableVendorPic(
  vendorId: number,
  picId: number,
): Promise<ChangeRequestAccepted> {
  const { data } = await api.post<ChangeRequestAccepted>(
    `${BASE}/${vendorId}/pics/${picId}/enable`,
  );
  return data;
}

export async function listVendorPackages(
  vendorId: number,
  params: AdminVendorPackagesListParams,
): Promise<AdminVendorPackagesListResponse> {
  const q = new URLSearchParams();
  appendCommonParams(q, params);
  if (params.branch_id) q.set("branch_id", String(params.branch_id));
  const { data } = await api.get<AdminVendorPackagesListResponse>(
    `${BASE}/${vendorId}/packages?${q.toString()}`,
  );
  return data;
}

export async function createVendorPackage(
  vendorId: number,
  payload: CreateVendorPackagePayload,
): Promise<ChangeRequestAccepted> {
  const { data } = await api.post<ChangeRequestAccepted>(`${BASE}/${vendorId}/packages`, payload);
  return data;
}

export async function disableVendorPackage(
  vendorId: number,
  packageId: number,
): Promise<ChangeRequestAccepted> {
  const { data } = await api.post<ChangeRequestAccepted>(
    `${BASE}/${vendorId}/packages/${packageId}/disable`,
  );
  return data;
}

export async function enableVendorPackage(
  vendorId: number,
  packageId: number,
): Promise<ChangeRequestAccepted> {
  const { data } = await api.post<ChangeRequestAccepted>(
    `${BASE}/${vendorId}/packages/${packageId}/enable`,
  );
  return data;
}

// ─── Vendor package prices: vendor-scoped, no Enable (effective-dated history) ─

export async function listVendorPackagePrices(
  vendorId: number,
  params: AdminVendorPackagePricesListParams,
): Promise<AdminVendorPackagePricesListResponse> {
  const q = new URLSearchParams();
  appendCommonParams(q, params);
  if (params.package_code) q.set("package_code", params.package_code);
  if (params.machine_group) q.set("machine_group", params.machine_group);
  if (params.price_class) q.set("price_class", params.price_class);
  const { data } = await api.get<AdminVendorPackagePricesListResponse>(
    `${BASE}/${vendorId}/package-prices?${q.toString()}`,
  );
  return data;
}

export async function createVendorPackagePrice(
  vendorId: number,
  payload: CreateVendorPackagePricePayload,
): Promise<ChangeRequestAccepted> {
  const { data } = await api.post<ChangeRequestAccepted>(
    `${BASE}/${vendorId}/package-prices`,
    payload,
  );
  return data;
}

export async function updateVendorPackagePrice(
  vendorId: number,
  priceId: number,
  payload: UpdateVendorPackagePricePayload,
): Promise<ChangeRequestAccepted> {
  const { data } = await api.put<ChangeRequestAccepted>(
    `${BASE}/${vendorId}/package-prices/${priceId}`,
    payload,
  );
  return data;
}

export async function disableVendorPackagePrice(
  vendorId: number,
  priceId: number,
): Promise<ChangeRequestAccepted> {
  const { data } = await api.post<ChangeRequestAccepted>(
    `${BASE}/${vendorId}/package-prices/${priceId}/disable`,
  );
  return data;
}
