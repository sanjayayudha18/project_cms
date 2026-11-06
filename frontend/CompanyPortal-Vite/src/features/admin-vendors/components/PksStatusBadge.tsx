import { Badge, type BadgeProps } from "@/components/ui/Badge";
import { AlarmClock, Ban, CheckCircle2, CircleDashed, XCircle } from "lucide-react";
import type { PksStatus } from "../types";

const PKS_BADGES: Record<PksStatus, Pick<BadgeProps, "variant" | "icon" | "label">> = {
  aktif: { variant: "success", icon: CheckCircle2, label: "Aktif" },
  need_renewal: { variant: "warning", icon: AlarmClock, label: "Need Renewal" },
  expired: { variant: "danger", icon: XCircle, label: "Expired" },
  terminated: { variant: "neutral", icon: Ban, label: "Terminated" },
  belum_diisi: { variant: "neutral", icon: CircleDashed, label: "Belum diisi" },
};

interface PksStatusBadgeProps {
  /** null/undefined = vendor internal (tidak punya status PKS). */
  status: PksStatus | null | undefined;
}

export function PksStatusBadge({ status }: PksStatusBadgeProps) {
  if (!status) return <span>-</span>;
  return <Badge {...PKS_BADGES[status]} />;
}
