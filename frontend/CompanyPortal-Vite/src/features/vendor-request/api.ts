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
  ForecastResponse,
  ListVendorRequestParams,
  VendorRequestDetail,
  VendorRequestItemInput,
  VendorRequestListResponse,
} from "./types";

const BASE = "/vendor-requests";

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
