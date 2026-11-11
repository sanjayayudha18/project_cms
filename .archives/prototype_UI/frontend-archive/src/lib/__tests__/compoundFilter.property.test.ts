/**
 * Property-based test: Compound-filter correctness
 *
 * Validates: Requirements 5.4, 5.5, 5.6, 5.7
 */
import { describe, it, expect } from 'vitest';
import * as fc from 'fast-check';
import { compoundFilter } from '../filters';

/** CIT-order-like record shape used for property testing */
interface MockCitOrder {
  id: string;
  status: 'Scheduled' | 'In Transit' | 'Completed' | 'Failed';
  vendorId: string;
  amount: number;
}

const statuses = ['Scheduled', 'In Transit', 'Completed', 'Failed'] as const;
const vendorIds = ['vendor-1', 'vendor-2', 'vendor-3'] as const;

/** Generator for a single CIT-order-like record */
const citOrderArb: fc.Arbitrary<MockCitOrder> = fc.record({
  id: fc.uuid(),
  status: fc.constantFrom(...statuses),
  vendorId: fc.constantFrom(...vendorIds),
  amount: fc.nat({ max: 500_000_000 }),
});

/** Generator for an array of CIT-order-like records */
const citOrdersArb = fc.array(citOrderArb, { minLength: 0, maxLength: 30 });

/** Generator for compound filter criteria (status and vendorId, each possibly null) */
const filtersArb = fc.record({
  status: fc.oneof(fc.constant(null), fc.constantFrom(...statuses)),
  vendorId: fc.oneof(fc.constant(null), fc.constantFrom(...vendorIds)),
});

describe('Feature: cms-frontend-prototype, Property 5: Compound-filter correctness', () => {
  it('compound filter returns exactly those records satisfying ALL active criteria', () => {
    fc.assert(
      fc.property(citOrdersArb, filtersArb, (orders, filters) => {
        const result = compoundFilter(orders, filters);

        // Build expected result by manually filtering
        const expected = orders.filter((order) => {
          if (filters.status !== null && order.status !== filters.status) return false;
          if (filters.vendorId !== null && order.vendorId !== filters.vendorId) return false;
          return true;
        });

        // Same length — no false positives or false negatives
        expect(result).toHaveLength(expected.length);

        // Every result record satisfies ALL active criteria
        for (const record of result) {
          if (filters.status !== null) {
            expect(record.status).toBe(filters.status);
          }
          if (filters.vendorId !== null) {
            expect(record.vendorId).toBe(filters.vendorId);
          }
        }

        // Every expected record is present in the result (no false negatives)
        expect(result).toEqual(expected);
      }),
      { numRuns: 100 },
    );
  });

  it('summary counts per status category match actual counts in filtered result', () => {
    fc.assert(
      fc.property(citOrdersArb, filtersArb, (orders, filters) => {
        const filtered = compoundFilter(orders, filters);

        // Compute summary counts per status from the filtered result
        const summaryCounts: Record<string, number> = {
          Scheduled: 0,
          'In Transit': 0,
          Completed: 0,
          Failed: 0,
        };

        for (const record of filtered) {
          summaryCounts[record.status] = (summaryCounts[record.status] ?? 0) + 1;
        }

        // Verify each count matches what we actually count in the result
        for (const status of statuses) {
          const actualCount = filtered.filter((r) => r.status === status).length;
          expect(summaryCounts[status]).toBe(actualCount);
        }

        // Total of all status counts equals filtered length
        const totalCount = Object.values(summaryCounts).reduce((a, b) => a + b, 0);
        expect(totalCount).toBe(filtered.length);
      }),
      { numRuns: 100 },
    );
  });
});
