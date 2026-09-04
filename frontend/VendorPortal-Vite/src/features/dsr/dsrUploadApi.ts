// ─── DSR Upload API ────────────────────────────────────────────────────────
// Two-phase flow: POST /uploads (dry-run parse, nothing written to DB yet) ->
// vendor reviews the parsed preview -> POST /uploads/confirm (re-parses the
// staged file server-side and actually persists it). See
// .claude/sdlc/vendor-upload-dsr/ for the full design.

import { api } from "@/lib/api/client";

export interface DsrDailyRow {
  row_no: number;
  section: string;
  flow: string;
  line_label: string;
  memo_no: string | null;
  denom_100k: string;
  denom_50k: string;
  denom_20k: string;
  denom_10k: string;
  denom_5k: string;
  denom_2k: string;
  denom_1k: string;
  line_total_idr: string | null;
  remarks: string | null;
}

export interface DsrRencanaIsiRow {
  row_no: number;
  atm_terminal_id: string;
  atm_location: string | null;
  denom_config: string | null;
  fill_100k_idr: string;
  fill_50k_idr: string;
  splank_balance_0800_idr: string;
  remarks: string | null;
}

export interface DsrDailyPreview {
  error?: string;
  fields?: Record<string, unknown>;
  rows?: DsrDailyRow[];
  error_count?: number;
}

export interface DsrRencanaIsiPreview {
  error?: string;
  plan_date?: string;
  rows?: DsrRencanaIsiRow[];
}

export interface DsrDryRunResponse {
  mode: "dry_run";
  filename: string;
  original_filename: string;
  checksum: string;
  vendor_code: string;
  vendor_name: string;
  uploaded_by_user_id: number | null;
  staged_filename: string;
  daily: DsrDailyPreview;
  rencana_isi: DsrRencanaIsiPreview;
}

export interface DsrRowErrorResponse {
  row_no: number;
  label: string;
}

export interface DsrSheetResponse {
  file_id: number | null;
  status: string;
  row_count: number;
  success_count: number;
  error_count: number;
  errors: DsrRowErrorResponse[];
}

export interface DsrConfirmResponse {
  checksum: string;
  daily: DsrSheetResponse;
  rencana_isi: DsrSheetResponse;
}

export async function uploadDsrFile(file: File): Promise<DsrDryRunResponse> {
  const formData = new FormData();
  formData.append("file", file);
  const { data } = await api.post<DsrDryRunResponse>("/api/v1/dsr/uploads", formData);
  return data;
}

export async function confirmDsrUpload(
  stagedFilename: string,
  checksum: string,
): Promise<DsrConfirmResponse> {
  const { data } = await api.post<DsrConfirmResponse>("/api/v1/dsr/uploads/confirm", {
    staged_filename: stagedFilename,
    checksum,
  });
  return data;
}

// ─── List uploads (DSR Monitor) ────────────────────────────────────────────

export interface DsrUploadSheetSummary {
  file_id: number;
  status: string;
}

export interface DsrUploadListItem {
  report_date: string;
  daily: DsrUploadSheetSummary | null;
  rencana_isi: DsrUploadSheetSummary | null;
}

export interface DsrUploadListResponse {
  data: DsrUploadListItem[];
  total: number;
  page: number;
  page_size: number;
}

export async function listDsrUploads(params: {
  dateFrom?: string;
  dateTo?: string;
}): Promise<DsrUploadListResponse> {
  const query = new URLSearchParams();
  if (params.dateFrom) query.set("date_from", params.dateFrom);
  if (params.dateTo) query.set("date_to", params.dateTo);
  const qs = query.toString();
  const { data } = await api.get<DsrUploadListResponse>(
    `/api/v1/dsr/uploads${qs ? `?${qs}` : ""}`,
  );
  return data;
}

// ─── Sheet detail (view uploaded rows) ─────────────────────────────────────

export interface DsrSheetDetailDailyRow {
  row_no: number;
  section: string;
  flow: string;
  line_label: string;
  memo_no: string | null;
  denom_100k: number | null;
  denom_50k: number | null;
  denom_20k: number | null;
  denom_10k: number | null;
  denom_5k: number | null;
  denom_2k: number | null;
  denom_1k: number | null;
  line_total_idr: number | null;
  remarks: string | null;
}

export interface DsrSheetDetailRencanaIsiRow {
  row_no: number;
  atm_terminal_id: string;
  atm_location: string | null;
  denom_config: string | null;
  fill_100k_idr: number | null;
  fill_50k_idr: number | null;
  splank_balance_0800_idr: number | null;
  remarks: string | null;
}

export interface DsrSheetDetailResponse {
  file_id: number;
  filename: string;
  status: string;
  report_date: string | null;
  row_count: number;
  success_count: number;
  error_count: number;
  errors: DsrRowErrorResponse[];
  daily_rows?: DsrSheetDetailDailyRow[];
  rencana_isi_rows?: DsrSheetDetailRencanaIsiRow[];
}

export async function getDsrDailyDetail(fileId: number): Promise<DsrSheetDetailResponse> {
  const { data } = await api.get<DsrSheetDetailResponse>(`/api/v1/dsr/uploads/daily/${fileId}`);
  return data;
}

export async function getDsrRencanaIsiDetail(fileId: number): Promise<DsrSheetDetailResponse> {
  const { data } = await api.get<DsrSheetDetailResponse>(
    `/api/v1/dsr/uploads/rencana-isi/${fileId}`,
  );
  return data;
}
