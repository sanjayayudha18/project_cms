/**
 * Property-based tests: Progress bar calculation
 *
 * Feature: cms-dashboard-redesign, Property 4: Progress bar calculation
 * Validates: Requirements 4.3
 */
import { describe, it, expect } from 'vitest';
import * as fc from 'fast-check';
import { progressPercent } from '../formatters';

describe('Feature: cms-dashboard-redesign, Property 4: Progress bar calculation', () => {
  it('for any pair where 0 ≤ completed ≤ total and total > 0, result equals Math.round(completed / total * 100)', () => {
    fc.assert(
      fc.property(
        fc.integer({ min: 1, max: 10000 }).chain((total) =>
          fc.tuple(fc.integer({ min: 0, max: total }), fc.constant(total))
        ),
        ([completed, total]) => {
          expect(progressPercent(completed, total)).toBe(
            Math.round((completed / total) * 100)
          );
        }
      ),
      { numRuns: 100 }
    );
  });

  it('result is always between 0 and 100 inclusive', () => {
    fc.assert(
      fc.property(
        fc.integer({ min: 1, max: 10000 }).chain((total) =>
          fc.tuple(fc.integer({ min: 0, max: total }), fc.constant(total))
        ),
        ([completed, total]) => {
          const result = progressPercent(completed, total);
          expect(result).toBeGreaterThanOrEqual(0);
          expect(result).toBeLessThanOrEqual(100);
        }
      ),
      { numRuns: 100 }
    );
  });

  it('when completed = 0, result is 0', () => {
    fc.assert(
      fc.property(
        fc.integer({ min: 1, max: 10000 }),
        (total) => {
          expect(progressPercent(0, total)).toBe(0);
        }
      ),
      { numRuns: 100 }
    );
  });

  it('when completed = total, result is 100', () => {
    fc.assert(
      fc.property(
        fc.integer({ min: 1, max: 10000 }),
        (total) => {
          expect(progressPercent(total, total)).toBe(100);
        }
      ),
      { numRuns: 100 }
    );
  });

  it('when total = 0, result is 0 (division by zero guard)', () => {
    fc.assert(
      fc.property(
        fc.integer({ min: 0, max: 10000 }),
        (completed) => {
          expect(progressPercent(completed, 0)).toBe(0);
        }
      ),
      { numRuns: 100 }
    );
  });
});
