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

/**
 * CIT-2 (cit-vendor-request-enhancements spec): a Manual_Request's
 * classification, governing its allowed Replenish_Date (Req 3). null for
 * legacy rows and for the standard, non-manual Forecast-Browser flow.
 */
export type VendorRequestCategory = "planned" | "emergency" | "additional";

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
  /** Empty string when the ATM has no matching master-data row. */
  lokasi_atm: string;
  /** Empty string when the ATM has no matching master-data row. */
  brand: string;
  /** Empty string when the ATM has no active vendor package. */
  flm_vendor: string;
  /** Empty string when the ATM has no active vendor package. */
  flm_vendor_region: string;
  /**
   * replenishment-request-enhancements (Req 5.2): ATM priority tier
   * (atms.priority_class). Empty string when unset.
   */
  priority_class: string;
  /**
   * The ATM's single active vendor package code as of periode_pred, chosen
   * deterministically when more than one is active (Req 5.3, 5.4). Empty
   * string when the ATM has no active package.
   */
  paket: string;
  /**
   * Decimal string (never a float, Req 5.10); null when the terminal has no
   * itm_replenish row (Req 5.11 — render "-", not "0").
   */
  escrow: string | null;
}

export interface ForecastResponse {
  data: ForecastRow[];
  pagination: PaginationMeta;
}

/** Result of fetchAllForecastForSelection — the cross-page select-all fetch (Req 2.2, 2.10). */
export interface FetchAllForecastResult {
  rows: ForecastRow[];
  totalCount: number;
  /** True when totalCount exceeds the Vendor Request item cap; rows is empty in that case. */
  exceededCap: boolean;
}

// -- Vendor options (GET /vendors) ------------------------------------------
// New endpoint added alongside the Forecast Browser filters (CIT-2 Req 1.2,
// 1.3, 3 Q2): no existing vendor-listing endpoint existed for the required
// FLM Vendor / FLM Vendor Region selects or the manual-request vendor select
// to call.

export interface VendorOption {
  id: number;
  name: string;
}

export interface VendorOptionsResponse {
  vendors: VendorOption[];
  regions: string[];
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
  /** ISO date "YYYY-MM-DD"; null only for legacy rows (CIT-2 Req 2, 5.10). */
  replenish_date: string | null;
  /** null for legacy rows and for the standard, non-manual flow (CIT-2 Req 3). */
  request_category: VendorRequestCategory | null;
  /** Soft-cancel flag; the row is never deleted (CIT-2 Req 5). */
  is_canceled: boolean;
  /** True when items were entered directly without a matching DMAA row (CIT-2 Req 3). */
  is_manual: boolean;
  /**
   * replenishment-request-enhancements (Req 3.11 Opsi B): the reason
   * captured on cancel. null for non-canceled rows and for rows canceled
   * before this column existed (their reason lives only in the audit log).
   */
  cancellation_reason: string | null;
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
  replenish_date: string | null;
  request_category: VendorRequestCategory | null;
  is_canceled: boolean;
  is_manual: boolean;
  /** See VendorRequestDetail.cancellation_reason (Req 3.11 Opsi B). */
  cancellation_reason: string | null;
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
  /**
   * ISO date "YYYY-MM-DD". Required for a DMAA-backed item; omit for a
   * Manual_Request item (CIT-2 Req 3, Q5) — the backend anchors it to
   * replenish_date since a manual item has no DMAA periode.
   */
  periode_pred?: string;
  denom: number;
  amount_replenish: number;
  /** Manual_Request items only (CIT-2 Req 3, Q4); omitted for DMAA-backed items. */
  brand?: string;
  lokasi_atm?: string;
}

export interface CreateVendorRequestPayload {
  /**
   * Required for the standard, non-manual flow; omit for a Manual_Request
   * (CIT-2 Q5) — the backend anchors forecast_date to the creation date.
   */
  forecast_date?: string;
  notes?: string;
  items: VendorRequestItemInput[];
  /** ISO date "YYYY-MM-DD", always required (CIT-2 Req 2, 3). */
  replenish_date: string;
  /**
   * replenishment-request-enhancements (Req 1.9): required for EVERY
   * create, DMAA-backed or manual alike — not just is_manual creates as
   * CIT-2 originally scoped it. Kept optional in the type since the wire
   * contract doesn't reject an omitted value at this layer; the create UI
   * (VendorRequestCreate.tsx) enforces the user picks one before submit.
   */
  request_category?: VendorRequestCategory;
  /** True for a Manual_Request (CIT-2 Req 3); defaults to the standard flow when omitted. */
  is_manual?: boolean;
  /** The single CIT vendor this request is constrained to (CIT-2 Req 4, Q2). */
  vendor_id: number;
}

export interface RejectVendorRequestPayload {
  rejection_reason: string;
}

/** replenishment-request-enhancements (Req 3.2, 3.3): 1-500 non-whitespace chars, same bound as reject. */
export interface CancelVendorRequestPayload {
  cancellation_reason: string;
}

// -- Hook params (camelCase; mapped to query strings in api.ts) -----------

export interface BrowseForecastParams {
  forecastDate: string;
  atmId?: string;
  /** Optional (CIT-2 Req 1.9): "" / omitted = no filter (the "Semua" option). */
  brand?: string;
  /** Required, no ALL option (CIT-2 Req 1.2, 1.4) — the page blocks the fetch until this is set. */
  flmVendor: string;
  /** Required, no ALL option (CIT-2 Req 1.3, 1.4) — the page blocks the fetch until this is set. */
  flmVendorRegion: string;
  page?: number;
  pageSize?: number;
}

export interface ListVendorRequestParams {
  status?: VendorRequestStatus[];
  forecastDate?: string;
  createdBy?: number;
  requestNumber?: string;
  /** Default false excludes is_canceled rows (CIT-2 Req 5.5). */
  includeCanceled?: boolean;
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
