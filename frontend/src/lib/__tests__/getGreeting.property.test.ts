/**
 * Property-based test: Time-of-day greeting
 *
 * Validates: Requirements 3.1
 */
import { describe, it, expect } from 'vitest';
import * as fc from 'fast-check';
import { getGreeting } from '../formatters';

describe('Feature: cms-dashboard-redesign, Property 3: Time-of-day greeting', () => {
  it('returns "Good morning" for any hour in [0, 11]', () => {
    fc.assert(
      fc.property(
        fc.integer({ min: 0, max: 11 }),
        (hour) => {
          expect(getGreeting(hour)).toBe('Good morning');
        }
      ),
      { numRuns: 100 }
    );
  });

  it('returns "Good afternoon" for any hour in [12, 17]', () => {
    fc.assert(
      fc.property(
        fc.integer({ min: 12, max: 17 }),
        (hour) => {
          expect(getGreeting(hour)).toBe('Good afternoon');
        }
      ),
      { numRuns: 100 }
    );
  });

  it('returns "Good evening" for any hour in [18, 23]', () => {
    fc.assert(
      fc.property(
        fc.integer({ min: 18, max: 23 }),
        (hour) => {
          expect(getGreeting(hour)).toBe('Good evening');
        }
      ),
      { numRuns: 100 }
    );
  });

  it('always returns one of the three valid greetings for any hour in [0, 23]', () => {
    const validGreetings = ['Good morning', 'Good afternoon', 'Good evening'];

    fc.assert(
      fc.property(
        fc.integer({ min: 0, max: 23 }),
        (hour) => {
          const result = getGreeting(hour);
          expect(validGreetings).toContain(result);
        }
      ),
      { numRuns: 100 }
    );
  });
});
