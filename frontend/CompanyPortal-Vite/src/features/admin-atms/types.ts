/**
 * Types for the admin ATMs management screen, matching the flat-JSON shape
 * returned by backend/internal/handler/admin_atm_handler.go (atmToResponse).
 * Mounted at /api/v1/admin/atms, ADMIN/ADMIN_PARAM-only.
 */

export type ATMStatus = "active" | "disabled" | "all";

export type PriorityClass = "VIP" | "Non VIP" | "Industri";

export interface AdminATM {
  id: number;
  terminal_id: string;
  location_id: number;
  location_name: string | null;
  machine_type: string;
  brand: string;
  model: string;
  operation_hours: string;
  deployment_type: string;
  capacity_amount: string | null;
  low_threshold_amount: string | null;
  critical_threshold_amount: string | null;
  blacklisted: boolean;
  escrow_account: string | null;
  priority_class: PriorityClass | null;
  is_active: boolean;
  created_at: string | null;
  updated_at: string | null;
  deleted_at: string | null;
}

export interface AdminATMsListParams {
  page: number;
  page_size: number;
  status?: ATMStatus;
  q?: string;
  brand?: string;
  machine_type?: string;
  deployment_type?: string;
  priority_class?: PriorityClass;
  location_id?: number;
}

export interface AdminATMsListResponse {
  atms: AdminATM[];
  page: number;
  page_size: number;
  total: number;
}

export interface CreateATMPayload {
  terminal_id: string;
  location_id: number;
  machine_type: string;
  brand: string;
  model: string;
  operation_hours: string;
  deployment_type: string;
  capacity_amount?: string | null;
  low_threshold_amount?: string | null;
  critical_threshold_amount?: string | null;
  blacklisted: boolean;
  escrow_account?: string | null;
  priority_class?: PriorityClass | null;
}

export interface UpdateATMPayload {
  terminal_id?: string;
  location_id: number;
  machine_type: string;
  brand: string;
  model: string;
  operation_hours: string;
  deployment_type: string;
  capacity_amount?: string | null;
  low_threshold_amount?: string | null;
  critical_threshold_amount?: string | null;
  blacklisted: boolean;
  escrow_account?: string | null;
  priority_class?: PriorityClass | null;
}

export interface LocationOption {
  id: number;
  name: string;
  city_or_regency: string;
  province: string;
  region_id: number;
  region_name: string;
}

export interface LocationOptionsResponse {
  locations: LocationOption[];
}
