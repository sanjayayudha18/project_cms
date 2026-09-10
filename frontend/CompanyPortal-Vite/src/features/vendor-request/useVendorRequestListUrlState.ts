/**
 * URL search-param sync for the Vendor Request list page (Req 13.3-13.4).
 * Same shape as dmaa-forecast's useDmaaForecastUrlState.ts: state lives in
 * URL search params, filter changes reset page to 1, defaults are omitted
 * from the URL for clean/shareable links.
 */

import { useNavigate, useSearch } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { z } from "zod";
import { VENDOR_REQUEST_STATUSES, type VendorRequestStatus } from "./types";

export const VENDOR_REQUEST_LIST_SEARCH_SCHEMA = z.object({
  status: z
    .array(z.enum(VENDOR_REQUEST_STATUSES as [VendorRequestStatus, ...VendorRequestStatus[]]))
    .optional(),
  forecastDate: z.string().optional(),
  search: z.string().max(50).optional(),
  page: z.number().int().min(1).optional(),
  pageSize: z.number().int().min(1).max(100).optional(),
});

export type VendorRequestListSearchParams = z.infer<typeof VENDOR_REQUEST_LIST_SEARCH_SCHEMA>;

export interface ListParams {
  status: VendorRequestStatus[];
  forecastDate: string;
  search: string;
  page: number;
  pageSize: number;
}

const DEFAULTS: ListParams = {
  status: [],
  forecastDate: "",
  search: "",
  page: 1,
  pageSize: 10,
};

const SEARCH_DEBOUNCE_MS = 300;

export function parseParams(raw: Record<string, unknown>): ListParams {
  return {
    status: Array.isArray(raw.status) ? (raw.status as VendorRequestStatus[]) : DEFAULTS.status,
    forecastDate: typeof raw.forecastDate === "string" ? raw.forecastDate : DEFAULTS.forecastDate,
    search: typeof raw.search === "string" ? raw.search : DEFAULTS.search,
    page: typeof raw.page === "number" ? raw.page : DEFAULTS.page,
    pageSize: typeof raw.pageSize === "number" ? raw.pageSize : DEFAULTS.pageSize,
  };
}

export function omitDefaults(params: ListParams): VendorRequestListSearchParams {
  const out: VendorRequestListSearchParams = {};
  if (params.status.length > 0) out.status = params.status;
  if (params.forecastDate !== DEFAULTS.forecastDate) out.forecastDate = params.forecastDate;
  if (params.search !== DEFAULTS.search) out.search = params.search;
  if (params.page !== DEFAULTS.page) out.page = params.page;
  if (params.pageSize !== DEFAULTS.pageSize) out.pageSize = params.pageSize;
  return out;
}

function useDebouncedValue<T>(value: T, delayMs: number): T {
  const [debounced, setDebounced] = useState(value);
  useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), delayMs);
    return () => clearTimeout(timer);
  }, [value, delayMs]);
  return debounced;
}

export function useVendorRequestListUrlState() {
  const rawSearch = useSearch({ strict: false }) as Record<string, unknown>;
  const navigate = useNavigate();
  const params = parseParams(rawSearch);

  const [searchInput, setSearchInput] = useState(params.search);
  const debouncedSearch = useDebouncedValue(searchInput, SEARCH_DEBOUNCE_MS);

  useEffect(() => {
    setSearchInput(params.search);
  }, [params.search]);

  // biome-ignore lint/correctness/useExhaustiveDependencies: params is re-derived every render; including it would re-fire this effect every render
  useEffect(() => {
    if (debouncedSearch === params.search) return;
    const next = omitDefaults({ ...params, search: debouncedSearch, page: 1 });
    navigate({ to: ".", search: next });
  }, [debouncedSearch]);

  function setParams(partial: Partial<ListParams>): void {
    const next = omitDefaults({ ...params, ...partial });
    navigate({ to: ".", search: next });
  }

  return { params, searchInput, setSearchInput, setParams };
}
