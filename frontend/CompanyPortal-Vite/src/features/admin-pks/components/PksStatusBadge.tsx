import { Badge, type BadgeProps } from "@/components/ui/Badge";
import { AlarmClock, Ban, CheckCircle2, Clock, History, Hourglass, XCircle } from "lucide-react";
import type { PksRecordStatus } from "../types";

// Status is always icon + text, never colour alone.
const PKS_BADGES: Record<PksRecordStatus, Pick<BadgeProps, "variant" | "icon" | "label">> = {
  aktif: { variant: "success", icon: CheckCircle2, label: "Aktif" },
  need_renewal: { variant: "warning", icon: AlarmClock, label: "Need Renewal" },
  belum_berlaku: { variant: "info", icon: Clock, label: "Belum Berlaku" },
  expired: { variant: "danger", icon: XCircle, label: "Expired" },
  terminated: { variant: "neutral", icon: Ban, label: "Terminated" },
  diganti: { variant: "neutral", icon: History, label: "Diganti" },
  menunggu_persetujuan: { variant: "info", icon: Hourglass, label: "Menunggu Persetujuan" },
};

export function PksStatusBadge({ status }: { status: PksRecordStatus }) {
  return <Badge {...PKS_BADGES[status]} />;
}
