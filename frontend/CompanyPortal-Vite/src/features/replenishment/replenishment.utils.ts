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
export function sortByStatusPriority(records: ReplenishmentSchedule[]): ReplenishmentSchedule[] {
  return [...records].sort((a, b) => STATUS_PRIORITY[a.status] - STATUS_PRIORITY[b.status]);
}

/**
 * Filter replenishment schedules by region and vendor (combined AND filter).
 * "All" or "All regions" means no region constraint.
 * "All" or "All vendors" means no vendor constraint.
 */
export function filterSchedules(
  records: ReplenishmentSchedule[],
  region: string,
  vendor: string,
): ReplenishmentSchedule[] {
  const regionActive = region !== "All" && region !== "All regions";
  const vendorActive = vendor !== "All" && vendor !== "All vendors";

  if (!regionActive && !vendorActive) {
    return records;
  }

  return records.filter((record) => {
    if (regionActive && record.region !== region) return false;
    if (vendorActive && record.vendor !== vendor) return false;
    return true;
  });
}
