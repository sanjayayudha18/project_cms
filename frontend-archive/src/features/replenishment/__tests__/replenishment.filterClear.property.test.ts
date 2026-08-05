/**
 * Property-based test: Filter clear restores full dataset
 *
 * Validates: Requirements 6.8
 */
import { describe, it, expect } from 'vitest';
import * as fc from 'fast-check';
import { filterSchedules, type ReplenishmentSchedule } from '../replenishment.utils';

const STATUSES: ReplenishmentSchedule['status'][] = [
  'completed',
  'in-transit',
  'scheduled',
  'delayed',
  'pending-vendor',
];

const REGIONS = ['South Jakarta', 'North Jakarta', 'West Jakarta', 'East Jakarta', 'Central Jakarta'];
const VENDORS = ['TAG', 'G4S', 'Brinks', 'Securitas', 'Loomis'];

const arbSchedule: fc.Arbitrary<ReplenishmentSchedule> = fc.record({
  id: fc.string({ minLength: 1, maxLength: 20 }),
  routeCode: fc.string({ minLength: 1, maxLength: 15 }),
  region: fc.constantFrom(...REGIONS),
  vendor: fc.constantFrom(...VENDORS),
  windowStart: fc.constant('08:00'),
  windowEnd: fc.constant('12:00'),
  machineCount: fc.integer({ min: 1, max: 50 }),
  completionCount: fc.integer({ min: 0, max: 50 }),
  status: fc.constantFrom(...STATUSES),
  cashValue: fc.integer({ min: 0, max: 100_000_000_000 }),
});

const arbSchedules = fc.array(arbSchedule, { minLength: 0, maxLength: 30 });

describe('Feature: cms-dashboard-redesign, Property 7: Filter clear restores full dataset', () => {
  it('filterSchedules(arr, "All", "All") returns the same array (same length and same records)', () => {
    fc.assert(
      fc.property(arbSchedules, (schedules) => {
        const result = filterSchedules(schedules, 'All', 'All');
        expect(result).toHaveLength(schedules.length);
        expect(result).toEqual(schedules);
      }),
      { numRuns: 100 }
    );
  });

  it('filterSchedules(arr, "All regions", "All vendors") returns the same array', () => {
    fc.assert(
      fc.property(arbSchedules, (schedules) => {
        const result = filterSchedules(schedules, 'All regions', 'All vendors');
        expect(result).toHaveLength(schedules.length);
        expect(result).toEqual(schedules);
      }),
      { numRuns: 100 }
    );
  });

  it('after applying a filter, clearing to "All" restores original dataset', () => {
    fc.assert(
      fc.property(
        arbSchedules,
        fc.constantFrom(...REGIONS),
        fc.constantFrom(...VENDORS),
        (schedules, region, vendor) => {
          // Apply a non-trivial filter
          filterSchedules(schedules, region, vendor);

          // Clear filters back to "All"
          const restored = filterSchedules(schedules, 'All', 'All');

          expect(restored).toHaveLength(schedules.length);
          expect(restored).toEqual(schedules);
        }
      ),
      { numRuns: 100 }
    );
  });
});
