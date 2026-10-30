/**
 * Laporan DSR telat / tidak kirim (dsr-late-report FR6) -- backend/internal/handler/dsr_late_report_handler.go
 * (/api/v1/dsr-late-reports; ATM-USER, ATM-SPV, BRANCH-ATM-SPV, ADMIN). Status is computed live on read.
 */

import type { ApiError } from "@/lib/api/client";
import { api } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { keepPreviousData, useQuery } from "@tanstack/react-query";

type S = components["schemas"];
export type DsrLateStatus = S["DsrLateStatus"];
export type DsrLateDailyRow = S["DsrLateDailyRow"];
export type DsrLateDailyList = S["DsrLateDailyList"];
export type DsrLateSummaryRow = S["DsrLateSummaryRow"];
export type DsrLateSummary = S["DsrLateSummary"];
export type DsrLateWarnings = S["DsrLateWarnings"];

export const PAGE_SIZE = 20;

export const STATUS_LABELS: Record<DsrLateStatus, string> = {
  ok: "OK",
  late: "TELAT",
  not_sent: "TIDAK KIRIM",
  pending: "BELUM JATUH TEMPO",
};

export interface ReportFilter {
  month: string;
  vendorId: number | null;
  vendorBranchId: number | null;
}

export interface DailyFilter extends ReportFilter {
  status: DsrLateStatus | null;
}

const BASE = "/dsr-late-reports";

function query(f: ReportFilter & { status?: DsrLateStatus | null }, page?: number): string {
  const q = new URLSearchParams({ month: f.month });
  if (f.vendorId !== null) q.set("vendor_id", String(f.vendorId));
  if (f.vendorBranchId !== null) q.set("vendor_branch_id", String(f.vendorBranchId));
  if (f.status) q.set("status", f.status);
  if (page) {
    q.set("page", String(page));
    q.set("page_size", String(PAGE_SIZE));
  }
  return q.toString();
}

export function useDsrLateDaily(f: DailyFilter, page: number) {
  return useQuery<DsrLateDailyList, ApiError>({
    queryKey: ["dsr-late-report", "daily", f, page],
    queryFn: async () => (await api.get<DsrLateDailyList>(`${BASE}/daily?${query(f, page)}`)).data,
    placeholderData: keepPreviousData,
    retry: false,
  });
}

export function useDsrLateSummary(f: ReportFilter) {
  return useQuery<DsrLateSummary, ApiError>({
    queryKey: ["dsr-late-report", "summary", f],
    queryFn: async () => (await api.get<DsrLateSummary>(`${BASE}/summary?${query(f)}`)).data,
    placeholderData: keepPreviousData,
    retry: false,
  });
}

export function useDsrLateWarnings(month: string, vendorId: number | null) {
  return useQuery<DsrLateWarnings, ApiError>({
    queryKey: ["dsr-late-report", "warnings", month, vendorId],
    queryFn: async () => {
      const q = new URLSearchParams({ month });
      if (vendorId !== null) q.set("vendor_id", String(vendorId));
      return (await api.get<DsrLateWarnings>(`${BASE}/warnings?${q}`)).data;
    },
    retry: false,
  });
}
