/**
 * Types for the admin Vendors management screen, matching the flat-JSON
 * shape returned by backend/internal/handler/admin_vendor_handler.go
 * (listVendorRowToResponse/getVendorRowToResponse/createVendorRowToResponse/
 * updateVendorRowToResponse). Mounted at /api/v1/admin/vendors,
 * ADMIN/ADMIN_PARAM-only.
 */

import type { ChangeRequestAccepted } from "../master-data/changeRequest";

export type VendorStatus = "active" | "disabled" | "all";

export interface AdminVendor {
  id: number;
  code: string;
  name: string;
  legal_name: string;
  npwp: string;
  contact_email: string;
  contact_phone: string;
  hq_address: string;
  is_active: boolean;
  deleted_at: string | null;
}

export interface AdminVendorsListParams {
  page: number;
  page_size: number;
  status?: VendorStatus;
  q?: string;
}

export interface AdminVendorsListResponse {
  vendors: AdminVendor[];
  page: number;
  page_size: number;
  total: number;
}

export interface CreateVendorPayload {
  code: string;
  name: string;
  legal_name: string;
  npwp: string;
  contact_email: string;
  contact_phone: string;
  hq_address: string;
}

export interface UpdateVendorPayload {
  code?: string;
  name: string;
  legal_name: string;
  npwp: string;
  contact_email: string;
  contact_phone: string;
  hq_address: string;
}

/** Disable is staged for approval (202); `warning` is set when the vendor still has linked active users. */
export type DisableVendorResponse = ChangeRequestAccepted;

/**
 * Types below match the flat-JSON shape of the four vendor child-entity
 * admin handlers (admin_vendor_branch_handler.go, admin_vendor_vault_handler.go,
 * admin_vendor_pic_handler.go, admin_vendor_package_handler.go), mounted at
 * /api/v1/admin/vendors/{vendorId}/{branches,vaults,pics,packages}. All
 * create/update/disable/enable calls are maker-checker (202 = staged, not saved).
 */

export type VendorChildStatus = "active" | "disabled" | "all";

export interface AdminVendorBranch {
  id: number;
  vendor_id: number;
  branch_code: string;
  branch_name: string;
  location_id: number | null;
  region: string | null;
  category: VendorVaultCategory;
  is_active: boolean;
  deleted_at: string | null;
}

export interface AdminVendorBranchesListParams {
  page: number;
  page_size: number;
  status?: VendorChildStatus;
  q?: string;
}

export interface AdminVendorBranchesListResponse {
  branches: AdminVendorBranch[];
  page: number;
  page_size: number;
  total: number;
}

export interface CreateVendorBranchPayload {
  branch_code: string;
  branch_name: string;
  location_id: number | null;
  region: string | null;
  category: VendorVaultCategory;
}

export interface UpdateVendorBranchPayload {
  branch_name: string;
  location_id: number | null;
  region: string | null;
  category: VendorVaultCategory;
}

export type VendorVaultCategory = "ATM" | "CASH" | "ATM_CASH";

export interface AdminVendorVault {
  id: number;
  vendor_branch_id: number;
  vault_code: string;
  category: VendorVaultCategory;
  currency_code: string;
  min_capacity_amount: string | null;
  max_capacity_amount: string | null;
  latitude: string | null;
  longitude: string | null;
  operating_hours: string | null;
  location_id: number | null;
  is_active: boolean;
  deleted_at: string | null;
}

export interface AdminVendorVaultsListParams {
  page: number;
  page_size: number;
  status?: VendorChildStatus;
  q?: string;
  /** Branch drill-down: omit for all branches under the vendor. */
  branch_id?: number;
}

export interface AdminVendorVaultsListResponse {
  vaults: AdminVendorVault[];
  page: number;
  page_size: number;
  total: number;
}

export interface VendorVaultFormPayload {
  category: VendorVaultCategory;
  currency_code: string;
  min_capacity_amount: string | null;
  max_capacity_amount: string | null;
  latitude: string | null;
  longitude: string | null;
  operating_hours: string | null;
  location_id: number | null;
}

export interface CreateVendorVaultPayload extends VendorVaultFormPayload {
  vendor_branch_id: number;
  vault_code: string;
}

export type UpdateVendorVaultPayload = VendorVaultFormPayload;

export interface AdminVendorPic {
  id: number;
  vendor_id: number;
  vendor_branch_id: number | null;
  name: string;
  position: string | null;
  phone: string | null;
  email: string | null;
  is_notification_recipient: boolean;
  is_active: boolean;
  deleted_at: string | null;
}

export interface AdminVendorPicsListParams {
  page: number;
  page_size: number;
  status?: VendorChildStatus;
  q?: string;
  /** Branch drill-down: omit for all PICs under the vendor. */
  branch_id?: number;
  /** true = only vendor-wide PICs (vendor_branch_id IS NULL). */
  vendor_wide_only?: boolean;
}

export interface AdminVendorPicsListResponse {
  pics: AdminVendorPic[];
  page: number;
  page_size: number;
  total: number;
  /** Non-blocking, e.g. "no active notification recipient". */
  warnings: string[];
}

export interface VendorPicFormPayload {
  vendor_branch_id: number | null;
  name: string;
  position: string | null;
  phone: string | null;
  email: string | null;
  is_notification_recipient: boolean;
}

export type CreateVendorPicPayload = VendorPicFormPayload;
export type UpdateVendorPicPayload = VendorPicFormPayload;

/**
 * A branch's own special/custom package price ("harga khusus cabang",
 * migration 016) -- same field shape as AdminVendorPackagePrice, but always
 * branch-scoped (vendor_branch_id required, not optional) and distinct from
 * it: this is the branch's OWN price, not an override row in the vendor-wide
 * PT/branch/ATM price tree (that stays in vendor_package_prices, shown on
 * the vendor page). Money fields are exact decimal strings, never numbers.
 * No is_active/deleted_at: effective-dated history, not a togglable entity
 * -- "disable" ends validity (effective_end_date), never soft-deletes.
 */
export interface AdminVendorPackage {
  id: number;
  vendor_branch_id: number;
  package_code: string;
  machine_group: "ATM" | "CDM_CRM";
  price_class: "REGULAR" | "VIP_INDUSTRI";
  tier_min: number;
  tier_max: number | null;
  base_price: string | null;
  atm_id: number | null;
  sla_note: string | null;
  currency: string;
  effective_start_date: string;
  effective_end_date: string | null;
}

export interface AdminVendorPackagesListParams {
  page: number;
  page_size: number;
  status?: VendorPackagePriceStatus;
  q?: string;
  /** Branch drill-down: omit for all branches under the vendor. */
  branch_id?: number;
}

export interface AdminVendorPackagesListResponse {
  packages: AdminVendorPackage[];
  page: number;
  page_size: number;
  total: number;
}

/** Editable on Update; also embedded in the create payload. */
export interface VendorPackageContentPayload {
  base_price: string | null;
  sla_note: string | null;
  effective_end_date: string | null;
}

/** Grain fields are immutable after create -- a grain change is a new price period. */
export interface CreateVendorPackagePayload extends VendorPackageContentPayload {
  vendor_branch_id: number;
  package_code: string;
  machine_group: "ATM" | "CDM_CRM";
  price_class: "REGULAR" | "VIP_INDUSTRI";
  tier_min: number;
  tier_max: number | null;
  atm_id: number | null;
  currency: string;
  effective_start_date: string;
}

export type UpdateVendorPackagePayload = VendorPackageContentPayload;

/**
 * Vendor package price (backend/internal/handler/admin_vendor_package_price_handler.go,
 * mounted at /api/v1/admin/vendors/{vendorId}/package-prices). Vendor-scoped
 * directly (not per-branch like AdminVendorPackage above): vendor_branch_id/
 * atm_id are optional overrides on top of the PT-level base row. Money
 * fields are exact decimal strings, never numbers -- never do arithmetic on
 * them in JS, only display. No is_active/deleted_at: a price row is
 * effective-dated history, not a togglable entity -- "disable" ends its
 * validity (effective_end_date), it does not soft-delete the row.
 *
 * Migration 017 split the single identifier: `package` is the shared
 * human-readable label ("PAKET 3") that groups rows; `package_code` is the
 * per-row unique machine code ("PKG3_ABA_001"), server-generated and never
 * collected from the operator.
 */
export interface AdminVendorPackagePrice {
  id: number;
  vendor_id: number;
  package: string; // label, groups rows ("PAKET 3")
  package_code: string; // per-row code ("PKG3_ABA_001")
  machine_group: "ATM" | "CDM_CRM";
  price_class: "REGULAR" | "VIP_INDUSTRI";
  tier_min: number;
  tier_max: number | null;
  base_price: string | null;
  vendor_branch_id: number | null;
  atm_id: number | null;
  sla_note: string | null;
  currency: string;
  effective_start_date: string;
  effective_end_date: string | null;
}

/** 'active' = current or open-ended, 'disabled' = effective_end_date in the past. */
export type VendorPackagePriceStatus = "active" | "disabled" | "all";

export interface AdminVendorPackagePricesListParams {
  page: number;
  page_size: number;
  status?: VendorPackagePriceStatus;
  package?: string;
  machine_group?: string;
  price_class?: string;
}

export interface AdminVendorPackagePricesListResponse {
  package_prices: AdminVendorPackagePrice[];
  page: number;
  page_size: number;
  total: number;
}

/** Editable on Update; also embedded in the create payload. */
export interface VendorPackagePriceContentPayload {
  base_price: string | null;
  sla_note: string | null;
  effective_end_date: string | null;
}

/**
 * Grain fields are immutable after create -- a grain change is a new price
 * period. `package` is the label the operator picks; `package_code` is NOT
 * collected here -- it is generated server-side on approval.
 */
export interface CreateVendorPackagePricePayload extends VendorPackagePriceContentPayload {
  package: string;
  machine_group: "ATM" | "CDM_CRM";
  price_class: "REGULAR" | "VIP_INDUSTRI";
  tier_min: number;
  tier_max: number | null;
  vendor_branch_id: number | null;
  atm_id: number | null;
  currency: string;
  effective_start_date: string;
}

export type UpdateVendorPackagePricePayload = VendorPackagePriceContentPayload;

/**
 * Read-only "ATM" sub-tab on the vendor branch detail page
 * (backend/internal/handler/admin_branch_atm_handler.go, mounted at
 * /api/v1/admin/vendors/{vendorId}/branches/{branchId}/atms). Display-only:
 * no create/update/disable, no maker-checker. An ATM is "managed by" a
 * branch through an active atm_vendor_packages assignment to one of the
 * branch's packages -- location/priority_class are nullable per the LEFT
 * JOIN/nullable source columns.
 */
export interface ManagedATM {
  atm_id: number;
  terminal_id: string;
  location_name: string | null;
  location_city_or_regency: string | null;
  priority_class: string | null;
  is_active: boolean;
  package_code: string;
}

export interface BranchATMsListParams {
  page: number;
  page_size: number;
}

export interface BranchATMsListResponse {
  atms: ManagedATM[];
  page: number;
  page_size: number;
  total: number;
}
