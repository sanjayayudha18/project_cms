/**
 * URL search-param sync for Daftar PKS (Req 5.3): filters live in the URL, a filter change
 * resets page to 1, defaults (status terkini, page 1) are omitted for clean shareable links.
 * `?vendor=<kode>` is applied on load (links from the vendor list).
 */

import { useNavigate, useSearch } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { z } from "zod";
import { useDebouncedValue } from "../admin-vendors/useAdminVendorsUrlState";

export const PKS_LIST_STATUSES = [
  "terkini",
  "semua",
  "aktif",
  "need_renewal",
  "belum_berlaku",
  "expired",
  "terminated",
  "diganti",
] as const;

export type PksListStatusValue = (typeof PKS_LIST_STATUSES)[number];

export const ADMIN_PKS_SEARCH_SCHEMA = z.object({
  page: z.number().int().min(1).optional(),
  page_size: z.number().int().min(1).max(100).optional(),
  status: z.enum(PKS_LIST_STATUSES).optional(),
  pks_type: z.string().max(50).optional(),
  vendor: z.string().max(50).optional(),
  q: z.string().max(100).optional(),
});

export type AdminPksSearchParams = z.infer<typeof ADMIN_PKS_SEARCH_SCHEMA>;

export interface AdminPksUrlParams {
  page: number;
  page_size: number;
  status: PksListStatusValue;
  pks_type?: string;
  vendor?: string;
  q: string;
}

const DEFAULTS = { page: 1, page_size: 25, status: "terkini", q: "" } as const;
const SEARCH_DEBOUNCE_MS = 300;

export function parseParams(raw: Record<string, unknown>): AdminPksUrlParams {
  const str = (v: unknown) => (typeof v === "string" && v.trim() !== "" ? v.trim() : undefined);
  return {
    page: typeof raw.page === "number" ? raw.page : DEFAULTS.page,
    page_size: typeof raw.page_size === "number" ? raw.page_size : DEFAULTS.page_size,
    status: PKS_LIST_STATUSES.find((s) => s === raw.status) ?? DEFAULTS.status,
    pks_type: str(raw.pks_type),
    vendor: str(raw.vendor),
    q: typeof raw.q === "string" ? raw.q : DEFAULTS.q,
  };
}

export function omitDefaults(p: AdminPksUrlParams): AdminPksSearchParams {
  const out: AdminPksSearchParams = {};
  if (p.page !== DEFAULTS.page) out.page = p.page;
  if (p.page_size !== DEFAULTS.page_size) out.page_size = p.page_size;
  if (p.status !== DEFAULTS.status) out.status = p.status;
  if (p.pks_type) out.pks_type = p.pks_type;
  if (p.vendor) out.vendor = p.vendor;
  if (p.q !== DEFAULTS.q) out.q = p.q;
  return out;
}

export function useAdminPksUrlState() {
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
    navigate({ to: ".", search: omitDefaults({ ...params, q: debouncedSearch, page: 1 }) });
  }, [debouncedSearch]);

  function setParams(partial: Partial<AdminPksUrlParams>): void {
    const resetPage = partial.page === undefined;
    navigate({
      to: ".",
      search: omitDefaults({ ...params, ...partial, ...(resetPage ? { page: 1 } : {}) }),
    });
  }

  return { params, searchInput, setSearchInput, setParams };
}
