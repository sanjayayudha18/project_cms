/**
 * Vendor Request API types (request-replenish-to-vendor spec, design.md
 * API contract). Wire fields stay snake_case to match the backend response
 * verbatim (same convention as dmaa-forecast/types.ts) — no camelCase
 * transform layer.
 */

export type VendorRequestStatus =
  | "draft"
  | "pending_approval"
  | "approved"
  | "rejected"
  | "processing"
  | "completed"
  | "failed"
  | "cancelled";

export interface PaginationMeta {
  page: number;
  page_size: number;
  total_count: number;
  total_pages: number;
}

// -- Forecast Browser (GET /forecast) --------------------------------------

export interface ForecastRow {
  terminal_id: string;
  /** ISO date "YYYY-MM-DD" */
  periode_pred: string;
  denom: number;
  amount_replenish: number;
  amount_refund: number;
  dmaa_file_id: number;
}

export interface ForecastResponse {
  data: ForecastRow[];
  pagination: PaginationMeta;
}

// -- Vendor Request detail / list ------------------------------------------

export interface UserRef {
  id: number;
  full_name: string;
}

export interface VendorRequestItem {
  id: number;
  terminal_id: string;
  /** ISO date "YYYY-MM-DD" */
  periode_pred: string;
  denom: number;
  amount_replenish: number;
  amount_refund: number;
}

export interface VendorRequestDetail {
  id: number;
  request_number: string;
  /** ISO date "YYYY-MM-DD" */
  forecast_date: string;
  status: VendorRequestStatus;
  notes: string;
  created_by: UserRef;
  approved_by: UserRef | null;
  rejected_by: UserRef | null;
  rejection_reason: string;
  /** RFC3339 timestamp */
  created_at: string;
  updated_at: string;
  submitted_at: string | null;
  approved_at: string | null;
  rejected_at: string | null;
  items: VendorRequestItem[];
  total_amount: number;
}

export interface VendorRequestSummary {
  id: number;
  request_number: string;
  forecast_date: string;
  status: VendorRequestStatus;
  notes: string;
  item_count: number;
  total_amount: number;
  created_by: UserRef;
  approved_by: UserRef | null;
  created_at: string;
  submitted_at: string | null;
  approved_at: string | null;
  rejected_at: string | null;
}

export interface VendorRequestListResponse {
  data: VendorRequestSummary[];
  pagination: PaginationMeta;
}

// -- Audit log (GET /{id}/audit-log) ---------------------------------------

export interface AuditLogEntry {
  id: number;
  entity_type: string;
  entity_id: number;
  action: string;
  performed_by: number;
  /** RFC3339 timestamp */
  performed_at: string;
  previous_state: string | null;
  new_state: string | null;
  metadata: Record<string, unknown>;
}

export interface AuditLogResponse {
  data: AuditLogEntry[];
}

// -- Request payloads -------------------------------------------------------

export interface VendorRequestItemInput {
  terminal_id: string;
  /** ISO date "YYYY-MM-DD" */
  periode_pred: string;
  denom: number;
  amount_replenish: number;
}

export interface CreateVendorRequestPayload {
  forecast_date: string;
  notes?: string;
  items: VendorRequestItemInput[];
}

export interface RejectVendorRequestPayload {
  rejection_reason: string;
}

// -- Hook params (camelCase; mapped to query strings in api.ts) -----------

export interface BrowseForecastParams {
  forecastDate: string;
  atmId?: string;
  page?: number;
  pageSize?: number;
}

export interface ListVendorRequestParams {
  status?: VendorRequestStatus[];
  forecastDate?: string;
  createdBy?: number;
  requestNumber?: string;
  page?: number;
  pageSize?: number;
}

export const VENDOR_REQUEST_STATUSES: VendorRequestStatus[] = [
  "draft",
  "pending_approval",
  "approved",
  "rejected",
  "processing",
  "completed",
  "failed",
  "cancelled",
];
