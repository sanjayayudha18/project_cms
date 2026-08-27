import type { LucideIcon } from "lucide-react";
import { AlertTriangle, CheckCircle, Clock, Loader, XCircle } from "lucide-react";

// ─── API Response Envelope ────────────────────────────────────────────────────

export interface ApiEnvelope<T> {
  status: "success" | "error";
  data: T | null;
  error: string | null;
}

// ─── Summary ──────────────────────────────────────────────────────────────────

export interface SummaryCounts {
  pending: number;
  processing: number;
  completed: number;
  failed: number;
  max_retries_exhausted: number;
  late: number;
}

export interface EodSummary {
  processing_date: string;
  counts: SummaryCounts;
  by_file_type: Record<FileType, Partial<SummaryCounts>>;
}

// ─── File Status ──────────────────────────────────────────────────────────────

export type FileType = "dmaa" | "itm_cashpos" | "itm_replenish";

export type ProcessingStatus =
  | "pending"
  | "processing"
  | "completed"
  | "failed"
  | "max_retries_exhausted";

export interface FileStatusItem {
  file_id: string;
  filename: string;
  checksum: string;
  processing_status: ProcessingStatus;
  retry_count: number;
  max_retries_exhausted: boolean;
  detected_at: string;
  last_retry_at: string | null;
  failure_reason: string | null;
}

export interface FileStatusResponse {
  processing_date: string;
  by_file_type: Record<FileType, FileStatusItem[]>;
}

export interface FileStatusRow extends FileStatusItem {
  file_type: FileType;
}

export function flattenFileStatus(response: FileStatusResponse): FileStatusRow[] {
  const rows: FileStatusRow[] = [];
  for (const [fileType, files] of Object.entries(response.by_file_type)) {
    for (const file of files) {
      rows.push({ ...file, file_type: fileType as FileType });
    }
  }
  return rows;
}

// ─── File History ─────────────────────────────────────────────────────────────

export type TriggerType = "auto" | "manual";

export interface RetryAttempt {
  attempt_number: number;
  trigger: TriggerType;
  started_at: string;
  completed_at: string | null;
  outcome: "completed" | "failed" | null;
  duration_ms: number | null;
  error_detail: string | null;
}

export interface FileHistoryResponse {
  file_id: string;
  filename: string;
  file_type: FileType;
  attempts: RetryAttempt[];
}

// ─── Manual Retry ─────────────────────────────────────────────────────────────

export interface ManualRetryResponse {
  job_id: string;
  file_id: string;
  processing_status: ProcessingStatus;
  triggered_by: string;
}

// ─── Late Detection ───────────────────────────────────────────────────────────

export interface LateDetectionItem {
  id: string;
  file_type: FileType;
  processing_date: string;
  sla_deadline: string;
  detected_at: string;
  resolved_at: string | null;
  is_resolved: boolean;
}

// ─── Audit Log ────────────────────────────────────────────────────────────────

export interface AuditLogEntry {
  id: string;
  event_type: string;
  trigger: TriggerType;
  file_id: string | null;
  file_type: FileType;
  file_checksum: string | null;
  processing_date: string;
  initiated_by: string;
  outcome: "completed" | "failed" | null;
  duration_ms: number | null;
  error_detail: string | null;
  created_at: string;
}

// ─── Badge Mapping ────────────────────────────────────────────────────────────

export type BadgeVariant = "success" | "warning" | "danger" | "info" | "neutral";

export const STATUS_BADGE_MAP: Record<ProcessingStatus, BadgeVariant> = {
  completed: "success",
  failed: "danger",
  max_retries_exhausted: "danger",
  processing: "info",
  pending: "neutral",
};

export const TRIGGER_BADGE_MAP: Record<TriggerType, BadgeVariant> = {
  auto: "info",
  manual: "neutral",
};

export const OUTCOME_BADGE_MAP: Record<string, BadgeVariant> = {
  completed: "success",
  failed: "danger",
  in_progress: "info",
};

// ─── Display Labels ───────────────────────────────────────────────────────────

export const STATUS_LABELS: Record<ProcessingStatus, string> = {
  completed: "Completed",
  failed: "Failed",
  max_retries_exhausted: "Max Retries",
  processing: "Processing",
  pending: "Pending",
};

export const FILE_TYPE_LABELS: Record<FileType, string> = {
  dmaa: "DMAA",
  itm_cashpos: "ITM Cash Position",
  itm_replenish: "ITM Replenishment",
};

export const TRIGGER_LABELS: Record<TriggerType, string> = {
  auto: "Auto",
  manual: "Manual",
};

// ─── Status-to-Badge Config (icon + variant + label) ──────────────────────────

interface StatusBadgeConfig {
  variant: BadgeVariant;
  icon: LucideIcon;
  label: string;
}

export const STATUS_BADGE_CONFIG: Record<ProcessingStatus, StatusBadgeConfig> = {
  completed: { variant: "success", icon: CheckCircle, label: "Completed" },
  failed: { variant: "danger", icon: XCircle, label: "Failed" },
  max_retries_exhausted: { variant: "danger", icon: AlertTriangle, label: "Max Retries" },
  processing: { variant: "info", icon: Loader, label: "Processing" },
  pending: { variant: "neutral", icon: Clock, label: "Pending" },
};
