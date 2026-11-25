/**
 * URL state for Manajemen Peran (Req 5.1, 6.1): ?portal=main|vendor&role=<id>&q=<cari>.
 * Invalid values fall back (portal → main, role → none); defaults are omitted from the
 * URL. Same pattern as admin-regions/useAdminRegionsUrlState.ts.
 */

import { useNavigate, useSearch } from "@tanstack/react-router";
import { z } from "zod";
import type { Portal } from "./types";

export const ROLE_MGMT_SEARCH_SCHEMA = z.object({
  portal: z.enum(["main", "vendor"]).optional(),
  role: z.number().int().positive().optional(),
  q: z.string().max(100).optional(),
});

export interface RoleMgmtUrlParams {
  portal: Portal;
  role: number | null;
  q: string;
}

export function parseParams(raw: Record<string, unknown>): RoleMgmtUrlParams {
  const roleNum = typeof raw.role === "string" ? Number(raw.role) : raw.role;
  return {
    portal: raw.portal === "vendor" ? "vendor" : "main",
    role: typeof roleNum === "number" && Number.isInteger(roleNum) && roleNum > 0 ? roleNum : null,
    q: typeof raw.q === "string" ? raw.q.slice(0, 100) : "",
  };
}

export function toSearch(params: RoleMgmtUrlParams): z.infer<typeof ROLE_MGMT_SEARCH_SCHEMA> {
  const out: z.infer<typeof ROLE_MGMT_SEARCH_SCHEMA> = {};
  if (params.portal !== "main") out.portal = params.portal;
  if (params.role !== null) out.role = params.role;
  if (params.q !== "") out.q = params.q;
  return out;
}

export function useRoleManagementUrlState() {
  const raw = useSearch({ strict: false }) as Record<string, unknown>;
  const navigate = useNavigate();
  const params = parseParams(raw);

  function setParams(partial: Partial<RoleMgmtUrlParams>): void {
    navigate({ to: ".", search: toSearch({ ...params, ...partial }), replace: true });
  }

  return { params, setParams };
}
