/**
 * Import batch status display map (master-data-import-revamp Req 5.4, 5.6, 6.3, 6.4).
 * Status codes are the head Change_Row status the backend returns raw; labels live here.
 * Every status gets an icon + label so colour is never the only signal.
 */

import type { BadgeVariant } from "@/components/ui/Badge";
import { AlertTriangle, CheckCircle2, Clock, RotateCw, XCircle } from "lucide-react";
import type { LucideIcon } from "lucide-react";

export const BATCH_STATUS_CODES = ["pending", "approved", "rejected", "applied", "stale"] as const;

export type BatchStatus = (typeof BATCH_STATUS_CODES)[number];

export interface BatchStatusView {
  label: string;
  variant: BadgeVariant;
  icon: LucideIcon;
}

export const BATCH_STATUS: Record<BatchStatus, BatchStatusView> = {
  pending: { label: "Menunggu persetujuan", variant: "warning", icon: Clock },
  approved: { label: "Disetujui belum diterapkan", variant: "info", icon: RotateCw },
  rejected: { label: "Ditolak", variant: "neutral", icon: XCircle },
  applied: { label: "Diterapkan", variant: "success", icon: CheckCircle2 },
  stale: { label: "Gagal diterapkan", variant: "danger", icon: AlertTriangle },
};

/** "Coba terapkan lagi" is offered only while the batch is approved but not applied. */
export function canRetry(status: BatchStatus): boolean {
  return status === "approved";
}
