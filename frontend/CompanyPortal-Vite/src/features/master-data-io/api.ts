/**
 * CSV import client for master data (backend admin_master_data_import_handler.go, mounted at
 * /api/v1/admin/master-data). Export + template moved to features/exports (backend-exq).
 * Goes through the shared `api` client so a 401 triggers the single-flight token refresh.
 */

import type { MasterDataChange } from "@/features/approvals/api";
import { type ApiError, api } from "@/lib/api";

export const IO_ENTITIES = [
  { id: "vendors", label: "Vendor" },
  { id: "vendor-branches", label: "Cabang vendor" },
  { id: "vendor-vaults", label: "Vault vendor" },
  { id: "vendor-pics", label: "PIC vendor" },
  { id: "atms", label: "ATM" },
  { id: "atm-assignments", label: "Kelolaan ATM" },
] as const;

export type IoEntity = (typeof IO_ENTITIES)[number]["id"];

/** Mirrors backend MasterDataImportMaxBytes / MasterDataImportMaxRows. */
export const IMPORT_MAX_BYTES = 5 * 1024 * 1024;
export const IMPORT_MAX_ROWS = 2_000;

export interface ImportRowError {
  row: number;
  field: string;
  message: string;
}

export interface ImportPreview {
  entity: string;
  total_rows: number;
  valid_rows: number;
  creates: number;
  updates: number;
  unchanged: number;
  errors: ImportRowError[];
  errors_truncated: boolean;
  /** Updates that are not an enable/disable toggle; `updates` = data_updates + enables + disables. */
  data_updates: number;
  enables: number;
  disables: number;
  /** One entry per create/update/enable/disable row; empty whenever `errors` is not. */
  changes: ImportChange[];
}

export type ImportChangeOp = "create" | "update" | "enable" | "disable";

export interface ImportFieldChange {
  field: string;
  /** "" for a create. */
  before: string;
  after: string;
}

export interface ImportChange {
  /** File line, header = 1. */
  row: number;
  op: ImportChangeOp;
  key: string;
  fields: ImportFieldChange[];
}

export interface ImportConfirmResult {
  batch_id: number;
  rows: number;
  /** Null when the batch is staged but its approval request was not created yet. */
  approval_request_id: number | null;
  existing: boolean;
}

/** Raw head Change_Row status of a batch; labels live in batchStatus.ts. */
export type ImportBatchStatusCode = "pending" | "approved" | "rejected" | "applied" | "stale";

export interface ImportBatchSummary {
  id: number;
  entity: IoEntity;
  /** Null for batches imported before migration 036. */
  file_name: string | null;
  row_count: number;
  maker_id: number;
  maker_name: string;
  created_at: string;
  status: ImportBatchStatusCode;
  approval_request_id: number | null;
  error: string | null;
  decided_at: string | null;
  decided_by_name: string | null;
}

export interface ImportBatchPage {
  items: ImportBatchSummary[];
  total: number;
  page: number;
  page_size: number;
}

export interface ImportBatchFilter {
  entity?: IoEntity;
  status?: ImportBatchStatusCode;
  /** Only batches submitted by the caller. */
  mine?: boolean;
  page?: number;
  page_size?: number;
}

/** A batch Change_Row; `failed` marks the row that made the whole batch roll back. */
export interface ImportBatchChange extends MasterDataChange {
  error: string | null;
  failed: boolean;
}

export interface ImportBatchDetail extends ImportBatchSummary {
  counts: Record<ImportChangeOp, number>;
  changes: ImportBatchChange[];
  /** Error of the first failed row of the whole batch (any page); null for old batches. */
  failure_reason: string | null;
  total: number;
  page: number;
  page_size: number;
}

/** Error carrying the HTTP status and, for 422, the per-row errors. */
export class ImportApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
    readonly errors: ImportRowError[] = [],
  ) {
    super(message);
  }
}

const BASE = "/admin/master-data";

/** Re-wraps the shared client's ApiError so callers keep ImportApiError + row errors. */
function toImportError(err: unknown): unknown {
  const e = err as Partial<ApiError> | null;
  if (typeof e?.status !== "number") return err;
  const rows = Array.isArray(e.details) ? (e.details as ImportRowError[]) : [];
  return new ImportApiError(e.message || "Permintaan gagal", e.status, rows);
}

async function upload<T>(path: string, file: File): Promise<T> {
  const form = new FormData();
  form.append("file", file);
  try {
    return (await api.post<T>(path, form)).data;
  } catch (err) {
    throw toImportError(err);
  }
}

export function dryRunImport(entity: IoEntity, file: File): Promise<ImportPreview> {
  return upload<ImportPreview>(`${BASE}/import/${entity}/dry-run`, file);
}

export function confirmImport(entity: IoEntity, file: File): Promise<ImportConfirmResult> {
  return upload<ImportConfirmResult>(`${BASE}/import/${entity}/confirm`, file);
}

export async function listImportBatches(filter: ImportBatchFilter = {}): Promise<ImportBatchPage> {
  const q = new URLSearchParams();
  if (filter.entity) q.set("entity", filter.entity);
  if (filter.status) q.set("status", filter.status);
  if (filter.mine) q.set("mine", "true");
  if (filter.page) q.set("page", String(filter.page));
  if (filter.page_size) q.set("page_size", String(filter.page_size));
  const qs = q.toString();
  const { data } = await api.get<ImportBatchPage>(`${BASE}/import/batches${qs ? `?${qs}` : ""}`);
  return data;
}

export async function getImportBatch(
  id: number,
  page?: number,
  pageSize?: number,
): Promise<ImportBatchDetail> {
  const q = new URLSearchParams();
  if (page) q.set("page", String(page));
  if (pageSize) q.set("page_size", String(pageSize));
  const qs = q.toString();
  const { data } = await api.get<ImportBatchDetail>(
    `${BASE}/import/batches/${id}${qs ? `?${qs}` : ""}`,
  );
  return data;
}

/** Re-applies an approved batch whose commit failed (POST /approvals/{id}/retry-apply). */
export async function retryApplyImport(approvalRequestId: number): Promise<void> {
  await api.post(`/approvals/${approvalRequestId}/retry-apply`);
}
