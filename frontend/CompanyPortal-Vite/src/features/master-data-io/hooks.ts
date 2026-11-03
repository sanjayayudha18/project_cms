/**
 * TanStack Query hooks for the CSV import history (master-data-import-revamp Req 5, 6).
 * A retry-apply can change the batch status and the underlying master data, so it
 * refreshes the history list and every cached batch detail.
 */

import type { ApiError } from "@/lib/api/client";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  type ImportBatchDetail,
  type ImportBatchFilter,
  type ImportBatchPage,
  getImportBatch,
  listImportBatches,
  retryApplyImport,
} from "./api";

export const masterDataImportKeys = {
  all: ["master-data-import"] as const,
  list: (filter: ImportBatchFilter) => [...masterDataImportKeys.all, "list", filter] as const,
  detail: (id: number, page = 1) => [...masterDataImportKeys.all, "detail", id, page] as const,
};

export function useImportBatches(filter: ImportBatchFilter) {
  return useQuery<ImportBatchPage, ApiError>({
    queryKey: masterDataImportKeys.list(filter),
    queryFn: () => listImportBatches(filter),
  });
}

/** `id` null = no batch open (the dialog is closed), so nothing is fetched. */
export function useImportBatch(id: number | null, page = 1, pageSize?: number) {
  return useQuery<ImportBatchDetail, ApiError>({
    queryKey: masterDataImportKeys.detail(id ?? 0, page),
    queryFn: () => getImportBatch(id as number, page, pageSize),
    enabled: id !== null,
  });
}

export function useRetryApply() {
  const queryClient = useQueryClient();
  // The retry may fail with 409 after changing the batch state; refresh either way.
  const refresh = () => queryClient.invalidateQueries({ queryKey: masterDataImportKeys.all });
  return useMutation<void, ApiError, number>({
    mutationFn: retryApplyImport,
    onSuccess: refresh,
    onError: refresh,
  });
}
