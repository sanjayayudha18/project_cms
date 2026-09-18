/**
 * URL search-param sync for the admin ATMs list (Req 8.3-8.4, 10.x), same
 * pattern as admin-vendors/useAdminVendorsUrlState.ts: state lives in URL
 * search params, filter changes reset page to 1, defaults are omitted from
 * the URL for clean/shareable links.
 */

import { useNavigate, useSearch } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { z } from "zod";
import type { ATMStatus, AdminATMsListParams, PriorityClass } from "./types";

export const ADMIN_ATMS_SEARCH_SCHEMA = z.object({
  page: z.number().int().min(1).optional(),
  page_size: z.number().int().min(1).max(100).optional(),
  status: z.enum(["active", "disabled", "all"]).optional(),
  q: z.string().max(100).optional(),
  brand: z.string().max(100).optional(),
  machine_type: z.string().max(100).optional(),
  deployment_type: z.string().max(100).optional(),
  priority_class: z.enum(["VIP", "Non VIP", "Industri"]).optional(),
});

export type AdminATMsSearchParams = z.infer<typeof ADMIN_ATMS_SEARCH_SCHEMA>;

export interface AdminATMsUrlParams {
  page: number;
  page_size: number;
  status: ATMStatus;
  q: string;
  brand: string;
  machine_type: string;
  deployment_type: string;
  priority_class: PriorityClass | "";
}

const DEFAULTS: AdminATMsUrlParams = {
  page: 1,
  page_size: 25,
  status: "active",
  q: "",
  brand: "",
  machine_type: "",
  deployment_type: "",
  priority_class: "",
};

const SEARCH_DEBOUNCE_MS = 300;

export function parseParams(raw: Record<string, unknown>): AdminATMsUrlParams {
  return {
    page: typeof raw.page === "number" ? raw.page : DEFAULTS.page,
    page_size: typeof raw.page_size === "number" ? raw.page_size : DEFAULTS.page_size,
    status: typeof raw.status === "string" ? (raw.status as ATMStatus) : DEFAULTS.status,
    q: typeof raw.q === "string" ? raw.q : DEFAULTS.q,
    brand: typeof raw.brand === "string" ? raw.brand : DEFAULTS.brand,
    machine_type: typeof raw.machine_type === "string" ? raw.machine_type : DEFAULTS.machine_type,
    deployment_type:
      typeof raw.deployment_type === "string" ? raw.deployment_type : DEFAULTS.deployment_type,
    priority_class:
      typeof raw.priority_class === "string"
        ? (raw.priority_class as PriorityClass)
        : DEFAULTS.priority_class,
  };
}

export function omitDefaults(params: AdminATMsUrlParams): AdminATMsSearchParams {
  const out: AdminATMsSearchParams = {};
  if (params.page !== DEFAULTS.page) out.page = params.page;
  if (params.page_size !== DEFAULTS.page_size) out.page_size = params.page_size;
  if (params.status !== DEFAULTS.status) out.status = params.status;
  if (params.q !== DEFAULTS.q) out.q = params.q;
  if (params.brand !== DEFAULTS.brand) out.brand = params.brand;
  if (params.machine_type !== DEFAULTS.machine_type) out.machine_type = params.machine_type;
  if (params.deployment_type !== DEFAULTS.deployment_type)
    out.deployment_type = params.deployment_type;
  if (params.priority_class !== DEFAULTS.priority_class)
    out.priority_class = params.priority_class as PriorityClass;
  return out;
}

/** Maps the URL-state shape (empty string = "no filter") to the list-query params shape (undefined = "no filter"). */
export function toListParams(params: AdminATMsUrlParams): AdminATMsListParams {
  return {
    page: params.page,
    page_size: params.page_size,
    status: params.status,
    q: params.q || undefined,
    brand: params.brand || undefined,
    machine_type: params.machine_type || undefined,
    deployment_type: params.deployment_type || undefined,
    priority_class: params.priority_class || undefined,
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

export function useAdminATMsUrlState() {
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

  function setParams(partial: Partial<AdminATMsUrlParams>): void {
    const next = omitDefaults({ ...params, ...partial });
    navigate({ to: ".", search: next });
  }

  return { params, searchInput, setSearchInput, setParams };
}
