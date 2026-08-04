/**
 * Property-based test: Badge count display formatting
 *
 * Feature: cms-dashboard-redesign, Property 1: Badge count display formatting
 *
 * For any non-negative integer count, the badge display function SHALL return:
 * no badge (null) when count is 0, the count as a string when count is 1–99,
 * and "99+" when count exceeds 99.
 *
 * Validates: Requirements 1.6
 */
import { describe, it, expect } from 'vitest';
import * as fc from 'fast-check';
import { formatBadgeCount } from '../formatters';

describe('Feature: cms-dashboard-redesign, Property 1: Badge count display formatting', () => {
  it('returns null for count = 0', () => {
    fc.assert(
      fc.property(fc.constant(0), (count) => {
        expect(formatBadgeCount(count)).toBeNull();
      }),
      { numRuns: 100 }
    );
  });

  it('returns the count as a string for any integer in [1, 99]', () => {
    fc.assert(
      fc.property(fc.integer({ min: 1, max: 99 }), (count) => {
        const result = formatBadgeCount(count);
        expect(result).toBe(String(count));
      }),
      { numRuns: 100 }
    );
  });

  it('returns "99+" for any integer > 99', () => {
    fc.assert(
      fc.property(fc.integer({ min: 100, max: 1_000_000 }), (count) => {
        expect(formatBadgeCount(count)).toBe('99+');
      }),
      { numRuns: 100 }
    );
  });

  it('returns null for any negative integer', () => {
    fc.assert(
      fc.property(fc.integer({ min: -1_000_000, max: -1 }), (count) => {
        expect(formatBadgeCount(count)).toBeNull();
      }),
      { numRuns: 100 }
    );
  });
});
