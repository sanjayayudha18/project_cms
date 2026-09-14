/**
 * TanStack Query hooks for the Vendor Request feature. Query hooks wrap
 * api.ts's reads with keepPreviousData (same convention as
 * useDmaaForecastData); mutation hooks invalidate the whole
 * ["vendor-requests"] prefix on success (same convention as
 * useEodQueries.ts's useRetryFile) so list/detail/audit-log all refetch.
 */

import type { ApiError } from "@/lib/api/client";
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  approveVendorRequest,
  cancelVendorRequest,
  createVendorRequest,
  fetchAllForecastForSelection,
  fetchForecast,
  fetchVendorRequest,
  fetchVendorRequestAuditLog,
  fetchVendorRequests,
  rejectVendorRequest,
  reviseVendorRequest,
  submitVendorRequest,
  updateVendorRequestItems,
} from "./api";
import type {
  AuditLogResponse,
  BrowseForecastParams,
  CreateVendorRequestPayload,
  FetchAllForecastResult,
  ForecastResponse,
  ListVendorRequestParams,
  VendorRequestDetail,
  VendorRequestItemInput,
  VendorRequestListResponse,
} from "./types";

const VENDOR_REQUEST_STALE_TIME = 30_000;

export const vendorRequestKeys = {
  all: ["vendor-requests"] as const,
  forecast: (params: BrowseForecastParams) => ["vendor-requests", "forecast", params] as const,
  list: (params: ListVendorRequestParams) => ["vendor-requests", "list", params] as const,
  detail: (id: number) => ["vendor-requests", "detail", id] as const,
  auditLog: (id: number) => ["vendor-requests", "audit-log", id] as const,
};

// -- Reads ------------------------------------------------------------------

export function useForecastBrowse(params: BrowseForecastParams, enabled = true) {
  return useQuery<ForecastResponse, ApiError>({
    queryKey: vendorRequestKeys.forecast(params),
    queryFn: () => fetchForecast(params),
    enabled: enabled && params.forecastDate !== "",
    staleTime: VENDOR_REQUEST_STALE_TIME,
    placeholderData: keepPreviousData,
  });
}

/**
 * Cross-page select-all fetch (Req 2.2, 2.8). Disabled by default — call
 * `.refetch()` imperatively on button click, not on every render, since it
 * can issue multiple requests (fetchAllForecastForSelection pages through
 * at page_size=100). `gcTime: 0` so a stale result never lingers to be
 * silently reused for a different date/filter.
 */
export function useForecastSelectAll(
  params: Pick<BrowseForecastParams, "forecastDate" | "atmId">,
  cap: number,
) {
  return useQuery<FetchAllForecastResult, ApiError>({
    queryKey: ["vendor-requests", "forecast-select-all", params, cap],
    queryFn: () => fetchAllForecastForSelection(params, cap),
    enabled: false,
    gcTime: 0,
  });
}

export function useVendorRequests(params: ListVendorRequestParams) {
  return useQuery<VendorRequestListResponse, ApiError>({
    queryKey: vendorRequestKeys.list(params),
    queryFn: () => fetchVendorRequests(params),
    staleTime: VENDOR_REQUEST_STALE_TIME,
    placeholderData: keepPreviousData,
  });
}

export function useVendorRequest(id: number | null) {
  return useQuery<VendorRequestDetail, ApiError>({
    queryKey: vendorRequestKeys.detail(id ?? 0),
    queryFn: () => fetchVendorRequest(id as number),
    enabled: id !== null,
  });
}

export function useVendorRequestAuditLog(id: number | null) {
  return useQuery<AuditLogResponse, ApiError>({
    queryKey: vendorRequestKeys.auditLog(id ?? 0),
    queryFn: () => fetchVendorRequestAuditLog(id as number),
    enabled: id !== null,
  });
}

// -- Mutations ----------------------------------------------------------

/** Shared onSuccess: refetch every vendor-request query (list/detail/audit-log alike). */
function useInvalidateOnSuccess() {
  const queryClient = useQueryClient();
  return () => queryClient.invalidateQueries({ queryKey: vendorRequestKeys.all });
}

export function useCreateVendorRequest() {
  const invalidate = useInvalidateOnSuccess();
  return useMutation<VendorRequestDetail, ApiError, CreateVendorRequestPayload>({
    mutationFn: createVendorRequest,
    onSuccess: invalidate,
  });
}

export function useUpdateVendorRequestItems() {
  const invalidate = useInvalidateOnSuccess();
  return useMutation<
    VendorRequestDetail,
    ApiError,
    { id: number; items: VendorRequestItemInput[] }
  >({
    mutationFn: ({ id, items }) => updateVendorRequestItems(id, items),
    onSuccess: invalidate,
  });
}

export function useSubmitVendorRequest() {
  const invalidate = useInvalidateOnSuccess();
  return useMutation<VendorRequestDetail, ApiError, number>({
    mutationFn: submitVendorRequest,
    onSuccess: invalidate,
  });
}

export function useApproveVendorRequest() {
  const invalidate = useInvalidateOnSuccess();
  return useMutation<VendorRequestDetail, ApiError, number>({
    mutationFn: approveVendorRequest,
    onSuccess: invalidate,
  });
}

export function useRejectVendorRequest() {
  const invalidate = useInvalidateOnSuccess();
  return useMutation<VendorRequestDetail, ApiError, { id: number; reason: string }>({
    mutationFn: ({ id, reason }) => rejectVendorRequest(id, reason),
    onSuccess: invalidate,
  });
}

export function useReviseVendorRequest() {
  const invalidate = useInvalidateOnSuccess();
  return useMutation<VendorRequestDetail, ApiError, number>({
    mutationFn: reviseVendorRequest,
    onSuccess: invalidate,
  });
}

export function useCancelVendorRequest() {
  const invalidate = useInvalidateOnSuccess();
  return useMutation<VendorRequestDetail, ApiError, number>({
    mutationFn: cancelVendorRequest,
    onSuccess: invalidate,
  });
}

/** Extracts a display message from an unknown error (thrown ApiError or Error). */
export function getVendorRequestErrorMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  if (
    typeof error === "object" &&
    error !== null &&
    "message" in error &&
    typeof (error as { message: unknown }).message === "string"
  ) {
    return (error as { message: string }).message;
  }
  return "Terjadi kesalahan yang tidak diketahui";
}
