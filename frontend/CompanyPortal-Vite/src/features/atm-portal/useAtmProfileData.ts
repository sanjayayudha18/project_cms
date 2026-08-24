/**
 * TanStack Query hooks for the ATM Profile page: master data header +
 * per-tab paginated history (replenish/cashpos). Follows useAtmPortalData.ts's
 * fetch-fn + useQuery pattern.
 */

import type { ApiError } from "@/lib/api/client";
import { api } from "@/lib/api/client";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import type {
  AtmCashposResponse,
  AtmProfileHistoryParams,
  AtmProfileMasterData,
  AtmReplenishResponse,
} from "./types";

const MASTER_STALE_TIME = 5 * 60 * 1000; // 5 min — master data changes infrequently
const HISTORY_STALE_TIME = 2 * 60 * 1000; // 2 min

/**
 * Retries once, but never for a 4xx response — a 404 (unknown terminal ID)
 * or 400 (bad params) will never succeed on retry, so retrying only delays
 * the not-found/error UI from showing (and, if the query happens to land on
 * a retry backoff exactly when the browser's online/offline state flickers,
 * can leave the query stuck in a "paused" fetchStatus that never resolves).
 */
function retryUnlessClientError(failureCount: number, error: ApiError): boolean {
  if (error.status >= 400 && error.status < 500) {
    return false;
  }
  return failureCount < 1;
}

function buildHistoryQueryString(params: AtmProfileHistoryParams): string {
  return new URLSearchParams({
    page: String(params.page),
    page_size: String(params.page_size),
    date_from: params.date_from,
    date_to: params.date_to,
  }).toString();
}

async function fetchAtmMasterData(terminalId: string): Promise<AtmProfileMasterData> {
  const { data } = await api.get<AtmProfileMasterData>(`/atm-portal/atms/${terminalId}`);
  return data;
}

async function fetchAtmReplenishHistory(
  terminalId: string,
  params: AtmProfileHistoryParams,
): Promise<AtmReplenishResponse> {
  const { data } = await api.get<AtmReplenishResponse>(
    `/atm-portal/atms/${terminalId}/replenish?${buildHistoryQueryString(params)}`,
  );
  return data;
}

async function fetchAtmCashposHistory(
  terminalId: string,
  params: AtmProfileHistoryParams,
): Promise<AtmCashposResponse> {
  const { data } = await api.get<AtmCashposResponse>(
    `/atm-portal/atms/${terminalId}/cashpos?${buildHistoryQueryString(params)}`,
  );
  return data;
}

export function useAtmMasterData(terminalId: string) {
  return useQuery<AtmProfileMasterData, ApiError>({
    queryKey: ["atm-profile", "master", terminalId],
    queryFn: () => fetchAtmMasterData(terminalId),
    staleTime: MASTER_STALE_TIME,
    retry: retryUnlessClientError,
  });
}

export function useAtmReplenishHistory(
  terminalId: string,
  params: AtmProfileHistoryParams,
  enabled: boolean,
) {
  return useQuery<AtmReplenishResponse, ApiError>({
    queryKey: ["atm-profile", "replenish", terminalId, params],
    queryFn: () => fetchAtmReplenishHistory(terminalId, params),
    staleTime: HISTORY_STALE_TIME,
    retry: retryUnlessClientError,
    placeholderData: keepPreviousData,
    enabled,
  });
}

export function useAtmCashposHistory(
  terminalId: string,
  params: AtmProfileHistoryParams,
  enabled: boolean,
) {
  return useQuery<AtmCashposResponse, ApiError>({
    queryKey: ["atm-profile", "cashpos", terminalId, params],
    queryFn: () => fetchAtmCashposHistory(terminalId, params),
    staleTime: HISTORY_STALE_TIME,
    retry: retryUnlessClientError,
    placeholderData: keepPreviousData,
    enabled,
  });
}
