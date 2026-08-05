/**
 * Property-based test: Status-priority sort ordering
 *
 * Validates: Requirements 4.7
 */
import { describe, it, expect } from 'vitest';
import * as fc from 'fast-check';
import {
  sortByStatusPriority,
  type ReplenishmentSchedule,
} from '../replenishment.utils';

const statuses: ReplenishmentSchedule['status'][] = [
  'completed',
  'in-transit',
  'scheduled',
  'delayed',
  'pending-vendor',
];

const arbSchedule: fc.Arbitrary<ReplenishmentSchedule> = fc.record({
  id: fc.string({ minLength: 1, maxLength: 20 }),
  routeCode: fc.string({ minLength: 1, maxLength: 10 }),
  region: fc.string({ minLength: 1, maxLength: 20 }),
  vendor: fc.string({ minLength: 1, maxLength: 20 }),
  windowStart: fc.constant('08:00'),
  windowEnd: fc.constant('12:00'),
  machineCount: fc.integer({ min: 1, max: 100 }),
  completionCount: fc.integer({ min: 0, max: 100 }),
  status: fc.constantFrom(...statuses),
  cashValue: fc.integer({ min: 0, max: 1_000_000_000_000 }),
});

const arbScheduleArray = fc.array(arbSchedule, { minLength: 0, maxLength: 30 });

describe('Feature: cms-dashboard-redesign, Property 5: Status-priority sort ordering', () => {
  it('all "delayed" records appear before all "in-transit" records', () => {
    fc.assert(
      fc.property(arbScheduleArray, (records) => {
        const sorted = sortByStatusPriority(records);
        const lastDelayedIdx = sorted.findLastIndex(
          (r) => r.status === 'delayed',
        );
        const firstInTransitIdx = sorted.findIndex(
          (r) => r.status === 'in-transit',
        );

        if (lastDelayedIdx !== -1 && firstInTransitIdx !== -1) {
          expect(lastDelayedIdx).toBeLessThan(firstInTransitIdx);
        }
      }),
      { numRuns: 100 },
    );
  });

  it('all "in-transit" records appear before all "completed" records', () => {
    fc.assert(
      fc.property(arbScheduleArray, (records) => {
        const sorted = sortByStatusPriority(records);
        const lastInTransitIdx = sorted.findLastIndex(
          (r) => r.status === 'in-transit',
        );
        const firstCompletedIdx = sorted.findIndex(
          (r) => r.status === 'completed',
        );

        if (lastInTransitIdx !== -1 && firstCompletedIdx !== -1) {
          expect(lastInTransitIdx).toBeLessThan(firstCompletedIdx);
        }
      }),
      { numRuns: 100 },
    );
  });

  it('all "pending-vendor" records appear before "scheduled" and "completed"', () => {
    fc.assert(
      fc.property(arbScheduleArray, (records) => {
        const sorted = sortByStatusPriority(records);
        const lastPendingVendorIdx = sorted.findLastIndex(
          (r) => r.status === 'pending-vendor',
        );
        const firstScheduledIdx = sorted.findIndex(
          (r) => r.status === 'scheduled',
        );
        const firstCompletedIdx = sorted.findIndex(
          (r) => r.status === 'completed',
        );

        if (lastPendingVendorIdx !== -1 && firstScheduledIdx !== -1) {
          expect(lastPendingVendorIdx).toBeLessThan(firstScheduledIdx);
        }
        if (lastPendingVendorIdx !== -1 && firstCompletedIdx !== -1) {
          expect(lastPendingVendorIdx).toBeLessThan(firstCompletedIdx);
        }
      }),
      { numRuns: 100 },
    );
  });

  it('the sorted array has the same length as the input', () => {
    fc.assert(
      fc.property(arbScheduleArray, (records) => {
        const sorted = sortByStatusPriority(records);
        expect(sorted).toHaveLength(records.length);
      }),
      { numRuns: 100 },
    );
  });

  it('every record in the input exists in the output (no data lost)', () => {
    fc.assert(
      fc.property(arbScheduleArray, (records) => {
        const sorted = sortByStatusPriority(records);

        for (const record of records) {
          expect(sorted).toContain(record);
        }
      }),
      { numRuns: 100 },
    );
  });
});
