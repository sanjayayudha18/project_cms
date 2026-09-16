/**
 * Types for the admin Vendors management screen, matching the flat-JSON
 * shape returned by backend/internal/handler/admin_vendor_handler.go
 * (listVendorRowToResponse/getVendorRowToResponse/createVendorRowToResponse/
 * updateVendorRowToResponse). Mounted at /api/v1/admin/vendors,
 * ADMIN/ADMIN_PARAM-only.
 */

export type VendorStatus = "active" | "disabled" | "all";

export interface AdminVendor {
  id: number;
  code: string;
  name: string;
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
  contact_email: string;
  contact_phone: string;
  hq_address: string;
}

export interface UpdateVendorPayload {
  code?: string;
  name: string;
  contact_email: string;
  contact_phone: string;
  hq_address: string;
}

export interface DisableVendorResponse {
  message: string;
  warning?: string;
  linked_active_users?: number;
}
