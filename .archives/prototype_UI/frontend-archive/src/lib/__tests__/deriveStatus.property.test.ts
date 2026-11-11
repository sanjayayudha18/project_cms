import { describe, it, expect } from 'vitest';
import * as fc from 'fast-check';
import { deriveStatus } from '../deriveStatus';

/**
 * **Validates: Requirements 3.4**
 *
 * Property 3: Status derivation from ending balance thresholds
 * - Totality: function always returns exactly one of three values for any non-negative integer
 * - Boundary correctness at 49,999,999 / 50,000,000 / 150,000,000 / 150,000,001
 */
describe('Feature: cms-frontend-prototype, Property 3: Status derivation from ending balance thresholds', () => {
  const validStatuses = ['Critical', 'Low', 'Normal'] as const;

  it('always returns exactly one of three values for any non-negative integer', () => {
    fc.assert(
      fc.property(fc.nat(), (n) => {
        const status = deriveStatus(n);
        expect(validStatuses).toContain(status);
      }),
      { numRuns: 100 }
    );
  });

  it('boundary correctness', () => {
    expect(deriveStatus(49_999_999)).toBe('Critical');
    expect(deriveStatus(50_000_000)).toBe('Low');
    expect(deriveStatus(150_000_000)).toBe('Low');
    expect(deriveStatus(150_000_001)).toBe('Normal');
  });

  it('Critical when below 50M', () => {
    fc.assert(
      fc.property(
        fc.nat({ max: 49_999_999 }),
        (n) => { expect(deriveStatus(n)).toBe('Critical'); }
      ),
      { numRuns: 100 }
    );
  });

  it('Low when 50M to 150M inclusive', () => {
    fc.assert(
      fc.property(
        fc.integer({ min: 50_000_000, max: 150_000_000 }),
        (n) => { expect(deriveStatus(n)).toBe('Low'); }
      ),
      { numRuns: 100 }
    );
  });

  it('Normal when above 150M', () => {
    fc.assert(
      fc.property(
        fc.integer({ min: 150_000_001, max: 999_999_999_999 }),
        (n) => { expect(deriveStatus(n)).toBe('Normal'); }
      ),
      { numRuns: 100 }
    );
  });
});
