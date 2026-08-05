import type { ReplenishmentSchedule } from "./types";

const STATUS_PRIORITY: Record<ReplenishmentSchedule["status"], number> = {
  delayed: 0,
  "in-transit": 1,
  "pending-vendor": 2,
  scheduled: 3,
  completed: 4,
};

/**
 * Sort replenishment schedules by status priority.
 * Order: delayed → in-transit → pending-vendor → scheduled → completed.
 * Stable sort preserves original order within same priority.
 */
export function sortByStatusPriority(
  records: ReplenishmentSchedule[],
): ReplenishmentSchedule[] {
  return [...records].sort(
    (a, b) => STATUS_PRIORITY[a.status] - STATUS_PRIORITY[b.status],
  );
}
