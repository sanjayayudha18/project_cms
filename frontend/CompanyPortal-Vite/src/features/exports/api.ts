/**
 * Async export client (backend-exq, `/api/v1/exports/*`, routed to :8081 by Vite/Nginx).
 * Uses the shared `api` client so a 401 triggers the single-flight token refresh.
 * Errors are the shared `ApiError` ({status, code?, message}); 429 `too_many_exports`
 * is detected with `isTooManyExports`.
 */

import { type ApiError, api } from "@/lib/api";
import { saveBlobResponse } from "@/lib/download";
import type {
  BranchEntity,
  ExportJob,
  ExportPreview,
  ExportRequestParams,
  ListExportJobsResponse,
  MasterDataEntity,
  MasterExportParams,
  ReportKey,
  ReportParams,
  RequestExportResult,
  VendorEntity,
} from "./types";

type JobEnvelope = { success: true; data: ExportJob };

function qs(params: Record<string, string | number | undefined>): string {
  const q = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) if (v !== undefined) q.set(k, String(v));
  const s = q.toString();
  return s ? `?${s}` : "";
}

async function post(path: string): Promise<RequestExportResult> {
  const res = await api.post<JobEnvelope>(path);
  return { job: res.data.data, created: res.status === 202 };
}

export const requestVendorExport = (
  vendorId: number,
  entity: VendorEntity,
  status?: string,
): Promise<RequestExportResult> => post(`/exports/vendors/${vendorId}/${entity}${qs({ status })}`);

export const requestBranchExport = (
  vendorId: number,
  branchId: number,
  entity: BranchEntity,
  status?: string,
): Promise<RequestExportResult> =>
  post(`/exports/vendors/${vendorId}/branches/${branchId}/${entity}${qs({ status })}`);

export const requestPackagePriceExport = (
  vendorId: number,
  opts: { branchId?: number; priceLevel?: string; status?: string } = {},
): Promise<RequestExportResult> =>
  post(
    `/exports/vendors/${vendorId}/package-prices${qs({
      branch_id: opts.branchId,
      price_level: opts.priceLevel,
      status: opts.status,
    })}`,
  );

export const requestMasterDataExport = (
  entity: MasterDataEntity,
  status?: string,
): Promise<RequestExportResult> => post(`/exports/master-data/${entity}${qs({ status })}`);

/** Report filters travel under the legacy query names (same keys as `ReportParams`). */
export const requestReportExport = (
  report: ReportKey,
  params: ReportParams,
): Promise<RequestExportResult> => post(`/exports/reports/${report}${qs({ ...params })}`);

/**
 * Request path (+ query) of a master-data export, without the `/exports` prefix.
 * Shared by the POST request and the GET preview so both validate the same filter.
 */
export function masterPath(p: MasterExportParams): string {
  switch (p.kind) {
    case "vendor":
      return `/vendors/${p.vendorId}/${p.entity}${qs({ status: p.status })}`;
    case "cabang":
      return `/vendors/${p.vendorId}/branches/${p.branchId}/${p.entity}${qs({ status: p.status })}`;
    case "harga_paket":
      return `/vendors/${p.vendorId}/package-prices${qs({
        branch_id: p.branchId,
        price_level: p.priceLevel,
        status: p.status,
      })}`;
    case "master_data_global":
      return `/master-data/${p.entity}${qs({ status: p.status })}`;
  }
}

/** Dispatches on `params.kind` (used by useExportFlow). */
export function requestExport(p: ExportRequestParams): Promise<RequestExportResult> {
  if (p.kind === "report") return requestReportExport(p.report, p.params);
  return post(`/exports${masterPath(p)}`);
}

/** One keyset page of the table that `requestExport(p)` would export. */
export async function previewExport(
  p: MasterExportParams,
  after: number | undefined,
  limit: number,
): Promise<ExportPreview> {
  const path = masterPath(p);
  const paging = qs({ after, limit }).slice(1);
  const res = await api.get<{ success: true; data: ExportPreview }>(
    `/exports/preview${path}${path.includes("?") ? "&" : "?"}${paging}`,
  );
  return res.data.data;
}

export async function listMyExports(limit?: number): Promise<ExportJob[]> {
  const res = await api.get<ListExportJobsResponse>(`/exports/jobs${qs({ limit })}`);
  return res.data.data;
}

export async function getExportJob(id: number): Promise<ExportJob> {
  const res = await api.get<JobEnvelope>(`/exports/jobs/${id}`);
  return res.data.data;
}

/** One-shot download: the server deletes the file afterwards (409/410 when gone). */
export async function downloadExportFile(id: number, fallbackName = `export-${id}.csv`) {
  const res = await api.get<Blob>(`/exports/jobs/${id}/file`, { responseType: "blob" });
  saveBlobResponse(res, fallbackName);
}

export async function downloadMasterDataTemplate(entity: MasterDataEntity) {
  const res = await api.get<Blob>(`/exports/master-data/${entity}/template`, {
    responseType: "blob",
  });
  saveBlobResponse(res, `${entity}-template.csv`);
}

export const isApiError = (e: unknown): e is ApiError =>
  typeof e === "object" && e !== null && typeof (e as ApiError).status === "number";

/** 429 from the per-user active-export cap (code `too_many_exports`). */
export const isTooManyExports = (e: unknown): boolean =>
  isApiError(e) && e.status === 429 && (e.code === undefined || e.code === "too_many_exports");
