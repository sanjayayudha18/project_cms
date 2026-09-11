/**
 * Audit Log Viewer API types (design.md API contract).
 *
 * actor_id/entity_id are non-nullable numbers, not `number | null` as
 * design.md originally sketched — the actual audit_logs table
 * (backend/migrations/023_audit_logs.sql) declares both NOT NULL.
 */

export interface AuditLogListItem {
  id: number;
  actor_id: number;
  action: string;
  entity_type: string;
  entity_id: number;
  ip: string | null;
  /** RFC3339 timestamp (UTC) */
  created_at: string;
}

export interface AuditLogListResponse {
  data: AuditLogListItem[];
  page: number;
  page_size: number;
  total: number;
}

export interface AuditLogDetail extends AuditLogListItem {
  before: Record<string, unknown> | null;
  after: Record<string, unknown> | null;
}

export interface AuditLogFilters {
  actor_id: string | null;
  action: string | null;
  entity_type: string | null;
  entity_id: string | null;
  /** YYYY-MM-DD (Asia/Jakarta) */
  date_from: string | null;
  date_to: string | null;
}

export const EMPTY_AUDIT_LOG_FILTERS: AuditLogFilters = {
  actor_id: null,
  action: null,
  entity_type: null,
  entity_id: null,
  date_from: null,
  date_to: null,
};

export type BadgeVariant = "success" | "warning" | "danger" | "info" | "neutral";

/**
 * action -> badge variant. Suffix convention: *.delete/reject/fail -> danger,
 * *.approve/complete/create -> success, *.submit/request -> info,
 * *.update/edit -> warning, else neutral.
 *
 * Heuristic on the `action` string convention. As real action names
 * stabilize (from the internal/audit writer + module wiring), replace the
 * regex with an explicit lookup table. Every badge still renders icon +
 * label (AuditLogTable, Task 7), so a wrong color never hides meaning.
 */
export function actionBadgeVariant(action: string): BadgeVariant {
  if (/\.(delete|reject|fail)/.test(action)) return "danger";
  if (/\.(approve|complete|create)/.test(action)) return "success";
  if (/\.(submit|request)/.test(action)) return "info";
  if (/\.(update|edit)/.test(action)) return "warning";
  return "neutral";
}
