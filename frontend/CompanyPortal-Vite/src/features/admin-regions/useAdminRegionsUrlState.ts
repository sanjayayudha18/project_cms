/**
 * URL search-param sync for the admin regions list (Req 1.2, 1.4), same
 * pattern as admin-atms/useAdminATMsUrlState.ts: state lives in URL search
 * params, filter changes reset page to 1, defaults are omitted from the URL
 * for clean/shareable links. page_size defaults to 20 (Req 1.2) -- distinct
 * from admin-atms' 25 default.
 */

import { useNavigate, useSearch } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { z } from "zod";
import type { AdminRegionsListParams, RegionStatus } from "./types";

export const ADMIN_REGIONS_SEARCH_SCHEMA = z.object({
  page: z.number().int().min(1).optional(),
  page_size: z.number().int().min(1).max(100).optional(),
  status: z.enum(["active", "inactive", "all"]).optional(),
  q: z.string().max(100).optional(),
});

export type AdminRegionsSearchParams = z.infer<typeof ADMIN_REGIONS_SEARCH_SCHEMA>;

export interface AdminRegionsUrlParams {
  page: number;
  page_size: number;
  status: RegionStatus;
  q: string;
}

const DEFAULTS: AdminRegionsUrlParams = {
  page: 1,
  page_size: 20,
  status: "active",
  q: "",
};

const SEARCH_DEBOUNCE_MS = 300;

export function parseParams(raw: Record<string, unknown>): AdminRegionsUrlParams {
  return {
    page: typeof raw.page === "number" ? raw.page : DEFAULTS.page,
    page_size: typeof raw.page_size === "number" ? raw.page_size : DEFAULTS.page_size,
    status: typeof raw.status === "string" ? (raw.status as RegionStatus) : DEFAULTS.status,
    q: typeof raw.q === "string" ? raw.q : DEFAULTS.q,
  };
}

export function omitDefaults(params: AdminRegionsUrlParams): AdminRegionsSearchParams {
  const out: AdminRegionsSearchParams = {};
  if (params.page !== DEFAULTS.page) out.page = params.page;
  if (params.page_size !== DEFAULTS.page_size) out.page_size = params.page_size;
  if (params.status !== DEFAULTS.status) out.status = params.status;
  if (params.q !== DEFAULTS.q) out.q = params.q;
  return out;
}

/** Maps the URL-state shape (empty string = "no filter") to the list-query params shape (undefined = "no filter"). */
export function toListParams(params: AdminRegionsUrlParams): AdminRegionsListParams {
  return {
    page: params.page,
    page_size: params.page_size,
    status: params.status,
    q: params.q || undefined,
  };
}

function useDebouncedValue<T>(value: T, delayMs: number): T {
  const [debounced, setDebounced] = useState(value);
  useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), delayMs);
    return () => clearTimeout(timer);
  }, [value, delayMs]);
  return debounced;
}

export function useAdminRegionsUrlState() {
  const rawSearch = useSearch({ strict: false }) as Record<string, unknown>;
  const navigate = useNavigate();
  const params = parseParams(rawSearch);

  const [searchInput, setSearchInput] = useState(params.q);
  const debouncedSearch = useDebouncedValue(searchInput, SEARCH_DEBOUNCE_MS);

  useEffect(() => {
    setSearchInput(params.q);
  }, [params.q]);

  // biome-ignore lint/correctness/useExhaustiveDependencies: params is re-derived every render; including it would re-fire this effect every render
  useEffect(() => {
    if (debouncedSearch === params.q) return;
    const next = omitDefaults({ ...params, q: debouncedSearch, page: 1 });
    navigate({ to: ".", search: next });
  }, [debouncedSearch]);

  function setParams(partial: Partial<AdminRegionsUrlParams>): void {
    const next = omitDefaults({ ...params, ...partial });
    navigate({ to: ".", search: next });
  }

  return { params, searchInput, setSearchInput, setParams };
}
