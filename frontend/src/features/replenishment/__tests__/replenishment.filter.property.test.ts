/**
 * Property-based test: Replenishment combined filter correctness
 *
 * Feature: cms-dashboard-redesign, Property 6: Replenishment combined filter correctness
 *
 * For any array of replenishment schedules and any combination of region and vendor
 * filter values, every record in the filtered result SHALL match the selected region
 * (if not "All") AND match the selected vendor (if not "All"), and the result count
 * SHALL equal the length of the filtered array.
 *
 * Validates: Requirements 6.5, 6.6, 6.7
 */
import { describe, it, expect } from 'vitest';
import * as fc from 'fast-check';
import {
  filterSchedules,
  type ReplenishmentSchedule,
} from '../replenishment.utils';

const regions = ['South Jakarta', 'North Jakarta', 'West Jakarta', 'East Jakarta', 'Central Jakarta'] as const;
const vendors = ['TAG', 'SSI', 'G4S', 'ISS', 'Brinks'] as const;
const statuses = ['completed', 'in-transit', 'scheduled', 'delayed', 'pending-vendor'] as const;

/** Generator for a single replenishment schedule record */
const scheduleArb: fc.Arbitrary<ReplenishmentSchedule> = fc.record({
  id: fc.uuid(),
  routeCode: fc.string({ minLength: 3, maxLength: 12 }),
  region: fc.constantFrom(...regions),
  vendor: fc.constantFrom(...vendors),
  windowStart: fc.constant('08:00'),
  windowEnd: fc.constant('12:00'),
  machineCount: fc.integer({ min: 1, max: 50 }),
  completionCount: fc.integer({ min: 0, max: 50 }),
  status: fc.constantFrom(...statuses),
  cashValue: fc.nat({ max: 50_000_000_000 }),
});

/** Generator for an array of schedules */
const schedulesArb = fc.array(scheduleArb, { minLength: 0, maxLength: 30 });

/**
 * Generator that picks a region filter from existing schedule values or "All".
 * This ensures meaningful filter tests against actual data.
 */
function regionFilterArb(schedules: ReplenishmentSchedule[]): fc.Arbitrary<string> {
  const existingRegions = [...new Set(schedules.map((s) => s.region))];
  if (existingRegions.length === 0) {
    return fc.constantFrom('All', 'All regions');
  }
  return fc.oneof(
    fc.constant('All'),
    fc.constant('All regions'),
    fc.constantFrom(...existingRegions),
  );
}

/**
 * Generator that picks a vendor filter from existing schedule values or "All".
 */
function vendorFilterArb(schedules: ReplenishmentSchedule[]): fc.Arbitrary<string> {
  const existingVendors = [...new Set(schedules.map((s) => s.vendor))];
  if (existingVendors.length === 0) {
    return fc.constantFrom('All', 'All vendors');
  }
  return fc.oneof(
    fc.constant('All'),
    fc.constant('All vendors'),
    fc.constantFrom(...existingVendors),
  );
}

describe('Feature: cms-dashboard-redesign, Property 6: Replenishment combined filter correctness', () => {
  it('every record in filtered result matches the selected region (when not "All")', () => {
    fc.assert(
      fc.property(schedulesArb, (schedules) => {
        return fc.assert(
          fc.property(regionFilterArb(schedules), (regionFilter) => {
            const result = filterSchedules(schedules, regionFilter, 'All');

            const regionActive = regionFilter !== 'All' && regionFilter !== 'All regions';
            if (regionActive) {
              for (const record of result) {
                expect(record.region).toBe(regionFilter);
              }
            }
          }),
          { numRuns: 10 },
        );
      }),
      { numRuns: 100 },
    );
  });

  it('every record in filtered result matches the selected vendor (when not "All")', () => {
    fc.assert(
      fc.property(schedulesArb, (schedules) => {
        return fc.assert(
          fc.property(vendorFilterArb(schedules), (vendorFilter) => {
            const result = filterSchedules(schedules, 'All', vendorFilter);

            const vendorActive = vendorFilter !== 'All' && vendorFilter !== 'All vendors';
            if (vendorActive) {
              for (const record of result) {
                expect(record.vendor).toBe(vendorFilter);
              }
            }
          }),
          { numRuns: 10 },
        );
      }),
      { numRuns: 100 },
    );
  });

  it('when both region and vendor are active, every record matches both', () => {
    fc.assert(
      fc.property(schedulesArb, (schedules) => {
        return fc.assert(
          fc.property(
            regionFilterArb(schedules),
            vendorFilterArb(schedules),
            (regionFilter, vendorFilter) => {
              const result = filterSchedules(schedules, regionFilter, vendorFilter);

              const regionActive = regionFilter !== 'All' && regionFilter !== 'All regions';
              const vendorActive = vendorFilter !== 'All' && vendorFilter !== 'All vendors';

              for (const record of result) {
                if (regionActive) {
                  expect(record.region).toBe(regionFilter);
                }
                if (vendorActive) {
                  expect(record.vendor).toBe(vendorFilter);
                }
              }
            },
          ),
          { numRuns: 10 },
        );
      }),
      { numRuns: 100 },
    );
  });

  it('result count equals filtered array length', () => {
    fc.assert(
      fc.property(schedulesArb, (schedules) => {
        return fc.assert(
          fc.property(
            regionFilterArb(schedules),
            vendorFilterArb(schedules),
            (regionFilter, vendorFilter) => {
              const result = filterSchedules(schedules, regionFilter, vendorFilter);

              // Manually compute expected count
              const regionActive = regionFilter !== 'All' && regionFilter !== 'All regions';
              const vendorActive = vendorFilter !== 'All' && vendorFilter !== 'All vendors';

              const expected = schedules.filter((record) => {
                if (regionActive && record.region !== regionFilter) return false;
                if (vendorActive && record.vendor !== vendorFilter) return false;
                return true;
              });

              expect(result).toHaveLength(expected.length);
            },
          ),
          { numRuns: 10 },
        );
      }),
      { numRuns: 100 },
    );
  });

  it('filtered result is a subset of the input (no new records introduced)', () => {
    fc.assert(
      fc.property(schedulesArb, (schedules) => {
        return fc.assert(
          fc.property(
            regionFilterArb(schedules),
            vendorFilterArb(schedules),
            (regionFilter, vendorFilter) => {
              const result = filterSchedules(schedules, regionFilter, vendorFilter);

              // Every record in the result must exist in the original input
              for (const record of result) {
                expect(schedules).toContain(record);
              }
            },
          ),
          { numRuns: 10 },
        );
      }),
      { numRuns: 100 },
    );
  });
});
