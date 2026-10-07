/**
 * CSV export / template / import client for master data
 * (backend admin_master_data_{export,import}_handler.go, mounted at
 * /api/v1/admin/master-data). The shared `api` client is JSON-only, so files
 * go through fetch directly with the same Bearer token.
 */

import { apiConfig } from "@/lib/api/config";
import { useAuthStore } from "@/lib/auth/store";

export const IO_ENTITIES = [
  { id: "vendors", label: "Vendor" },
  { id: "vendor-branches", label: "Cabang vendor" },
  { id: "vendor-vaults", label: "Vault vendor" },
  { id: "vendor-pics", label: "PIC vendor" },
  { id: "atms", label: "ATM" },
  { id: "atm-assignments", label: "Kelolaan ATM" },
] as const;

export type IoEntity = (typeof IO_ENTITIES)[number]["id"];
export type ExportStatus = "active" | "disabled" | "all";

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
}

export interface ImportConfirmResult {
  batch_id: number;
  rows: number;
  approval_request_id: number;
  existing: boolean;
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

function authHeaders(): Record<string, string> {
  const token = useAuthStore.getState().accessToken;
  return token ? { Authorization: `Bearer ${token}` } : {};
}

async function toError(res: Response): Promise<ImportApiError> {
  try {
    const body = (await res.json()) as { message?: string; errors?: ImportRowError[] };
    return new ImportApiError(body.message ?? res.statusText, res.status, body.errors ?? []);
  } catch {
    return new ImportApiError(res.statusText || "Permintaan gagal", res.status);
  }
}

async function download(path: string, fallbackName: string): Promise<void> {
  const res = await fetch(`${apiConfig.baseURL}${path}`, {
    headers: authHeaders(),
    credentials: "include",
  });
  if (!res.ok) throw await toError(res);
  const match = /filename="?([^";]+)"?/.exec(res.headers.get("Content-Disposition") ?? "");
  const url = URL.createObjectURL(await res.blob());
  const a = document.createElement("a");
  a.href = url;
  a.download = match?.[1] ?? fallbackName;
  a.click();
  URL.revokeObjectURL(url);
}

export function downloadExport(entity: IoEntity, status: ExportStatus): Promise<void> {
  return download(`${BASE}/export/${entity}?status=${status}`, `master-data-${entity}.csv`);
}

export function downloadTemplate(entity: IoEntity): Promise<void> {
  return download(`${BASE}/export/${entity}/template`, `master-data-${entity}-template.csv`);
}

async function upload<T>(path: string, file: File): Promise<T> {
  const form = new FormData();
  form.append("file", file);
  const res = await fetch(`${apiConfig.baseURL}${path}`, {
    method: "POST",
    headers: authHeaders(),
    credentials: "include",
    body: form,
  });
  if (!res.ok) throw await toError(res);
  return (await res.json()) as T;
}

export function dryRunImport(entity: IoEntity, file: File): Promise<ImportPreview> {
  return upload<ImportPreview>(`${BASE}/import/${entity}/dry-run`, file);
}

export function confirmImport(entity: IoEntity, file: File): Promise<ImportConfirmResult> {
  return upload<ImportConfirmResult>(`${BASE}/import/${entity}/confirm`, file);
}
