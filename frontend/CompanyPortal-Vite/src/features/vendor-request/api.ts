/**
 * Vendor Request API client calls (design.md API contract). Thin wrappers
 * around `api` (src/lib/api/client.ts) — one function per endpoint, no
 * business logic. Consumed by hooks.ts.
 */

import { api } from "@/lib/api/client";
import type {
  AuditLogResponse,
  BrowseForecastParams,
  CreateVendorRequestPayload,
  FetchAllForecastResult,
  ForecastResponse,
  ForecastRow,
  ListVendorRequestParams,
  VendorRequestDetail,
  VendorRequestItemInput,
  VendorRequestListResponse,
} from "./types";

const BASE = "/vendor-requests";

// The forecast endpoint rejects page_size > 100 (backend/internal/service/
// vendor_request_actions.go: BrowseForecast validation) — it does not clamp,
// it 400s. So a select-all fetch must page at this size, not request 1000
// rows in one call.
const SELECT_ALL_PAGE_SIZE = 100;

function buildForecastQuery(params: BrowseForecastParams): string {
  const search = new URLSearchParams({ forecast_date: params.forecastDate });
  if (params.atmId) search.set("atm_id", params.atmId);
  search.set("page", String(params.page ?? 1));
  search.set("page_size", String(params.pageSize ?? 20));
  return search.toString();
}

export async function fetchForecast(params: BrowseForecastParams): Promise<ForecastResponse> {
  const { data } = await api.get<ForecastResponse>(
    `${BASE}/forecast?${buildForecastQuery(params)}`,
  );
  return data;
}

/**
 * Fetches every forecast row matching the date + ATM filter, across all
 * pages, for the cross-page "Pilih Semua Rekomendasi" select-all (Req 2.2).
 * The first page's `total_count` tells us the full match size up front, so
 * when it exceeds `cap` this returns immediately with `exceededCap: true`
 * and no rows — the caller must not apply a partial selection (Req 2.10:
 * inform, never silently truncate). Only pages through the rest when the
 * full set fits under the cap.
 */
export async function fetchAllForecastForSelection(
  params: Pick<BrowseForecastParams, "forecastDate" | "atmId">,
  cap: number,
): Promise<FetchAllForecastResult> {
  const first = await fetchForecast({ ...params, page: 1, pageSize: SELECT_ALL_PAGE_SIZE });
  const totalCount = first.pagination.total_count;
  if (totalCount > cap) {
    return { rows: [], totalCount, exceededCap: true };
  }

  const rows: ForecastRow[] = [...first.data];
  const totalPages = first.pagination.total_pages;
  for (let page = 2; page <= totalPages; page++) {
    const next = await fetchForecast({ ...params, page, pageSize: SELECT_ALL_PAGE_SIZE });
    rows.push(...next.data);
  }
  return { rows, totalCount, exceededCap: false };
}

function buildListQuery(params: ListVendorRequestParams): string {
  const search = new URLSearchParams();
  if (params.status && params.status.length > 0) search.set("status", params.status.join(","));
  if (params.forecastDate) search.set("forecast_date", params.forecastDate);
  if (params.createdBy) search.set("created_by", String(params.createdBy));
  if (params.requestNumber) search.set("request_number", params.requestNumber);
  search.set("page", String(params.page ?? 1));
  search.set("page_size", String(params.pageSize ?? 10));
  return search.toString();
}

export async function fetchVendorRequests(
  params: ListVendorRequestParams,
): Promise<VendorRequestListResponse> {
  const { data } = await api.get<VendorRequestListResponse>(`${BASE}?${buildListQuery(params)}`);
  return data;
}

export async function fetchVendorRequest(id: number): Promise<VendorRequestDetail> {
  const { data } = await api.get<VendorRequestDetail>(`${BASE}/${id}`);
  return data;
}

export async function fetchVendorRequestAuditLog(id: number): Promise<AuditLogResponse> {
  const { data } = await api.get<AuditLogResponse>(`${BASE}/${id}/audit-log`);
  return data;
}

export async function createVendorRequest(
  payload: CreateVendorRequestPayload,
): Promise<VendorRequestDetail> {
  const { data } = await api.post<VendorRequestDetail>(BASE, payload);
  return data;
}

export async function updateVendorRequestItems(
  id: number,
  items: VendorRequestItemInput[],
): Promise<VendorRequestDetail> {
  const { data } = await api.put<VendorRequestDetail>(`${BASE}/${id}/items`, { items });
  return data;
}

export async function submitVendorRequest(id: number): Promise<VendorRequestDetail> {
  const { data } = await api.post<VendorRequestDetail>(`${BASE}/${id}/submit`);
  return data;
}

export async function approveVendorRequest(id: number): Promise<VendorRequestDetail> {
  const { data } = await api.post<VendorRequestDetail>(`${BASE}/${id}/approve`);
  return data;
}

export async function rejectVendorRequest(
  id: number,
  reason: string,
): Promise<VendorRequestDetail> {
  const { data } = await api.post<VendorRequestDetail>(`${BASE}/${id}/reject`, {
    rejection_reason: reason,
  });
  return data;
}

export async function reviseVendorRequest(id: number): Promise<VendorRequestDetail> {
  const { data } = await api.post<VendorRequestDetail>(`${BASE}/${id}/revise`);
  return data;
}

export async function cancelVendorRequest(id: number): Promise<VendorRequestDetail> {
  const { data } = await api.post<VendorRequestDetail>(`${BASE}/${id}/cancel`);
  return data;
}
