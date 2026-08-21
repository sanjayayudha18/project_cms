/**
 * Renders a ReplenishmentStatus as an icon + label badge (Req 4.5).
 * Thin wrapper over the shared Badge component, which always renders the
 * label text alongside the icon — satisfies Req 11.2 (status conveyed via
 * icon and text, never color alone).
 */

import { Badge } from "@/components/ui/Badge";
import { STATUS_BADGE_CONFIG } from "../constants";
import type { ReplenishmentStatus } from "../types";

interface StatusBadgeProps {
  status: ReplenishmentStatus;
}

export function StatusBadge({ status }: StatusBadgeProps) {
  const config = STATUS_BADGE_CONFIG[status];
  return <Badge variant={config.variant} icon={config.icon} label={config.label} />;
}
