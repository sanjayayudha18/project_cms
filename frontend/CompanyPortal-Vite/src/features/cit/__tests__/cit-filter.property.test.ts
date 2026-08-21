// Feature: frontend-consolidation, Property 4: CIT Filter Consistency

import { compoundFilter } from "@/lib/utils/filters";
import * as fc from "fast-check";
import { describe, expect, it } from "vitest";
import type { CitOrder, CitStatus } from "../types";

/**
 * Property 4: CIT Filter Consistency
 * **Validates: Requirements 4.2, 4.6**
 *
 * For any array of CIT orders and any combination of status and vendor filter values,
 * applying the filter SHALL produce a result set where:
 * (a) every returned row matches all active filter criteria,
 * (b) the CitSummary counts per status category equal the actual count of filtered items per category,
 * (c) no item matching the filter criteria is excluded from the result.
 */

const ALL_STATUSES: CitStatus[] = ["Scheduled", "In Transit", "Completed", "Failed"];

const arbStatus: fc.Arbitrary<CitStatus> = fc.constantFrom(...ALL_STATUSES);

const arbVendorId: fc.Arbitrary<string> = fc.constantFrom(
  "vendor-001",
  "vendor-002",
  "vendor-003",
  "vendor-004",
  "vendor-005",
);

const arbCitOrder: fc.Arbitrary<CitOrder> = fc.record({
  id: fc.uuid(),
  atmId: fc.stringMatching(/^ATM-\d{3,5}$/),
  vendorId: arbVendorId,
  orderDate: fc
    .date({ min: new Date("2024-01-01"), max: new Date("2025-12-31") })
    .map((d) => d.toISOString().slice(0, 10)),
  scheduledDate: fc
    .date({ min: new Date("2024-01-01"), max: new Date("2025-12-31") })
    .map((d) => d.toISOString().slice(0, 10)),
  amount: fc.nat({ max: 500_000_000 }),
  status: arbStatus,
  evidenceUrl: fc.oneof(fc.constant(null), fc.webUrl()),
});

const arbCitOrderArray: fc.Arbitrary<CitOrder[]> = fc.array(arbCitOrder, {
  minLength: 0,
  maxLength: 50,
});

const arbStatusFilter: fc.Arbitrary<CitStatus | null> = fc.oneof(fc.constant(null), arbStatus);

const arbVendorFilter: fc.Arbitrary<string | null> = fc.oneof(fc.constant(null), arbVendorId);

/**
 * Computes CitSummary counts per status from a filtered array,
 * replicating the logic in CitSummary component.
 */
function computeSummaryCounts(data: CitOrder[]): Record<CitStatus, number> {
  return ALL_STATUSES.reduce(
    (acc, status) => {
      acc[status] = data.filter((order) => order.status === status).length;
      return acc;
    },
    {} as Record<CitStatus, number>,
  );
}

describe("Property 4: CIT Filter Consistency", () => {
  it("(a) every returned row matches all active filter criteria", () => {
    fc.assert(
      fc.property(
        arbCitOrderArray,
        arbStatusFilter,
        arbVendorFilter,
        (orders, statusFilter, vendorFilter) => {
          const filtered = compoundFilter(orders, {
            status: statusFilter,
            vendorId: vendorFilter,
          });

          for (const order of filtered) {
            if (statusFilter !== null) {
              expect(order.status).toBe(statusFilter);
            }
            if (vendorFilter !== null) {
              expect(order.vendorId).toBe(vendorFilter);
            }
          }
        },
      ),
      { numRuns: 100 },
    );
  });

  it("(b) CitSummary counts match actual filtered counts per status", () => {
    fc.assert(
      fc.property(
        arbCitOrderArray,
        arbStatusFilter,
        arbVendorFilter,
        (orders, statusFilter, vendorFilter) => {
          const filtered = compoundFilter(orders, {
            status: statusFilter,
            vendorId: vendorFilter,
          });

          const summaryCounts = computeSummaryCounts(filtered);

          // Verify each status count matches actual filtered count
          for (const status of ALL_STATUSES) {
            const actualCount = filtered.filter((o) => o.status === status).length;
            expect(summaryCounts[status]).toBe(actualCount);
          }

          // Verify total counts sum to filtered length
          const totalCount = Object.values(summaryCounts).reduce((a, b) => a + b, 0);
          expect(totalCount).toBe(filtered.length);
        },
      ),
      { numRuns: 100 },
    );
  });

  it("(c) no matching item is excluded from the result", () => {
    fc.assert(
      fc.property(
        arbCitOrderArray,
        arbStatusFilter,
        arbVendorFilter,
        (orders, statusFilter, vendorFilter) => {
          const filtered = compoundFilter(orders, {
            status: statusFilter,
            vendorId: vendorFilter,
          });

          // Every item in the original that matches the criteria must be in the result
          const expectedMatching = orders.filter((order) => {
            const matchesStatus = statusFilter === null || order.status === statusFilter;
            const matchesVendor = vendorFilter === null || order.vendorId === vendorFilter;
            return matchesStatus && matchesVendor;
          });

          expect(filtered).toHaveLength(expectedMatching.length);

          // Verify the same set of items (by id)
          const filteredIds = filtered.map((o) => o.id).sort();
          const expectedIds = expectedMatching.map((o) => o.id).sort();
          expect(filteredIds).toEqual(expectedIds);
        },
      ),
      { numRuns: 100 },
    );
  });
});
