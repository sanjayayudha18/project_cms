/**
 * TanStack Query hook for GET /api/v1/atm-portal/atms.
 *
 * Filter/sort/page state is owned by the caller (URL search params, wired
 * in Task 8.2) — this hook is a pure "fetch given these params" layer, not
 * a state owner, matching design.md's data-flow description.
 */

import type { ApiError } from "@/lib/api/client";
import { api } from "@/lib/api/client";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { ATM_PORTAL_QUERY_KEY, ATM_PORTAL_STALE_TIME } from "./constants";
import type { AtmPortalParams, AtmPortalResponse } from "./types";

function buildQueryString(params: AtmPortalParams): string {
  return new URLSearchParams({
    page: String(params.page),
    page_size: String(params.page_size),
    search: params.search,
    status: params.status,
    machine_type: params.machine_type,
    brand: params.brand,
    deployment_type: params.deployment_type,
    region: params.region,
    sort_by: params.sort_by,
    sort_order: params.sort_order,
  }).toString();
}

async function fetchAtmPortalData(params: AtmPortalParams): Promise<AtmPortalResponse> {
  const { data } = await api.get<AtmPortalResponse>(`/atm-portal/atms?${buildQueryString(params)}`);
  return data;
}

export function useAtmPortalData(params: AtmPortalParams) {
  return useQuery<AtmPortalResponse, ApiError>({
    queryKey: [...ATM_PORTAL_QUERY_KEY, params],
    queryFn: () => fetchAtmPortalData(params),
    staleTime: ATM_PORTAL_STALE_TIME,
    placeholderData: keepPreviousData,
    refetchOnWindowFocus: true,
  });
}
