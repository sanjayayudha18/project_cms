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
  ExportRequestParams,
  ListExportJobsResponse,
  MasterDataEntity,
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

/** Dispatches on `params.kind` (used by useExportFlow). */
export function requestExport(p: ExportRequestParams): Promise<RequestExportResult> {
  switch (p.kind) {
    case "vendor":
      return requestVendorExport(p.vendorId, p.entity, p.status);
    case "cabang":
      return requestBranchExport(p.vendorId, p.branchId, p.entity, p.status);
    case "harga_paket":
      return requestPackagePriceExport(p.vendorId, p);
    case "master_data_global":
      return requestMasterDataExport(p.entity, p.status);
    case "report":
      return requestReportExport(p.report, p.params);
  }
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
