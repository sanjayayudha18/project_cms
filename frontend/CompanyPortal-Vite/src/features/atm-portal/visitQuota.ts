/**
 * Kuota kunjungan replenish per ATM (.claude/sdlc/atm-visit-quota spec FR5):
 * types, API calls and TanStack Query hooks for /atm-visit-quotas. Used by the
 * ATM profile card and the Forecast Browser summary (reset per vendor).
 */

import type { ApiError } from "@/lib/api/client";
import { api } from "@/lib/api/client";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

const BASE = "/atm-visit-quotas";

/** ATM-SPV / BRANCH-ATM-SPV / ADMIN may reset kuota and cancel visits. */
export const VISIT_QUOTA_CHECKER_ROLES = ["ADMIN", "ATM-SPV", "BRANCH-ATM-SPV"];

export interface AtmVisit {
  id: number;
  vendor_request_id: number;
  request_number: string;
  /** false = counted without a kuota row (paket tidak dikenali). */
  quota_known: boolean;
  is_over_quota: boolean;
  /** RFC3339 timestamp */
  created_at: string;
  cancelled_at: string | null;
  cancelled_by_name: string | null;
  cancel_reason: string | null;
}

export interface AtmVisitQuota {
  terminal_id: string;
  /** false = the ATM has no kuota row yet (no counted visit, never reset). */
  has_quota: boolean;
  package_code: string;
  quota_total: number;
  /** May be negative (= kelebihan kuota). */
  remaining: number;
  sisa: number;
  kelebihan: number;
  reset_at: string | null;
  reset_by: { id: number; full_name: string } | null;
  /** Today's active package and its cr_frequency — what a reset would set; null = tidak dikenali. */
  current_package_code: string | null;
  current_quota: number | null;
  /** Visits of the current period (since the last reset), newest first. */
  visits: AtmVisit[];
}

export interface VendorQuotaResetResult {
  reset_count: number;
  skipped_count: number;
}

/** remaining -> what the screen shows: sisa never below 0, the rest is kelebihan. */
export function visitSisa(remaining: number): { sisa: number; kelebihan: number } {
  return remaining < 0 ? { sisa: 0, kelebihan: -remaining } : { sisa: remaining, kelebihan: 0 };
}

const quotaKey = (terminalId: string) => ["atm-visit-quota", terminalId] as const;

export function useAtmVisitQuota(terminalId: string) {
  return useQuery<AtmVisitQuota, ApiError>({
    queryKey: quotaKey(terminalId),
    queryFn: async () =>
      (await api.get<AtmVisitQuota>(`${BASE}/atms/${encodeURIComponent(terminalId)}`)).data,
    enabled: terminalId !== "",
    retry: false,
  });
}

/** Kuota changes also move the Forecast Browser's sisa column (vendor-requests prefix). */
function useInvalidateQuota() {
  const queryClient = useQueryClient();
  return async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ["atm-visit-quota"] }),
      queryClient.invalidateQueries({ queryKey: ["vendor-requests"] }),
    ]);
  };
}

export function useResetAtmQuota() {
  const invalidate = useInvalidateQuota();
  return useMutation<AtmVisitQuota, ApiError, string>({
    mutationFn: async (terminalId) =>
      (await api.post<AtmVisitQuota>(`${BASE}/atms/${encodeURIComponent(terminalId)}/reset`)).data,
    onSuccess: invalidate,
  });
}

export function useResetVendorQuota() {
  const invalidate = useInvalidateQuota();
  return useMutation<VendorQuotaResetResult, ApiError, number>({
    mutationFn: async (vendorId) =>
      (await api.post<VendorQuotaResetResult>(`${BASE}/vendors/${vendorId}/reset`)).data,
    onSuccess: invalidate,
  });
}

export function useCancelAtmVisit() {
  const invalidate = useInvalidateQuota();
  return useMutation<unknown, ApiError, { visitId: number; reason: string }>({
    mutationFn: async ({ visitId, reason }) =>
      (await api.post(`${BASE}/visits/${visitId}/cancel`, { reason })).data,
    onSuccess: invalidate,
  });
}
