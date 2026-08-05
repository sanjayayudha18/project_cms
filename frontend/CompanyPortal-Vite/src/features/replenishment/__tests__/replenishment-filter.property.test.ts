// Feature: frontend-consolidation, Property 8: Replenishment filterSchedules Behavioral Equivalence
import { describe, expect, it } from 'vitest';
import * as fc from 'fast-check';
import { filterSchedules } from '../replenishment.utils';
import type { ReplenishmentSchedule } from '../types';

/**
 * Property 8: Replenishment filterSchedules Behavioral Equivalence
 *
 * For any array of replenishment schedules and any filter criteria,
 * applying Target_App's filterSchedules function SHALL produce identical
 * results to the expected semantics:
 *   - region "All" or "All regions" → no region constraint
 *   - vendor "All" or "All vendors" → no vendor constraint
 *   - specific region → only records matching that region
 *   - specific vendor → only records matching that vendor
 *   - combined → AND logic (both must match)
 *
 * **Validates: Requirements 9.4**
 */

const statusArb = fc.constantFrom<ReplenishmentSchedule['status']>(
  'completed',
  'in-transit',
  'scheduled',
  'delayed',
  'pending-vendor',
);

const regionPool = ['Jakarta', 'Bandung', 'Surabaya', 'Medan', 'Bali'];
const vendorPool = ['CIT-A', 'CIT-B', 'CIT-C', 'Vendor-X', 'Vendor-Y'];

const regionArb = fc.constantFrom(...regionPool);
const vendorArb = fc.constantFrom(...vendorPool);

const replenishmentScheduleArb: fc.Arbitrary<ReplenishmentSchedule> = fc.record({
  id: fc.uuid(),
  routeCode: fc.string({ minLength: 2, maxLength: 8 }),
  region: regionArb,
  vendor: vendorArb,
  windowStart: fc.date().map((d) => d.toISOString()),
  windowEnd: fc.date().map((d) => d.toISOString()),
  machineCount: fc.integer({ min: 1, max: 100 }),
  completionCount: fc.integer({ min: 0, max: 100 }),
  status: statusArb,
  cashValue: fc.integer({ min: 0, max: 500_000_000 }),
});

const schedulesArb = fc.array(replenishmentScheduleArb, { minLength: 0, maxLength: 50 });

// Filter criteria arbitraries — include the "All" variants and specific values
const regionFilterArb = fc.oneof(
  fc.constant('All'),
  fc.constant('All regions'),
  fc.constantFrom(...regionPool),
);

const vendorFilterArb = fc.oneof(
  fc.constant('All'),
  fc.constant('All vendors'),
  fc.constantFrom(...vendorPool),
);

describe('Feature: frontend-consolidation, Property 8: Replenishment filterSchedules Behavioral Equivalence', () => {
  it('region "All" returns all records regardless of region', () => {
    fc.assert(
      fc.property(schedulesArb, (records) => {
        const result = filterSchedules(records, 'All', 'All');
        expect(result).toEqual(records);
      }),
      { numRuns: 100 },
    );
  });

  it('region "All regions" returns all records regardless of region', () => {
    fc.assert(
      fc.property(schedulesArb, (records) => {
        const result = filterSchedules(records, 'All regions', 'All');
        expect(result).toEqual(records);
      }),
      { numRuns: 100 },
    );
  });

  it('vendor "All vendors" returns all records regardless of vendor', () => {
    fc.assert(
      fc.property(schedulesArb, (records) => {
        const result = filterSchedules(records, 'All', 'All vendors');
        expect(result).toEqual(records);
      }),
      { numRuns: 100 },
    );
  });

  it('specific region returns only records matching that region', () => {
    fc.assert(
      fc.property(schedulesArb, regionArb, (records, region) => {
        const result = filterSchedules(records, region, 'All');
        const expected = records.filter((r) => r.region === region);
        expect(result).toEqual(expected);
      }),
      { numRuns: 100 },
    );
  });

  it('specific vendor returns only records matching that vendor', () => {
    fc.assert(
      fc.property(schedulesArb, vendorArb, (records, vendor) => {
        const result = filterSchedules(records, 'All', vendor);
        const expected = records.filter((r) => r.vendor === vendor);
        expect(result).toEqual(expected);
      }),
      { numRuns: 100 },
    );
  });

  it('combined region + vendor filters apply as AND — both must match', () => {
    fc.assert(
      fc.property(
        schedulesArb,
        regionFilterArb,
        vendorFilterArb,
        (records, region, vendor) => {
          const result = filterSchedules(records, region, vendor);

          const regionActive = region !== 'All' && region !== 'All regions';
          const vendorActive = vendor !== 'All' && vendor !== 'All vendors';

          // Reference implementation — identical logic to Source_App
          const expected = records.filter((record) => {
            if (regionActive && record.region !== region) return false;
            if (vendorActive && record.vendor !== vendor) return false;
            return true;
          });

          expect(result).toEqual(expected);
        },
      ),
      { numRuns: 100 },
    );
  });

  it('filter result is always a subset of input — no records are invented', () => {
    fc.assert(
      fc.property(
        schedulesArb,
        regionFilterArb,
        vendorFilterArb,
        (records, region, vendor) => {
          const result = filterSchedules(records, region, vendor);
          expect(result.length).toBeLessThanOrEqual(records.length);
          for (const r of result) {
            expect(records).toContainEqual(r);
          }
        },
      ),
      { numRuns: 100 },
    );
  });

  it('filter preserves original order of matching records', () => {
    fc.assert(
      fc.property(
        schedulesArb,
        regionFilterArb,
        vendorFilterArb,
        (records, region, vendor) => {
          const result = filterSchedules(records, region, vendor);
          const indices = result.map((r) => records.indexOf(r));
          for (let i = 1; i < indices.length; i++) {
            expect(indices[i]).toBeGreaterThan(indices[i - 1]);
          }
        },
      ),
      { numRuns: 100 },
    );
  });
});
