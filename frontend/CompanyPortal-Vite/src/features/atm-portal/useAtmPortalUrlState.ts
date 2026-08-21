/**
 * URL search-param sync for ATM Portal filters/sort/page (design.md's
 * "URL State Sync" section):
 * - Filter/sort/page state lives in URL search params, via useSearch()
 * - Default values are omitted from the URL for clean, shareable links
 * - Browser back/forward navigates filter history — so param changes push a
 *   new history entry, not replace (the one exception: `search` writes are
 *   debounced 300ms, so a burst of keystrokes collapses into one entry)
 *
 * Written to be self-sufficient ahead of Task 10.3's route registration:
 * reads/parses search params defensively with useSearch({ strict: false })
 * rather than depending on a route's `validateSearch` already existing.
 * ATM_PORTAL_SEARCH_SCHEMA is exported so Task 10.3 can wire it into the
 * route's `validateSearch` for compile-time-typed search params too.
 */

import { useNavigate, useSearch } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { z } from "zod";
import type { AtmPortalParams } from "./types";

export const ATM_PORTAL_SEARCH_SCHEMA = z.object({
  page: z.number().int().min(1).optional(),
  page_size: z.number().int().min(1).max(100).optional(),
  search: z.string().max(100).optional(),
  status: z.string().optional(),
  machine_type: z.string().optional(),
  brand: z.string().optional(),
  deployment_type: z.string().optional(),
  region: z.string().optional(),
  sort_by: z.string().optional(),
  sort_order: z.enum(["asc", "desc"]).optional(),
});

export type AtmPortalSearchParams = z.infer<typeof ATM_PORTAL_SEARCH_SCHEMA>;

const ATM_PORTAL_SEARCH_DEFAULTS: AtmPortalParams = {
  page: 1,
  page_size: 25,
  search: "",
  status: "all",
  machine_type: "",
  brand: "",
  deployment_type: "",
  region: "",
  sort_by: "terminal_id",
  sort_order: "asc",
};

const SEARCH_DEBOUNCE_MS = 300;

function toNumber(value: unknown, fallback: number): number {
  const n = typeof value === "number" ? value : Number(value);
  return Number.isFinite(n) ? n : fallback;
}

function toStringOrDefault(value: unknown, fallback: string): string {
  return typeof value === "string" && value !== "" ? value : fallback;
}

/**
 * Merges raw (possibly-string, possibly-absent) URL search values with
 * defaults. Exported (paired with omitDefaults) for Property 11's
 * round-trip test — serializing a params object with omitDefaults and then
 * deserializing it with this function must reconstruct the original.
 */
export function parseSearchParams(raw: Record<string, unknown>): AtmPortalParams {
  return {
    page: toNumber(raw.page, ATM_PORTAL_SEARCH_DEFAULTS.page),
    page_size: toNumber(raw.page_size, ATM_PORTAL_SEARCH_DEFAULTS.page_size),
    search: toStringOrDefault(raw.search, ATM_PORTAL_SEARCH_DEFAULTS.search),
    status: toStringOrDefault(raw.status, ATM_PORTAL_SEARCH_DEFAULTS.status),
    machine_type: toStringOrDefault(raw.machine_type, ATM_PORTAL_SEARCH_DEFAULTS.machine_type),
    brand: toStringOrDefault(raw.brand, ATM_PORTAL_SEARCH_DEFAULTS.brand),
    deployment_type: toStringOrDefault(
      raw.deployment_type,
      ATM_PORTAL_SEARCH_DEFAULTS.deployment_type,
    ),
    region: toStringOrDefault(raw.region, ATM_PORTAL_SEARCH_DEFAULTS.region),
    sort_by: toStringOrDefault(raw.sort_by, ATM_PORTAL_SEARCH_DEFAULTS.sort_by),
    sort_order: raw.sort_order === "desc" ? "desc" : ATM_PORTAL_SEARCH_DEFAULTS.sort_order,
  };
}

/** Drops keys whose value equals the default, so the URL stays clean. */
export function omitDefaults(params: AtmPortalParams): AtmPortalSearchParams {
  const out: AtmPortalSearchParams = {};
  for (const key of Object.keys(ATM_PORTAL_SEARCH_DEFAULTS) as (keyof AtmPortalParams)[]) {
    if (params[key] !== ATM_PORTAL_SEARCH_DEFAULTS[key]) {
      (out as Record<string, unknown>)[key] = params[key];
    }
  }
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

export interface UseAtmPortalUrlStateReturn {
  /** Current filter/sort/page state, defaults already applied. */
  readonly params: AtmPortalParams;
  /** The search input's live (non-debounced) value, for the controlled input. */
  readonly searchInput: string;
  /** Updates the search input immediately; the URL write is debounced 300ms. */
  readonly setSearchInput: (value: string) => void;
  /** Updates any non-search param(s) immediately (page, sort, filters, ...). */
  readonly setParams: (partial: Partial<Omit<AtmPortalParams, "search">>) => void;
}

export function useAtmPortalUrlState(): UseAtmPortalUrlStateReturn {
  const rawSearch = useSearch({ strict: false }) as Record<string, unknown>;
  const navigate = useNavigate();
  const params = parseSearchParams(rawSearch);

  const [searchInput, setSearchInput] = useState(params.search);
  const debouncedSearch = useDebouncedValue(searchInput, SEARCH_DEBOUNCE_MS);

  // Reflect external URL changes (back/forward, shared link) into the input.
  useEffect(() => {
    setSearchInput(params.search);
  }, [params.search]);

  // biome-ignore lint/correctness/useExhaustiveDependencies: params is re-derived every render; including it (or navigate/omitDefaults) here would re-fire this effect every render and fight the debounce
  useEffect(() => {
    if (debouncedSearch === params.search) {
      return;
    }
    const next = omitDefaults({ ...params, search: debouncedSearch });
    navigate({ to: ".", search: next });
  }, [debouncedSearch]);

  function setParams(partial: Partial<Omit<AtmPortalParams, "search">>): void {
    const next = omitDefaults({ ...params, ...partial });
    navigate({ to: ".", search: next });
  }

  return { params, searchInput, setSearchInput, setParams };
}
