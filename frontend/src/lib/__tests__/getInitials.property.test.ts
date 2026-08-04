/**
 * Property-based test: Avatar initials extraction
 *
 * Feature: cms-dashboard-redesign, Property 2: Avatar initials extraction
 *
 * For any non-empty user name string containing a first name and last name,
 * the initials function SHALL return exactly 2 characters consisting of the
 * uppercase first letter of the first name and the uppercase first letter of the last name.
 *
 * Validates: Requirements 2.5
 */
import { describe, it, expect } from 'vitest';
import * as fc from 'fast-check';
import { getInitials } from '../formatters';

/** Generator for valid name parts (2–10 alphabetical characters) */
const namePart = fc.stringMatching(/^[A-Za-z]{2,10}$/);

describe('Feature: cms-dashboard-redesign, Property 2: Avatar initials extraction', () => {
  it('for any two-word name (first + last), returns exactly 2 uppercase characters', () => {
    fc.assert(
      fc.property(namePart, namePart, (first, last) => {
        const result = getInitials(`${first} ${last}`);
        expect(result).toHaveLength(2);
        expect(result).toMatch(/^[A-Z]{2}$/);
      }),
      { numRuns: 100 }
    );
  });

  it('the first character equals the uppercase first letter of the first name', () => {
    fc.assert(
      fc.property(namePart, namePart, (first, last) => {
        const result = getInitials(`${first} ${last}`);
        expect(result[0]).toBe(first[0]!.toUpperCase());
      }),
      { numRuns: 100 }
    );
  });

  it('the second character equals the uppercase first letter of the last name', () => {
    fc.assert(
      fc.property(namePart, namePart, (first, last) => {
        const result = getInitials(`${first} ${last}`);
        expect(result[1]).toBe(last[0]!.toUpperCase());
      }),
      { numRuns: 100 }
    );
  });

  it('for single-word names, returns exactly 1 uppercase character', () => {
    fc.assert(
      fc.property(namePart, (name) => {
        const result = getInitials(name);
        expect(result).toHaveLength(1);
        expect(result).toMatch(/^[A-Z]$/);
        expect(result).toBe(name[0]!.toUpperCase());
      }),
      { numRuns: 100 }
    );
  });

  it('for empty string, returns empty string', () => {
    expect(getInitials('')).toBe('');
  });

  it('result is always uppercase', () => {
    fc.assert(
      fc.property(namePart, namePart, (first, last) => {
        const result = getInitials(`${first} ${last}`);
        expect(result).toBe(result.toUpperCase());
      }),
      { numRuns: 100 }
    );
  });
});
