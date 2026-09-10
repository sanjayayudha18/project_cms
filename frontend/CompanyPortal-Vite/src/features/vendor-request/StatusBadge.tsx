/**
 * Vendor Request status badge (Req 13.2, 14.1): draft=neutral,
 * pending_approval=warning, approved=success, rejected=danger,
 * processing=info, completed=success, cancelled=neutral+strikethrough.
 * Never brand red for status. `failed` isn't in the requirements' 7-status
 * badge mapping table (only in the DB CHECK's superset, see migration
 * 028's comment) — mapped to danger as the closest semantic fit.
 */

import { Badge, type BadgeVariant } from "@/components/ui/Badge";
import type { VendorRequestStatus } from "./types";

const STATUS_CONFIG: Record<VendorRequestStatus, { variant: BadgeVariant; label: string }> = {
  draft: { variant: "neutral", label: "Draft" },
  pending_approval: { variant: "warning", label: "Menunggu Approval" },
  approved: { variant: "success", label: "Disetujui" },
  rejected: { variant: "danger", label: "Ditolak" },
  processing: { variant: "info", label: "Diproses" },
  completed: { variant: "success", label: "Selesai" },
  failed: { variant: "danger", label: "Gagal" },
  cancelled: { variant: "neutral", label: "Dibatalkan" },
};

interface StatusBadgeProps {
  status: VendorRequestStatus;
}

export function StatusBadge({ status }: StatusBadgeProps) {
  const config = STATUS_CONFIG[status];
  return (
    <span className={status === "cancelled" ? "line-through" : undefined}>
      <Badge variant={config.variant} label={config.label} />
    </span>
  );
}
