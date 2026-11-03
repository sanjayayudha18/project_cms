import { Badge } from "@/components/ui/Badge";
import type { BadgeVariant } from "@/components/ui/Badge";
import type { LucideIcon } from "lucide-react";
import { CheckCircle2, Clock, Hourglass, XCircle } from "lucide-react";
import { STATUS_LABELS } from "./api";
import type { DsrLateStatus } from "./api";

const STATUS_STYLE: Record<DsrLateStatus, { variant: BadgeVariant; icon: LucideIcon }> = {
  ok: { variant: "success", icon: CheckCircle2 },
  late: { variant: "warning", icon: Clock },
  not_sent: { variant: "danger", icon: XCircle },
  pending: { variant: "neutral", icon: Hourglass },
};

/** Status = text label + icon; colour is only a complement (Sec 13). `deadline` is shown as a tooltip. */
export function StatusBadge({ status, deadline }: { status: DsrLateStatus; deadline?: string }) {
  const { variant, icon } = STATUS_STYLE[status];
  const title = deadline ? `Batas kirim 09:00 WIB (${deadline})` : "Batas kirim 09:00 WIB";
  return (
    <span title={title} aria-label={`${STATUS_LABELS[status]}. ${title}`}>
      <Badge variant={variant} icon={icon} label={STATUS_LABELS[status]} />
    </span>
  );
}
