/**
 * URL search-param sync for the admin Vendors list (Req 13.3-13.4), same
 * pattern as admin-users/useAdminUsersUrlState.ts (which itself mirrors
 * vendor-request/useVendorRequestListUrlState.ts): state lives in URL
 * search params, filter changes reset page to 1, defaults are omitted from
 * the URL for clean/shareable links.
 */

import { useNavigate, useSearch } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { z } from "zod";
import type { VendorStatus } from "./types";

export const ADMIN_VENDORS_SEARCH_SCHEMA = z.object({
  page: z.number().int().min(1).optional(),
  page_size: z.number().int().min(1).max(100).optional(),
  status: z.enum(["active", "disabled", "all"]).optional(),
  q: z.string().max(100).optional(),
});

export type AdminVendorsSearchParams = z.infer<typeof ADMIN_VENDORS_SEARCH_SCHEMA>;

export interface AdminVendorsUrlParams {
  page: number;
  page_size: number;
  status: VendorStatus;
  q: string;
}

const DEFAULTS: AdminVendorsUrlParams = {
  page: 1,
  page_size: 25,
  status: "active",
  q: "",
};

const SEARCH_DEBOUNCE_MS = 300;

export function parseParams(raw: Record<string, unknown>): AdminVendorsUrlParams {
  return {
    page: typeof raw.page === "number" ? raw.page : DEFAULTS.page,
    page_size: typeof raw.page_size === "number" ? raw.page_size : DEFAULTS.page_size,
    status: typeof raw.status === "string" ? (raw.status as VendorStatus) : DEFAULTS.status,
    q: typeof raw.q === "string" ? raw.q : DEFAULTS.q,
  };
}

export function omitDefaults(params: AdminVendorsUrlParams): AdminVendorsSearchParams {
  const out: AdminVendorsSearchParams = {};
  if (params.page !== DEFAULTS.page) out.page = params.page;
  if (params.page_size !== DEFAULTS.page_size) out.page_size = params.page_size;
  if (params.status !== DEFAULTS.status) out.status = params.status;
  if (params.q !== DEFAULTS.q) out.q = params.q;
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

export function useAdminVendorsUrlState() {
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

  function setParams(partial: Partial<AdminVendorsUrlParams>): void {
    const next = omitDefaults({ ...params, ...partial });
    navigate({ to: ".", search: next });
  }

  return { params, searchInput, setSearchInput, setParams };
}
