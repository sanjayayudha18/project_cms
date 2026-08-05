import { describe, it, expect } from 'vitest';
import * as fc from 'fast-check';
import { filterByField } from '../filters';

/**
 * Property 4: Single-filter state management
 *
 * For any array of records and for any filter value, applying the filter returns
 * only records matching that value (no false positives), includes all records
 * matching that value (no false negatives), and clearing the filter restores the
 * original unfiltered array in its entirety.
 *
 * **Validates: Requirements 4.6, 4.7, 4.9**
 */

interface MockRecord {
  id: number;
  status: 'A' | 'B' | 'C';
}

const statusArb = fc.constantFrom<'A' | 'B' | 'C'>('A', 'B', 'C');

const mockRecordArb: fc.Arbitrary<MockRecord> = fc.record({
  id: fc.integer({ min: 1, max: 10000 }),
  status: statusArb,
});

const recordsArb = fc.array(mockRecordArb, { minLength: 0, maxLength: 50 });

describe('Feature: cms-frontend-prototype, Property 4: Single-filter state management', () => {
  it('filtering returns only matching records — no false positives', () => {
    fc.assert(
      fc.property(recordsArb, statusArb, (records, filterValue) => {
        const result = filterByField(records, 'status', filterValue);
        // Every record in the result must match the filter value
        expect(result.every((r) => r.status === filterValue)).toBe(true);
      }),
      { numRuns: 100 },
    );
  });

  it('filtering returns all matching records — no false negatives', () => {
    fc.assert(
      fc.property(recordsArb, statusArb, (records, filterValue) => {
        const result = filterByField(records, 'status', filterValue);
        // Count of matching records in original must equal result length
        const expectedCount = records.filter((r) => r.status === filterValue).length;
        expect(result.length).toBe(expectedCount);
      }),
      { numRuns: 100 },
    );
  });

  it('clearing filter (null) restores original array', () => {
    fc.assert(
      fc.property(recordsArb, (records) => {
        const result = filterByField(records, 'status', null);
        expect(result).toEqual(records);
      }),
      { numRuns: 100 },
    );
  });
});
