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
import type { ExportStatus } from "./types";

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
