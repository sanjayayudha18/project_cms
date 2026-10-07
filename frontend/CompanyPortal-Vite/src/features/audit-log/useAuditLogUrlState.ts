/**
 * URL search-param sync for the Audit Log list (Req 6.2, 6.4). Same pattern
 * as admin-users/useAdminUsersUrlState.ts: state lives in the URL, any filter
 * change resets page to 1, defaults are omitted for clean shareable links.
 * TanStack Router JSON-parses search values (`?actor_id=3` arrives as the
 * number 3), so ids are accepted as numbers and stringified into
 * AuditLogFilters, which the API query builder expects.
 */

import { useNavigate, useSearch } from "@tanstack/react-router";
import { z } from "zod";
import { type AuditLogFilters, EMPTY_AUDIT_LOG_FILTERS } from "./types";

export const AUDIT_LOG_SEARCH_SCHEMA = z.object({
  page: z.number().int().min(1).optional(),
  page_size: z.number().int().min(1).max(100).optional(),
  actor_id: z.number().int().optional(),
  action: z.string().optional(),
  entity_type: z.string().optional(),
  entity_id: z.number().int().optional(),
  date_from: z.string().optional(),
  date_to: z.string().optional(),
});

export type AuditLogSearchParams = z.infer<typeof AUDIT_LOG_SEARCH_SCHEMA>;

export interface AuditLogUrlParams {
  page: number;
  page_size: number;
  filters: AuditLogFilters;
}

const DEFAULT_PAGE = 1;
const DEFAULT_PAGE_SIZE = 25;

function asString(v: unknown): string | null {
  if (typeof v === "string" && v !== "") return v;
  if (typeof v === "number") return String(v);
  return null;
}

export function parseParams(raw: Record<string, unknown>): AuditLogUrlParams {
  return {
    page: typeof raw.page === "number" ? raw.page : DEFAULT_PAGE,
    page_size: typeof raw.page_size === "number" ? raw.page_size : DEFAULT_PAGE_SIZE,
    filters: {
      actor_id: asString(raw.actor_id),
      action: asString(raw.action),
      entity_type: asString(raw.entity_type),
      entity_id: asString(raw.entity_id),
      date_from: asString(raw.date_from),
      date_to: asString(raw.date_to),
    },
  };
}

export function omitDefaults(params: AuditLogUrlParams): AuditLogSearchParams {
  const { page, page_size, filters } = params;
  const out: AuditLogSearchParams = {};
  if (page !== DEFAULT_PAGE) out.page = page;
  if (page_size !== DEFAULT_PAGE_SIZE) out.page_size = page_size;
  if (filters.actor_id) out.actor_id = Number(filters.actor_id);
  if (filters.action) out.action = filters.action;
  if (filters.entity_type) out.entity_type = filters.entity_type;
  if (filters.entity_id) out.entity_id = Number(filters.entity_id);
  if (filters.date_from) out.date_from = filters.date_from;
  if (filters.date_to) out.date_to = filters.date_to;
  return out;
}

export function useAuditLogUrlState() {
  const rawSearch = useSearch({ strict: false }) as Record<string, unknown>;
  const navigate = useNavigate();
  const params = parseParams(rawSearch);

  /** Any filter change resets page to 1 (Req 6.2). */
  function setFilters(partial: Partial<AuditLogFilters>): void {
    const next = omitDefaults({
      ...params,
      page: DEFAULT_PAGE,
      filters: { ...params.filters, ...partial },
    });
    navigate({ to: ".", search: next });
  }

  function setPage(page: number): void {
    navigate({ to: ".", search: omitDefaults({ ...params, page }) });
  }

  function resetFilters(): void {
    navigate({
      to: ".",
      search: omitDefaults({ ...params, page: DEFAULT_PAGE, filters: EMPTY_AUDIT_LOG_FILTERS }),
    });
  }

  return { params, setFilters, setPage, resetFilters };
}
