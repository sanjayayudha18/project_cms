/**
 * Realisasi pengisian vs order (replenishment-realisasi FR4, FR6) -- backend/internal/handler/replenishment_handler.go
 * (/api/v1/replenishment, Vendor Request viewer roles). Classifications are written by the Python job
 * replenish_classify; amounts are whole IDR integers.
 */

import type { ApiError } from "@/lib/api/client";
import { api } from "@/lib/api/client";
import { apiConfig } from "@/lib/api/config";
import { useAuthStore } from "@/lib/auth/store";
import { keepPreviousData, useQuery } from "@tanstack/react-query";

export type Classification =
  | "pending"
  | "on_schedule"
  | "early"
  | "late"
  | "not_done"
  | "no_order"
  | "void";

export interface ClassificationAmount {
  denom: number;
  order_amount: number;
  itm_amount: number;
  vendor_amount: number | null;
  diff: number;
}

export interface ClassificationView {
  id: number;
  ticket_number: string | null;
  request_id: number | null;
  request_number: string | null;
  terminal_id: string;
  vendor_id: number | null;
  vendor_name: string | null;
  branch_code: string | null;
  branch_name: string | null;
  region_code: string | null;
  scheduled_date: string | null;
  realized_date: string | null;
  realized_time: string | null;
  workday_offset: number | null;
  classification: Classification;
  is_final: boolean;
  has_amount_diff: boolean;
  vendor_mismatch: boolean;
  vendor_report_late: boolean;
  amounts: ClassificationAmount[];
  currency: string;
}

export interface ClassificationList {
  items: ClassificationView[];
  total: number;
  page: number;
  page_size: number;
}

export interface ClassCount {
  classification: Classification;
  total: number;
  amount_diff: number;
  vendor_mismatch: number;
  vendor_report_late: number;
}

export interface DailyInstruction {
  date: string | null;
  terminals: number;
  order_amount: number;
}

export interface ReplenishmentSummaryData {
  by_class: ClassCount[];
  instructions: DailyInstruction[];
  currency: string;
}

export interface ClassificationParams {
  dateFrom: string;
  dateTo: string;
  vendorId: number | null;
  regionCode: string | null;
  classification: Classification | null;
  onlyDiff: boolean;
  page: number;
}

/** Max range per query/export (K5). */
export const MAX_RANGE_DAYS = 31;
export const PAGE_SIZE = 20;

export const CLASSIFICATION_LABELS: Record<Classification, string> = {
  pending: "Menunggu",
  on_schedule: "Sesuai jadwal",
  early: "Maju",
  late: "Mundur",
  not_done: "Tidak dilakukan",
  no_order: "Tanpa order",
  void: "Batal",
};

/** Days between two YYYY-MM-DD dates, inclusive. */
export function rangeDays(from: string, to: string): number {
  return Math.round((Date.parse(to) - Date.parse(from)) / 86_400_000) + 1;
}

export function rangeError(from: string, to: string): string | null {
  if (!from || !to) return "Isi tanggal awal dan akhir";
  if (to < from) return "Tanggal akhir sebelum tanggal awal";
  if (rangeDays(from, to) > MAX_RANGE_DAYS) return `Rentang maksimal ${MAX_RANGE_DAYS} hari`;
  return null;
}

function query(p: Omit<ClassificationParams, "page"> & { page?: number }): string {
  const q = new URLSearchParams({ date_from: p.dateFrom, date_to: p.dateTo });
  if (p.vendorId !== null) q.set("vendor_id", String(p.vendorId));
  if (p.regionCode) q.set("region_code", p.regionCode);
  if (p.classification) q.set("classification", p.classification);
  if (p.onlyDiff) q.set("only_diff", "true");
  if (p.page) {
    q.set("page", String(p.page));
    q.set("page_size", String(PAGE_SIZE));
  }
  return q.toString();
}

export function useClassifications(p: ClassificationParams) {
  return useQuery<ClassificationList, ApiError>({
    queryKey: ["replenishment", "classifications", p],
    queryFn: async () =>
      (await api.get<ClassificationList>(`/replenishment/classifications?${query(p)}`)).data,
    enabled: rangeError(p.dateFrom, p.dateTo) === null,
    placeholderData: keepPreviousData,
  });
}

export function useReplenishmentSummary(dateFrom: string, dateTo: string) {
  return useQuery<ReplenishmentSummaryData, ApiError>({
    queryKey: ["replenishment", "summary", dateFrom, dateTo],
    queryFn: async () =>
      (
        await api.get<ReplenishmentSummaryData>(
          `/replenishment/summary?${new URLSearchParams({ date_from: dateFrom, date_to: dateTo })}`,
        )
      ).data,
  });
}

export type ReportKind = "amount-mismatch" | "trips";

/** Downloads a CSV report (the server audits the export). */
export async function downloadReport(
  kind: ReportKind,
  p: Omit<ClassificationParams, "page">,
): Promise<void> {
  const token = useAuthStore.getState().accessToken;
  const res = await fetch(
    `${apiConfig.baseURL}/replenishment/reports/${kind}?${query(p)}&format=csv`,
    {
      headers: token ? { Authorization: `Bearer ${token}` } : {},
      credentials: "include",
    },
  );
  if (!res.ok) {
    const body = (await res.json().catch(() => ({}))) as { message?: string };
    throw new Error(body.message ?? "Gagal mengunduh laporan");
  }
  const match = /filename="?([^";]+)"?/.exec(res.headers.get("Content-Disposition") ?? "");
  const url = URL.createObjectURL(await res.blob());
  const a = document.createElement("a");
  a.href = url;
  a.download = match?.[1] ?? `replenishment-${kind}.csv`;
  a.click();
  URL.revokeObjectURL(url);
}
