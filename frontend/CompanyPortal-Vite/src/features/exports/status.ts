import {
  Check,
  CircleAlert,
  CircleCheck,
  Clock,
  Download,
  Loader,
  type LucideIcon,
  TimerOff,
} from "lucide-react";
import type { ExportKind, ExportStatus, ReportKey } from "./types";

export const KIND_LABEL: Record<ExportKind, string> = {
  vendor: "Vendor",
  cabang: "Cabang",
  harga_paket: "Harga paket",
  master_data_global: "Data master",
  dsr_late_daily: "Laporan DSR telat (harian)",
  dsr_late_summary: "Laporan DSR telat (rekap bulanan)",
  replenishment_amount_mismatch: "Replenish: selisih nominal",
  replenishment_trips: "Replenish: trip per ATM",
};

/** Report kind -> `/exports/reports/{report}` segment. */
export const REPORT_KEY_BY_KIND: Partial<Record<ExportKind, ReportKey>> = {
  dsr_late_daily: "dsr-late-daily",
  dsr_late_summary: "dsr-late-summary",
  replenishment_amount_mismatch: "replenishment-amount-mismatch",
  replenishment_trips: "replenishment-trips",
};

export const isReportKind = (k: ExportKind): boolean => k in REPORT_KEY_BY_KIND;

export type StatusTone = "info" | "success" | "neutral" | "danger" | "warning";

export const STATUS_META: Record<
  ExportStatus,
  { label: string; icon: LucideIcon; tone: StatusTone }
> = {
  queued: { label: "Antri", icon: Clock, tone: "info" },
  running: { label: "Diproses", icon: Loader, tone: "info" },
  succeeded: { label: "Siap diunduh", icon: CircleCheck, tone: "success" },
  downloading: { label: "Sedang diunduh", icon: Download, tone: "info" },
  downloaded: { label: "Sudah diunduh", icon: Check, tone: "neutral" },
  failed: { label: "Gagal", icon: CircleAlert, tone: "danger" },
  expired: { label: "Kedaluwarsa", icon: TimerOff, tone: "warning" },
};

export const isActiveStatus = (s: ExportStatus): boolean =>
  s === "queued" || s === "running" || s === "downloading";
