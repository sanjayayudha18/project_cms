/**
 * TanStack Query hook for the DMAA Forecast Viewer
 * (GET /api/v1/dmaa-forecast).
 */

import type { ApiError } from "@/lib/api/client";
import { api } from "@/lib/api/client";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import type { DmaaForecastParams, DmaaForecastResponse } from "./types";

const DMAA_FORECAST_QUERY_KEY = ["dmaa-forecast"] as const;
const DMAA_FORECAST_STALE_TIME = 30_000;

function buildQueryString(params: DmaaForecastParams): string {
  const search = new URLSearchParams({
    page: String(params.page),
    page_size: String(params.pageSize),
    sort_by: params.sortBy,
    sort_order: params.sortOrder,
  });
  if (params.dateFrom !== "") search.set("date_from", params.dateFrom);
  if (params.dateTo !== "") search.set("date_to", params.dateTo);
  if (params.terminalId !== "") search.set("terminal_id", params.terminalId);
  return search.toString();
}

async function fetchDmaaForecast(params: DmaaForecastParams): Promise<DmaaForecastResponse> {
  const { data } = await api.get<DmaaForecastResponse>(
    `/dmaa-forecast?${buildQueryString(params)}`,
  );
  return data;
}

export function useDmaaForecastData(params: DmaaForecastParams) {
  return useQuery<DmaaForecastResponse, ApiError>({
    queryKey: [...DMAA_FORECAST_QUERY_KEY, params],
    queryFn: () => fetchDmaaForecast(params),
    staleTime: DMAA_FORECAST_STALE_TIME,
    placeholderData: keepPreviousData,
    refetchOnWindowFocus: true,
  });
}
