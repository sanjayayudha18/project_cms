/**
 * URL search-param sync for ATM Portal filters/sort/page/mode.
 * - Filter/sort/page/mode state lives in URL search params
 * - Date filters default to TODAY (computed per call, local timezone);
 *   default values are omitted from the URL for clean, shareable links
 * - An explicit empty string ("?date_from=") means the user cleared the
 *   filter: no date bound sent to the API (backend "" = no filter)
 * - Mode default is replenish; switching mode resets page to 1 and mode-specific sort
 */

import { useNavigate, useSearch } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { z } from "zod";
import {
  ATM_PORTAL_MODE_CASHPOS,
  ATM_PORTAL_MODE_REPLENISH,
  CASHPOS_DEFAULT_SORT_BY,
  CASHPOS_DEFAULT_SORT_ORDER,
  REPLENISH_DEFAULT_SORT_BY,
  REPLENISH_DEFAULT_SORT_ORDER,
} from "./constants";
import { todayISO } from "./lib/formatters";
import type { AtmPortalMode, AtmPortalParams } from "./types";

export const ATM_PORTAL_SEARCH_SCHEMA = z.object({
  mode: z.enum(["replenish", "cashpos"]).optional(),
  page: z.number().int().min(1).optional(),
  page_size: z.number().int().min(1).max(100).optional(),
  search: z.string().max(100).optional(),
  status: z.string().optional(),
  machine_type: z.string().optional(),
  brand: z.string().optional(),
  deployment_type: z.string().optional(),
  region: z.string().optional(),
  date_from: z.string().optional(),
  date_to: z.string().optional(),
  sort_by: z.string().optional(),
  sort_order: z.enum(["asc", "desc"]).optional(),
});

export type AtmPortalSearchParams = z.infer<typeof ATM_PORTAL_SEARCH_SCHEMA>;

const ATM_PORTAL_SEARCH_DEFAULTS: AtmPortalParams = {
  mode: ATM_PORTAL_MODE_REPLENISH,
  page: 1,
  page_size: 25,
  search: "",
  status: "all",
  machine_type: "",
  brand: "",
  deployment_type: "",
  region: "",
  // Date defaults are dynamic (today) — applied in parseSearchParams and
  // defaultForKey, not here. The "" placeholders below are never read.
  date_from: "",
  date_to: "",
  sort_by: REPLENISH_DEFAULT_SORT_BY,
  sort_order: REPLENISH_DEFAULT_SORT_ORDER,
};

const SEARCH_DEBOUNCE_MS = 300;

function toNumber(value: unknown, fallback: number): number {
  const n = typeof value === "number" ? value : Number(value);
  return Number.isFinite(n) ? n : fallback;
}

function toStringOrDefault(value: unknown, fallback: string): string {
  return typeof value === "string" && value !== "" ? value : fallback;
}

function parseMode(value: unknown): AtmPortalMode {
  return value === ATM_PORTAL_MODE_CASHPOS ? ATM_PORTAL_MODE_CASHPOS : ATM_PORTAL_MODE_REPLENISH;
}

/**
 * Merges raw URL search values with defaults. Mode-aware sort defaults:
 * cashpos defaults to cashpos_date/desc when sort is absent. Dates: absent
 * → today (default), "" → explicitly cleared (no filter), any string →
 * itself.
 */
export function parseSearchParams(raw: Record<string, unknown>): AtmPortalParams {
  const mode = parseMode(raw.mode);
  const defaultSortBy =
    mode === ATM_PORTAL_MODE_CASHPOS ? CASHPOS_DEFAULT_SORT_BY : REPLENISH_DEFAULT_SORT_BY;
  const defaultSortOrder =
    mode === ATM_PORTAL_MODE_CASHPOS ? CASHPOS_DEFAULT_SORT_ORDER : REPLENISH_DEFAULT_SORT_ORDER;

  return {
    mode,
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
    date_from: typeof raw.date_from === "string" ? raw.date_from : todayISO(),
    date_to: typeof raw.date_to === "string" ? raw.date_to : todayISO(),
    sort_by: toStringOrDefault(raw.sort_by, defaultSortBy),
    sort_order:
      raw.sort_order === "asc" || raw.sort_order === "desc" ? raw.sort_order : defaultSortOrder,
  };
}

/** Mode-aware default comparison for omitDefaults. */
function defaultForKey(
  key: keyof AtmPortalParams,
  mode: AtmPortalMode,
): AtmPortalParams[keyof AtmPortalParams] {
  if (key === "sort_by") {
    return mode === ATM_PORTAL_MODE_CASHPOS ? CASHPOS_DEFAULT_SORT_BY : REPLENISH_DEFAULT_SORT_BY;
  }
  if (key === "sort_order") {
    return mode === ATM_PORTAL_MODE_CASHPOS
      ? CASHPOS_DEFAULT_SORT_ORDER
      : REPLENISH_DEFAULT_SORT_ORDER;
  }
  if (key === "date_from" || key === "date_to") {
    return todayISO();
  }
  return ATM_PORTAL_SEARCH_DEFAULTS[key];
}

/** Drops keys whose value equals the mode-aware default. */
export function omitDefaults(params: AtmPortalParams): AtmPortalSearchParams {
  const out: AtmPortalSearchParams = {};
  for (const key of Object.keys(ATM_PORTAL_SEARCH_DEFAULTS) as (keyof AtmPortalParams)[]) {
    if (params[key] !== defaultForKey(key, params.mode)) {
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
  readonly params: AtmPortalParams;
  readonly searchInput: string;
  readonly setSearchInput: (value: string) => void;
  readonly setParams: (partial: Partial<Omit<AtmPortalParams, "search">>) => void;
  readonly setMode: (mode: AtmPortalMode) => void;
}

export function useAtmPortalUrlState(): UseAtmPortalUrlStateReturn {
  const rawSearch = useSearch({ strict: false }) as Record<string, unknown>;
  const navigate = useNavigate();
  const params = parseSearchParams(rawSearch);

  const [searchInput, setSearchInput] = useState(params.search);
  const debouncedSearch = useDebouncedValue(searchInput, SEARCH_DEBOUNCE_MS);

  useEffect(() => {
    setSearchInput(params.search);
  }, [params.search]);

  // biome-ignore lint/correctness/useExhaustiveDependencies: params is re-derived every render; including it would re-fire this effect every render
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

  function setMode(mode: AtmPortalMode): void {
    if (mode === params.mode) {
      return;
    }
    const sortBy =
      mode === ATM_PORTAL_MODE_CASHPOS ? CASHPOS_DEFAULT_SORT_BY : REPLENISH_DEFAULT_SORT_BY;
    const sortOrder =
      mode === ATM_PORTAL_MODE_CASHPOS ? CASHPOS_DEFAULT_SORT_ORDER : REPLENISH_DEFAULT_SORT_ORDER;
    setParams({
      mode,
      page: 1,
      sort_by: sortBy,
      sort_order: sortOrder,
    });
  }

  return { params, searchInput, setSearchInput, setParams, setMode };
}
