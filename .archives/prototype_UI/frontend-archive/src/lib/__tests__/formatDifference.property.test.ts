/**
 * Property-based test: Difference sign formatting
 *
 * Feature: cms-dashboard-redesign, Property 8: Difference sign formatting
 *
 * For any non-zero numeric difference value, the formatting function SHALL return
 * a string with "- " prefix and danger color class when the value is negative,
 * and "+ " prefix and success color class when the value is positive.
 *
 * Validates: Requirements 7.5
 */
import { describe, it, expect } from 'vitest';
import * as fc from 'fast-check';
import { formatDifference } from '../formatters';

describe('Feature: cms-dashboard-redesign, Property 8: Difference sign formatting', () => {
  it('negative integers produce "- IDR " prefix and "text-danger" colorClass', () => {
    fc.assert(
      fc.property(
        fc.integer({ min: -1_000_000_000, max: -1 }),
        (value) => {
          const result = formatDifference(value);
          expect(result.text.startsWith('- IDR ')).toBe(true);
          expect(result.colorClass).toBe('text-danger');
        }
      ),
      { numRuns: 100 }
    );
  });

  it('positive integers produce "+ IDR " prefix and "text-success" colorClass', () => {
    fc.assert(
      fc.property(
        fc.integer({ min: 1, max: 1_000_000_000 }),
        (value) => {
          const result = formatDifference(value);
          expect(result.text.startsWith('+ IDR ')).toBe(true);
          expect(result.colorClass).toBe('text-success');
        }
      ),
      { numRuns: 100 }
    );
  });

  it('zero produces "IDR 0" text and empty colorClass', () => {
    const result = formatDifference(0);
    expect(result.text).toBe('IDR 0');
    expect(result.colorClass).toBe('');
  });

  it('numeric portion after sign and "IDR " never contains a negative sign', () => {
    fc.assert(
      fc.property(
        fc.integer({ min: -1_000_000_000, max: 1_000_000_000 }),
        (value) => {
          const result = formatDifference(value);
          // Extract numeric portion: strip sign prefix and "IDR "
          let numericPortion: string;
          if (result.text.startsWith('- IDR ')) {
            numericPortion = result.text.slice('- IDR '.length);
          } else if (result.text.startsWith('+ IDR ')) {
            numericPortion = result.text.slice('+ IDR '.length);
          } else {
            // Zero case: "IDR 0"
            numericPortion = result.text.slice('IDR '.length);
          }
          expect(numericPortion).not.toContain('-');
        }
      ),
      { numRuns: 100 }
    );
  });
});
