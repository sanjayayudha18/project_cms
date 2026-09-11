/**
 * TanStack Query hooks for the Audit Log Viewer
 * (GET /api/v1/audit-logs, GET /api/v1/audit-logs/{id}).
 */

import type { ApiError } from "@/lib/api/client";
import { api } from "@/lib/api/client";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import type { AuditLogDetail, AuditLogFilters, AuditLogListResponse } from "../types";

export const auditKeys = {
  all: ["audit-log"] as const,
  list: (filters: AuditLogFilters, page: number, pageSize: number) =>
    [...auditKeys.all, "list", filters, page, pageSize] as const,
  detail: (id: number | null) => [...auditKeys.all, "detail", id] as const,
};

function buildListQueryString(filters: AuditLogFilters, page: number, pageSize: number): string {
  const params = new URLSearchParams();
  params.set("page", String(page));
  params.set("page_size", String(pageSize));
  for (const [key, value] of Object.entries(filters)) {
    if (value) params.set(key, value);
  }
  return params.toString();
}

async function fetchAuditLogList(
  filters: AuditLogFilters,
  page: number,
  pageSize: number,
): Promise<AuditLogListResponse> {
  const { data } = await api.get<AuditLogListResponse>(
    `/audit-logs?${buildListQueryString(filters, page, pageSize)}`,
  );
  return data;
}

export function useAuditLogList(filters: AuditLogFilters, page: number, pageSize: number) {
  return useQuery<AuditLogListResponse, ApiError>({
    queryKey: auditKeys.list(filters, page, pageSize),
    queryFn: () => fetchAuditLogList(filters, page, pageSize),
    placeholderData: keepPreviousData,
  });
}

async function fetchAuditLogDetail(id: number): Promise<AuditLogDetail> {
  const { data } = await api.get<AuditLogDetail>(`/audit-logs/${id}`);
  return data;
}

export function useAuditLogDetail(id: number | null) {
  return useQuery<AuditLogDetail, ApiError>({
    queryKey: auditKeys.detail(id),
    // biome-ignore lint/style/noNonNullAssertion: enabled: id !== null guarantees id is non-null when this runs.
    queryFn: () => fetchAuditLogDetail(id!),
    enabled: id !== null,
  });
}
