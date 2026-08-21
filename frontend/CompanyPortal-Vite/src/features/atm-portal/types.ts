/**
 * ATM Portal feature local type definitions.
 * Mirrors the GET /api/v1/atm-portal/atms response contract (design.md).
 *
 * Deviates from design.md in three fields — verified against the live
 * backend response, not just the spec: `address`, `brand`, and
 * `deployment_type` are typed as plain `string`, not `string | null`.
 * The underlying DB columns (atms.brand, atms.deployment_type,
 * locations.address_line1) are all NOT NULL, and the actual JSON handler
 * (backend/internal/handler/atm_portal_handler.go) never emits null for
 * them. `machine_type` is `string`, not a `"ATM" | "CRM"` union — the real
 * data also has `CDM`.
 */

export type ReplenishmentStatus = "critical" | "low" | "normal" | "unconfigured" | "no_data";

export interface AtmRecord {
  readonly terminal_id: string;
  readonly location_name: string;
  readonly address: string;
  readonly machine_type: string;
  readonly brand: string;
  readonly deployment_type: string;
  readonly low_threshold: number | null;
  readonly critical_threshold: number | null;
  readonly last_replenish_date: string | null;
  readonly last_replenish_time: string | null;
  readonly refund_total: number | null;
  readonly replenish_total: number | null;
  readonly escrow: number | null;
  readonly status: ReplenishmentStatus;
}

export interface AtmSummary {
  readonly total: number;
  readonly critical: number;
  readonly low: number;
  readonly normal: number;
  readonly unconfigured: number;
  readonly no_data: number;
}

export interface AtmPortalResponse {
  readonly data: readonly AtmRecord[];
  readonly summary: AtmSummary;
  readonly total: number;
  readonly page: number;
  readonly page_size: number;
  readonly last_updated: string | null;
}

export interface AtmPortalParams {
  readonly page: number;
  readonly page_size: number;
  readonly search: string;
  readonly status: string;
  readonly machine_type: string;
  readonly brand: string;
  readonly deployment_type: string;
  readonly region: string;
  readonly date_from: string;
  readonly date_to: string;
  readonly sort_by: string;
  readonly sort_order: "asc" | "desc";
}
