/**
 * ATM Portal feature constants: query cache key/stale time, and the
 * status -> badge (icon/label/variant) config map used by the Status
 * column (requirements.md Req 76.5).
 *
 * Badge labels are the exact English strings quoted in Req 76.5 ("Critical",
 * "Low", "Normal", "Unconfigured", "No Data") — not Indonesian. That
 * requirement quotes them verbatim, which tasks.md's Notes calls out as
 * taking precedence over the feature's general Indonesian-labels
 * convention ("matching requirements.md acceptance criteria verbatim where
 * quoted").
 */

import type { BadgeVariant } from "@/components/ui/Badge";
import {
  AlertTriangle,
  CheckCircle,
  HelpCircle,
  type LucideIcon,
  Settings,
  TrendingDown,
} from "lucide-react";
import type { ReplenishmentStatus } from "./types";

export const ATM_PORTAL_QUERY_KEY = ["atm-portal", "list"] as const;
export const ATM_CASHPOS_QUERY_KEY = ["atm-portal", "cashpos"] as const;

export const ATM_PORTAL_STALE_TIME = 2 * 60 * 1000; // 2 minutes

export const ATM_PORTAL_MODE_REPLENISH = "replenish" as const;
export const ATM_PORTAL_MODE_CASHPOS = "cashpos" as const;

export const CASHPOS_DEFAULT_SORT_BY = "cashpos_date" as const;
export const CASHPOS_DEFAULT_SORT_ORDER = "desc" as const;
export const REPLENISH_DEFAULT_SORT_BY = "terminal_id" as const;
export const REPLENISH_DEFAULT_SORT_ORDER = "asc" as const;

interface StatusBadgeConfig {
  readonly label: string;
  readonly icon: LucideIcon;
  readonly variant: BadgeVariant;
}

export const STATUS_BADGE_CONFIG: Record<ReplenishmentStatus, StatusBadgeConfig> = {
  critical: { label: "Critical", icon: AlertTriangle, variant: "danger" },
  low: { label: "Low", icon: TrendingDown, variant: "warning" },
  normal: { label: "Normal", icon: CheckCircle, variant: "success" },
  unconfigured: { label: "Unconfigured", icon: Settings, variant: "neutral" },
  no_data: { label: "No Data", icon: HelpCircle, variant: "neutral" },
};
