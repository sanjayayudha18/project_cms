/**
 * TanStack Query hooks for ATM Portal replenish (/atms) and cashpos (/cashpos).
 */

import type { ApiError } from "@/lib/api/client";
import { api } from "@/lib/api/client";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import {
  ATM_CASHPOS_QUERY_KEY,
  ATM_PORTAL_MODE_CASHPOS,
  ATM_PORTAL_QUERY_KEY,
  ATM_PORTAL_STALE_TIME,
} from "./constants";
import type {
  AtmCashposParams,
  AtmCashposResponse,
  AtmPortalParams,
  AtmPortalResponse,
} from "./types";

function buildReplenishQueryString(params: AtmPortalParams): string {
  return new URLSearchParams({
    page: String(params.page),
    page_size: String(params.page_size),
    search: params.search,
    status: params.status,
    machine_type: params.machine_type,
    brand: params.brand,
    deployment_type: params.deployment_type,
    region: params.region,
    date_from: params.date_from,
    date_to: params.date_to,
    sort_by: params.sort_by,
    sort_order: params.sort_order,
  }).toString();
}

function toCashposParams(params: AtmPortalParams): AtmCashposParams {
  return {
    page: params.page,
    page_size: params.page_size,
    search: params.search,
    date_from: params.date_from,
    date_to: params.date_to,
    sort_by: params.sort_by,
    sort_order: params.sort_order,
  };
}

function buildCashposQueryString(params: AtmCashposParams): string {
  return new URLSearchParams({
    page: String(params.page),
    page_size: String(params.page_size),
    search: params.search,
    date_from: params.date_from,
    date_to: params.date_to,
    sort_by: params.sort_by,
    sort_order: params.sort_order,
  }).toString();
}

async function fetchAtmPortalData(params: AtmPortalParams): Promise<AtmPortalResponse> {
  const { data } = await api.get<AtmPortalResponse>(
    `/atm-portal/atms?${buildReplenishQueryString(params)}`,
  );
  return data;
}

async function fetchAtmCashposData(params: AtmCashposParams): Promise<AtmCashposResponse> {
  const { data } = await api.get<AtmCashposResponse>(
    `/atm-portal/cashpos?${buildCashposQueryString(params)}`,
  );
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

export function useAtmCashposData(params: AtmPortalParams) {
  const cashposParams = toCashposParams(params);
  const enabled = params.mode === ATM_PORTAL_MODE_CASHPOS;
  return useQuery<AtmCashposResponse, ApiError>({
    queryKey: [...ATM_CASHPOS_QUERY_KEY, cashposParams],
    queryFn: () => fetchAtmCashposData(cashposParams),
    staleTime: ATM_PORTAL_STALE_TIME,
    placeholderData: keepPreviousData,
    refetchOnWindowFocus: true,
    enabled,
  });
}
