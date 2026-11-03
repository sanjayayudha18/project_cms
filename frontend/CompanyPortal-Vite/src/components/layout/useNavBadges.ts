import { useInbox } from "@/features/approvals/hooks";
import { useEodSummary } from "@/features/eod-monitoring/hooks/useEodQueries";
import { useVendorRequests } from "@/features/vendor-request/hooks";
import type { VendorRequestStatus } from "@/features/vendor-request/types";
import type { DbRole } from "@/lib/auth/store";
import type { NavBadgeSource, VisibleNav } from "@/lib/config/navigation";
import { todayWib } from "@/lib/shell/time";
import { formatBadgeCount } from "@/lib/utils/formatters";

export type NavBadge = { kind: "count"; value: string; raw: number } | { kind: "dot" };

const POLL_MS = 60_000;
const EOD_POLL_MS = 300_000;

/** Status Request CIT yang dihitung per role (D8); null = tanpa badge. */
export function badgeRequestStatuses(role: DbRole): VendorRequestStatus[] | null {
  switch (role) {
    case "ATM-SPV":
    case "BRANCH-ATM-SPV":
      return ["pending_approval", "completion_pending", "vault_review"];
    case "ATM-USER":
    case "BRANCH-ATM-USER":
      return ["draft", "rejected"];
    default:
      return null;
  }
}

function countBadge(n: number | undefined): NavBadge | undefined {
  if (n === undefined) return undefined;
  const value = formatBadgeCount(n);
  return value ? { kind: "count", value, raw: n } : undefined;
}

export function useNavBadges(
  visible: VisibleNav,
  role: DbRole | null,
): Partial<Record<NavBadgeSource, NavBadge>> {
  const sources = new Set(visible.all.map((i) => i.badge));
  const statuses = role ? badgeRequestStatuses(role) : null;
  const isAdmin = role === "ADMIN" || role === "ADMIN_PARAM";

  const inbox = useInbox({ enabled: sources.has("approvals-inbox"), refetchInterval: POLL_MS });
  const cit = useVendorRequests(
    { status: statuses ?? [], pageSize: 1 },
    { enabled: sources.has("cit-requests") && statuses !== null, refetchInterval: POLL_MS },
  );
  const eod = useEodSummary(todayWib(new Date()), EOD_POLL_MS, {
    enabled: sources.has("eod-health") && isAdmin,
    refetchInterval: EOD_POLL_MS,
  });

  const counts = eod.isError ? undefined : eod.data?.counts;
  const eodAttention = counts ? counts.failed + counts.max_retries_exhausted + counts.late : 0;

  return {
    "approvals-inbox": inbox.isError ? undefined : countBadge(inbox.data?.length),
    "cit-requests": cit.isError ? undefined : countBadge(cit.data?.pagination.total_count),
    "eod-health": eodAttention > 0 ? { kind: "dot" } : undefined,
  };
}
