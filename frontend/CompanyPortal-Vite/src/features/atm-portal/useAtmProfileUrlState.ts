/**
 * URL search-param sync for the ATM Profile page: active tab (?tab=) plus
 * per-history pagination/date-range state. Follows useAtmPortalUrlState.ts's
 * pattern (defaults omitted from the URL for clean, shareable links).
 *
 * Date range defaults to the last 30 days (design.md) when absent from the
 * URL — computed once per mount so "last 30 days" doesn't silently drift
 * mid-session as the user pages through results.
 */

import { useNavigate, useSearch } from "@tanstack/react-router";
import { useState } from "react";
import { z } from "zod";
import type { AtmProfileHistoryParams, AtmProfileTab } from "./types";

export const ATM_PROFILE_SEARCH_SCHEMA = z.object({
  tab: z.enum(["replenish", "cashpos"]).optional(),
  page: z.number().int().min(1).optional(),
  page_size: z.number().int().min(1).max(100).optional(),
  date_from: z.string().optional(),
  date_to: z.string().optional(),
});

export type AtmProfileSearchParams = z.infer<typeof ATM_PROFILE_SEARCH_SCHEMA>;

const DAY_MS = 24 * 60 * 60 * 1000;
const DEFAULT_RANGE_DAYS = 30;

function toIsoDate(date: Date): string {
  return date.toISOString().slice(0, 10);
}

function defaultDateRange(): { date_from: string; date_to: string } {
  const today = new Date();
  const from = new Date(today.getTime() - DEFAULT_RANGE_DAYS * DAY_MS);
  return { date_from: toIsoDate(from), date_to: toIsoDate(today) };
}

export interface UseAtmProfileUrlStateReturn {
  readonly tab: AtmProfileTab;
  readonly historyParams: AtmProfileHistoryParams;
  readonly setTab: (tab: AtmProfileTab) => void;
  readonly setHistoryParams: (partial: Partial<AtmProfileHistoryParams>) => void;
}

export function useAtmProfileUrlState(): UseAtmProfileUrlStateReturn {
  const raw = useSearch({ strict: false }) as AtmProfileSearchParams;
  const navigate = useNavigate();
  // Computed once per mount (not on every raw.page/tab read) so the
  // "last 30 days" window is stable while the user interacts with the page.
  const [defaults] = useState(defaultDateRange);

  const tab: AtmProfileTab = raw.tab === "cashpos" ? "cashpos" : "replenish";
  const historyParams: AtmProfileHistoryParams = {
    page: raw.page ?? 1,
    page_size: raw.page_size ?? 25,
    date_from: raw.date_from ?? defaults.date_from,
    date_to: raw.date_to ?? defaults.date_to,
  };

  function setTab(nextTab: AtmProfileTab): void {
    navigate({ to: ".", search: (prev: AtmProfileSearchParams) => ({ ...prev, tab: nextTab }) });
  }

  function setHistoryParams(partial: Partial<AtmProfileHistoryParams>): void {
    navigate({
      to: ".",
      search: (prev: AtmProfileSearchParams) => ({ ...prev, ...partial }),
    });
  }

  return { tab, historyParams, setTab, setHistoryParams };
}
