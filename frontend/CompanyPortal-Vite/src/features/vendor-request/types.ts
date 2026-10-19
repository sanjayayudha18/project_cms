/**
 * Vendor Request API types (request-replenish-to-vendor spec, design.md
 * API contract). Wire fields stay snake_case to match the backend response
 * verbatim (same convention as dmaa-forecast/types.ts) — no camelCase
 * transform layer.
 */

import type { components } from "@/lib/api/schema";

// BE-10: wire shapes are generated from backend/api/openapi.yaml (`pnpm run gen:api`).
// Field docs live in the spec; only UI-side types below are written by hand.
type S = components["schemas"];

export type VendorRequestStatus = S["VendorRequestStatus"];
/** CIT-2: a Manual_Request's classification; null for legacy / standard rows. */
export type VendorRequestCategory = S["CreateVendorRequestPayload"]["request_category"];
export type PaginationMeta = S["Pagination"];

// -- Forecast Browser (GET /forecast, /forecast/summary, /vendors) ---------
export type ForecastRow = S["ForecastRow"];
export type ForecastResponse = S["ForecastResponse"];
export type ForecastSummaryCounts = Omit<S["ForecastSummaryTotals"], "unassigned_atm_count">;
/** flm_vendor === "" (and flm_vendor_region === "") = ATMs with no active vendor package. */
export type ForecastSummaryGroup = S["ForecastSummaryGroup"];
export type ForecastSummaryResponse = S["ForecastSummaryResponse"];
export type VendorOption = S["VendorOption"];
export type VendorOptionsResponse = S["VendorOptionsResponse"];

/** Result of fetchAllForecastForSelection — the cross-page select-all fetch (Req 2.2, 2.10). */
export interface FetchAllForecastResult {
  rows: ForecastRow[];
  totalCount: number;
  /** True when totalCount exceeds the Vendor Request item cap; rows is empty in that case. */
  exceededCap: boolean;
}

// -- Vendor Request detail / list / audit log -------------------------------
export type UserRef = S["UserRef"];
export type VendorRequestItem = S["VendorRequestItem"];
export type VendorRequestDetail = S["VendorRequestDetail"];
export type RequestAtmStatus = S["RequestAtmStatus"];
export type CompletionResult = S["CompletionResultInput"]["result"];
export type CompletionResultInput = S["CompletionResultInput"];
export type VendorRequestSummary = S["VendorRequestSummary"];
export type VendorRequestListResponse = S["VendorRequestListResponse"];
export type AuditLogEntry = S["AuditLogEntry"];
export type AuditLogResponse = S["AuditLogResponse"];

// -- Request payloads -------------------------------------------------------
export type VendorRequestItemInput = S["VendorRequestItemInput"];
export type CreateVendorRequestPayload = S["CreateVendorRequestPayload"];
export type RejectVendorRequestPayload = S["RejectVendorRequestPayload"];
export type CancelVendorRequestPayload = S["CancelVendorRequestPayload"];

// -- Hook params (camelCase; mapped to query strings in api.ts) -----------

export interface BrowseForecastParams {
  forecastDate: string;
  atmId?: string;
  /** Optional (CIT-2 Req 1.9): "" / omitted = no filter (the "Semua" option). */
  brand?: string;
  /** Optional: "" / omitted = Semua (forecast-browser-summary FR2 relaxed CIT-2 Req 1.4). */
  flmVendor?: string;
  /** Optional: "" / omitted = Semua. */
  flmVendorRegion?: string;
  /** Only rows with no active vendor package; never combined with flmVendor/flmVendorRegion. */
  unassigned?: boolean;
  /** review R1: only rows whose vendor branch has no region; never combined with flmVendorRegion. */
  noRegion?: boolean;
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
  "completion_pending",
  "vault_assignment",
  "vault_review",
  "ready",
  "sent_to_vendor",
  "vendor_accepted",
  "vendor_rejected",
];
