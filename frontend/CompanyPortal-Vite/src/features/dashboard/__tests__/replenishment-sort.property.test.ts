// Feature: frontend-consolidation, Property 3: Status Priority Sort Invariant
import * as fc from "fast-check";
import { describe, expect, it } from "vitest";
import { sortByStatusPriority } from "../replenishment.utils";
import type { ReplenishmentSchedule, ReplenishmentStatus } from "../types";

/**
 * Property 3: Status Priority Sort Invariant
 * Validates: Requirements 2.4, 9.4
 *
 * For any array of replenishment schedules with mixed statuses,
 * applying sortByStatusPriority SHALL produce an output array where
 * every element at index i has a status priority greater than or equal
 * to the element at index i+1, maintaining a stable total ordering.
 */

const ALL_STATUSES: ReplenishmentStatus[] = [
  "completed",
  "in-transit",
  "scheduled",
  "delayed",
  "pending-vendor",
];

const STATUS_PRIORITY: Record<ReplenishmentStatus, number> = {
  delayed: 0,
  "in-transit": 1,
  "pending-vendor": 2,
  scheduled: 3,
  completed: 4,
};

const arbStatus: fc.Arbitrary<ReplenishmentStatus> = fc.constantFrom(...ALL_STATUSES);

const arbReplenishmentSchedule: fc.Arbitrary<ReplenishmentSchedule> = fc.record({
  id: fc.uuid(),
  routeCode: fc.stringMatching(/^[A-Z]{2,4}-\d{2,4}$/),
  region: fc.constantFrom("Jakarta", "Bandung", "Surabaya", "Medan", "Semarang"),
  vendor: fc.constantFrom("Vendor-A", "Vendor-B", "Vendor-C"),
  windowStart: fc.date().map((d) => d.toISOString()),
  windowEnd: fc.date().map((d) => d.toISOString()),
  machineCount: fc.nat({ max: 100 }),
  completionCount: fc.nat({ max: 100 }),
  status: arbStatus,
  cashValue: fc.nat({ max: 10_000_000_000 }),
});

const arbScheduleArray: fc.Arbitrary<ReplenishmentSchedule[]> = fc.array(arbReplenishmentSchedule, {
  minLength: 0,
  maxLength: 50,
});

describe("Property 3: Status Priority Sort Invariant", () => {
  it("sorted output is totally ordered by status priority (each element priority ≤ next)", () => {
    fc.assert(
      fc.property(arbScheduleArray, (schedules) => {
        const sorted = sortByStatusPriority(schedules);

        // Output length must equal input length
        expect(sorted).toHaveLength(schedules.length);

        // Every consecutive pair must satisfy priority(i) <= priority(i+1)
        for (let i = 0; i < sorted.length - 1; i++) {
          const currentPriority = STATUS_PRIORITY[sorted[i].status];
          const nextPriority = STATUS_PRIORITY[sorted[i + 1].status];
          expect(currentPriority).toBeLessThanOrEqual(nextPriority);
        }
      }),
      { numRuns: 100 },
    );
  });

  it("sort does not mutate the original array", () => {
    fc.assert(
      fc.property(arbScheduleArray, (schedules) => {
        const original = [...schedules];
        sortByStatusPriority(schedules);

        // Original array must remain unchanged
        expect(schedules).toEqual(original);
      }),
      { numRuns: 100 },
    );
  });

  it("sort preserves all elements (no additions or removals)", () => {
    fc.assert(
      fc.property(arbScheduleArray, (schedules) => {
        const sorted = sortByStatusPriority(schedules);

        // Same set of IDs in input and output
        const inputIds = schedules.map((s) => s.id).sort();
        const outputIds = sorted.map((s) => s.id).sort();
        expect(outputIds).toEqual(inputIds);
      }),
      { numRuns: 100 },
    );
  });
});
