/**
 * DMAA Forecast Viewer API types (design.md API contract).
 */

export interface DmaaForecastRecord {
  terminal_id: string;
  dmaa_file_id: number;
  /** ISO date "YYYY-MM-DD" */
  periode_pred: string;
  denom: number;
  amount_replenish: number;
  amount_refund: number;
  /** RFC3339 timestamp */
  created_at: string;
}

export interface PaginationMeta {
  page: number;
  page_size: number;
  total_rows: number;
  total_pages: number;
}

export interface DmaaForecastResponse {
  data: DmaaForecastRecord[];
  pagination: PaginationMeta;
}

export interface DmaaForecastParams {
  page: number;
  pageSize: number;
  dateFrom: string;
  dateTo: string;
  terminalId: string;
  sortBy: string;
  sortOrder: "asc" | "desc";
}

export const DMAA_FORECAST_SORT_COLUMNS = [
  "terminal_id",
  "dmaa_file_id",
  "periode_pred",
  "denom",
  "amount_replenish",
  "amount_refund",
  "created_at",
] as const;

export const DMAA_FORECAST_DEFAULT_SORT_BY = "periode_pred";
export const DMAA_FORECAST_DEFAULT_SORT_ORDER: "asc" | "desc" = "desc";
