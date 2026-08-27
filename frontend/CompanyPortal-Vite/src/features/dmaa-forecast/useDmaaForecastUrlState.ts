/**
 * URL search-param sync for DMAA Forecast filters/sort/page.
 * - State lives in URL search params (camelCase keys per Req 10.3)
 * - Date filters default to TODAY (computed per call, local timezone);
 *   default values are omitted from the URL for clean, shareable links
 * - An explicit empty string ("?dateFrom=") means the user cleared the
 *   filter: no date bound sent to the API (backend "" = no filter)
 * - Filter changes reset page to 1 (Req 4.7, 5.3)
 */

import { todayISO } from "@/features/atm-portal/lib/formatters";
import { useNavigate, useSearch } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { z } from "zod";
import {
  DMAA_FORECAST_DEFAULT_SORT_BY,
  DMAA_FORECAST_DEFAULT_SORT_ORDER,
  type DmaaForecastParams,
} from "./types";

export const DMAA_FORECAST_SEARCH_SCHEMA = z.object({
  page: z.number().int().min(1).max(10_000).optional(),
  pageSize: z.number().int().min(1).max(100).optional(),
  dateFrom: z.string().optional(),
  dateTo: z.string().optional(),
  terminalId: z.string().max(100).optional(),
  sortBy: z.string().optional(),
  sortOrder: z.enum(["asc", "desc"]).optional(),
});

export type DmaaForecastSearchParams = z.infer<typeof DMAA_FORECAST_SEARCH_SCHEMA>;

// Dates default to today, computed per call (not at module load) so the
// default tracks the calendar day. All other defaults are static.
function defaultParams(): DmaaForecastParams {
  const today = todayISO();
  return {
    page: 1,
    pageSize: 25,
    dateFrom: today,
    dateTo: today,
    terminalId: "",
    sortBy: DMAA_FORECAST_DEFAULT_SORT_BY,
    sortOrder: DMAA_FORECAST_DEFAULT_SORT_ORDER,
  };
}

const SEARCH_DEBOUNCE_MS = 300;

function toNumber(value: unknown, fallback: number): number {
  const n = typeof value === "number" ? value : Number(value);
  return Number.isFinite(n) ? n : fallback;
}

function toStringOrDefault(value: unknown, fallback: string): string {
  return typeof value === "string" && value !== "" ? value : fallback;
}

/**
 * Merges raw URL search values with defaults. Dates: absent → today
 * (default), "" → explicitly cleared (no filter), any string → itself.
 */
export function parseSearchParams(raw: Record<string, unknown>): DmaaForecastParams {
  const defaults = defaultParams();
  return {
    page: toNumber(raw.page, defaults.page),
    pageSize: toNumber(raw.pageSize, defaults.pageSize),
    dateFrom: typeof raw.dateFrom === "string" ? raw.dateFrom : defaults.dateFrom,
    dateTo: typeof raw.dateTo === "string" ? raw.dateTo : defaults.dateTo,
    terminalId: toStringOrDefault(raw.terminalId, defaults.terminalId),
    sortBy: toStringOrDefault(raw.sortBy, defaults.sortBy),
    sortOrder:
      raw.sortOrder === "asc" || raw.sortOrder === "desc" ? raw.sortOrder : defaults.sortOrder,
  };
}

/** Drops keys whose value equals the default, for clean URLs. */
export function omitDefaults(params: DmaaForecastParams): DmaaForecastSearchParams {
  const defaults = defaultParams();
  const out: DmaaForecastSearchParams = {};
  for (const key of Object.keys(defaults) as (keyof DmaaForecastParams)[]) {
    if (params[key] !== defaults[key]) {
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

export interface UseDmaaForecastUrlStateReturn {
  readonly params: DmaaForecastParams;
  readonly terminalInput: string;
  readonly setTerminalInput: (value: string) => void;
  readonly setParams: (partial: Partial<DmaaForecastParams>) => void;
}

export function useDmaaForecastUrlState(): UseDmaaForecastUrlStateReturn {
  const rawSearch = useSearch({ strict: false }) as Record<string, unknown>;
  const navigate = useNavigate();
  const params = parseSearchParams(rawSearch);

  const [terminalInput, setTerminalInput] = useState(params.terminalId);
  const debouncedTerminal = useDebouncedValue(terminalInput, SEARCH_DEBOUNCE_MS);

  useEffect(() => {
    setTerminalInput(params.terminalId);
  }, [params.terminalId]);

  // biome-ignore lint/correctness/useExhaustiveDependencies: params is re-derived every render; including it would re-fire this effect every render
  useEffect(() => {
    if (debouncedTerminal === params.terminalId) {
      return;
    }
    const next = omitDefaults({ ...params, terminalId: debouncedTerminal, page: 1 });
    navigate({ to: ".", search: next });
  }, [debouncedTerminal]);

  function setParams(partial: Partial<DmaaForecastParams>): void {
    const next = omitDefaults({ ...params, ...partial });
    navigate({ to: ".", search: next });
  }

  return { params, terminalInput, setTerminalInput, setParams };
}
