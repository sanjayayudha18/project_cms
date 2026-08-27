import { api } from "@/lib/api/client";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type {
  ApiEnvelope,
  AuditLogEntry,
  EodSummary,
  FileHistoryResponse,
  FileStatusResponse,
  LateDetectionItem,
  ManualRetryResponse,
} from "../types";

/** Unwraps an ApiEnvelope, throwing on `status: "error"` instead of asserting `data` is non-null. */
function unwrap<T>(envelope: ApiEnvelope<T>): T {
  if (envelope.status === "error" || envelope.data === null) {
    throw new Error(envelope.error ?? "Unknown error");
  }
  return envelope.data;
}

// ─── Query Keys ───────────────────────────────────────────────────────────────

export const eodKeys = {
  all: ["eod"] as const,
  summary: (date: string) => [...eodKeys.all, "summary", date] as const,
  status: (date: string) => [...eodKeys.all, "status", date] as const,
  history: (fileId: string) => [...eodKeys.all, "history", fileId] as const,
  late: (date: string) => [...eodKeys.all, "late", date] as const,
  audit: (date: string, fileType: string | null, trigger: string | null) =>
    [...eodKeys.all, "audit", date, fileType, trigger] as const,
};

// ─── Hooks ────────────────────────────────────────────────────────────────────

export function useEodSummary(processingDate: string, refetchInterval: number | false) {
  return useQuery({
    queryKey: eodKeys.summary(processingDate),
    queryFn: async () => {
      const res = await api.get<ApiEnvelope<EodSummary>>(
        `/api/eod/summary?processing_date=${processingDate}`,
      );
      return unwrap(res.data);
    },
    refetchInterval,
  });
}

export function useEodStatus(processingDate: string, refetchInterval: number | false) {
  return useQuery({
    queryKey: eodKeys.status(processingDate),
    queryFn: async () => {
      const res = await api.get<ApiEnvelope<FileStatusResponse>>(
        `/api/eod/status?processing_date=${processingDate}`,
      );
      return unwrap(res.data);
    },
    refetchInterval,
  });
}

export function useFileHistory(fileId: string | null) {
  return useQuery({
    queryKey: eodKeys.history(fileId ?? ""),
    queryFn: async () => {
      const res = await api.get<ApiEnvelope<FileHistoryResponse>>(
        `/api/eod/status/${fileId}/history`,
      );
      return unwrap(res.data);
    },
    enabled: !!fileId,
  });
}

export function useEodLate(processingDate: string, refetchInterval: number | false) {
  return useQuery({
    queryKey: eodKeys.late(processingDate),
    queryFn: async () => {
      const res = await api.get<ApiEnvelope<LateDetectionItem[]>>(
        `/api/eod/late?processing_date=${processingDate}`,
      );
      return unwrap(res.data);
    },
    refetchInterval,
  });
}

export function useEodAudit(
  processingDate: string,
  fileType: string | null,
  trigger: string | null,
  refetchInterval: number | false,
) {
  return useQuery({
    queryKey: eodKeys.audit(processingDate, fileType, trigger),
    queryFn: async () => {
      const params = new URLSearchParams({ processing_date: processingDate });
      if (fileType) params.set("file_type", fileType);
      if (trigger) params.set("trigger", trigger);

      const res = await api.get<ApiEnvelope<AuditLogEntry[]>>(`/api/eod/audit?${params}`);
      return unwrap(res.data);
    },
    refetchInterval,
  });
}

export function useRetryFile() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (fileId: string) => {
      const res = await api.post<ApiEnvelope<ManualRetryResponse>>(`/api/eod/retry/${fileId}`);
      return unwrap(res.data);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: eodKeys.all });
    },
  });
}

/** Extracts a display message from an unknown error (thrown ApiError or Error). */
export function getErrorMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  if (
    typeof error === "object" &&
    error !== null &&
    "message" in error &&
    typeof (error as { message: unknown }).message === "string"
  ) {
    return (error as { message: string }).message;
  }
  return "Terjadi kesalahan yang tidak diketahui";
}
