/**
 * Property-based test: Currency formatting consistency
 *
 * Validates: Requirements 3.3, 4.1
 */
import { describe, it, expect } from 'vitest';
import * as fc from 'fast-check';
import { formatIDR, parseIDR } from '../formatCurrency';

describe('Feature: cms-frontend-prototype, Property 2: Currency formatting consistency', () => {
  it('round-trips: parseIDR(formatIDR(n)) === n for any non-negative integer', () => {
    fc.assert(
      fc.property(
        fc.nat({ max: 999_999_999_999 }),
        (n) => {
          expect(parseIDR(formatIDR(n))).toBe(n);
        }
      ),
      { numRuns: 100 }
    );
  });

  it('output uses dot-separated thousands with no decimal places', () => {
    fc.assert(
      fc.property(
        fc.nat({ max: 999_999_999_999 }),
        (n) => {
          const formatted = formatIDR(n);
          // No comma (Indonesian locale uses dots, not commas)
          expect(formatted).not.toMatch(/,/);
          // No decimal places (no dot followed by 1-2 trailing digits at end)
          expect(formatted).not.toMatch(/\.\d{1,2}$/);
        }
      ),
      { numRuns: 100 }
    );
  });
});
