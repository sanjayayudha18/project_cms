/**
 * Property-based test: Reconciliation filter correctness
 *
 * Validates: Requirements 7.7, 7.8
 */
import { describe, it, expect } from 'vitest';
import * as fc from 'fast-check';
import {
  filterExceptions,
  type ReconciliationException,
} from '../reconciliation.utils';

const severityArb = fc.constantFrom('high' as const, 'medium' as const);

const ownerArb = fc.oneof(
  fc.constant(null),
  fc.string({ minLength: 1, maxLength: 20 }),
);

const reconciliationExceptionArb: fc.Arbitrary<ReconciliationException> =
  fc.record({
    id: fc.uuid(),
    atmId: fc.string({ minLength: 3, maxLength: 10 }),
    lastCountTime: fc.integer({ min: 1577836800000, max: 1924905600000 }).map((ts) => new Date(ts).toISOString()),
    location: fc.string({ minLength: 1, maxLength: 30 }),
    countedAmount: fc.integer({ min: 0, max: 100_000_000 }),
    escrowAmount: fc.integer({ min: 0, max: 100_000_000 }),
    difference: fc.integer({ min: -50_000_000, max: 50_000_000 }),
    severity: severityArb,
    owner: ownerArb,
  });

const severityFilterArb = fc.constantFrom(
  'All severity',
  'All',
  'High',
  'Medium',
);

const exceptionTypeFilterArb = fc.constantFrom(
  'Open exceptions',
  'All records',
  'All',
  'Resolved',
);

describe('Feature: cms-dashboard-redesign, Property 9: Reconciliation filter correctness', () => {
  it('every record in filtered result matches the severity filter', () => {
    fc.assert(
      fc.property(
        fc.array(reconciliationExceptionArb, { minLength: 0, maxLength: 20 }),
        severityFilterArb,
        exceptionTypeFilterArb,
        (records, severity, exceptionType) => {
          const result = filterExceptions(records, severity, exceptionType);

          for (const record of result) {
            if (severity === 'High') {
              expect(record.severity).toBe('high');
            } else if (severity === 'Medium') {
              expect(record.severity).toBe('medium');
            }
            // "All severity" and "All" allow any severity
          }
        },
      ),
      { numRuns: 100 },
    );
  });

  it('every record in filtered result matches the exception type filter', () => {
    fc.assert(
      fc.property(
        fc.array(reconciliationExceptionArb, { minLength: 0, maxLength: 20 }),
        severityFilterArb,
        exceptionTypeFilterArb,
        (records, severity, exceptionType) => {
          const result = filterExceptions(records, severity, exceptionType);

          for (const record of result) {
            if (exceptionType === 'Open exceptions') {
              expect(record.owner).toBeNull();
            } else if (exceptionType === 'Resolved') {
              expect(record.owner).not.toBeNull();
            }
            // "All records" and "All" allow any owner status
          }
        },
      ),
      { numRuns: 100 },
    );
  });

  it('combined filters work as AND logic — both predicates hold simultaneously', () => {
    fc.assert(
      fc.property(
        fc.array(reconciliationExceptionArb, { minLength: 0, maxLength: 20 }),
        severityFilterArb,
        exceptionTypeFilterArb,
        (records, severity, exceptionType) => {
          const result = filterExceptions(records, severity, exceptionType);

          for (const record of result) {
            // Severity predicate
            if (severity === 'High') {
              expect(record.severity).toBe('high');
            } else if (severity === 'Medium') {
              expect(record.severity).toBe('medium');
            }
            // Exception type predicate
            if (exceptionType === 'Open exceptions') {
              expect(record.owner).toBeNull();
            } else if (exceptionType === 'Resolved') {
              expect(record.owner).not.toBeNull();
            }
          }
        },
      ),
      { numRuns: 100 },
    );
  });

  it('"All records" + "All severity" returns the full dataset', () => {
    fc.assert(
      fc.property(
        fc.array(reconciliationExceptionArb, { minLength: 0, maxLength: 20 }),
        fc.constantFrom('All severity', 'All'),
        fc.constantFrom('All records', 'All'),
        (records, severity, exceptionType) => {
          const result = filterExceptions(records, severity, exceptionType);
          expect(result).toHaveLength(records.length);
        },
      ),
      { numRuns: 100 },
    );
  });

  it('filtered result is always a subset of the original array', () => {
    fc.assert(
      fc.property(
        fc.array(reconciliationExceptionArb, { minLength: 0, maxLength: 20 }),
        severityFilterArb,
        exceptionTypeFilterArb,
        (records, severity, exceptionType) => {
          const result = filterExceptions(records, severity, exceptionType);

          expect(result.length).toBeLessThanOrEqual(records.length);
          for (const record of result) {
            expect(records).toContain(record);
          }
        },
      ),
      { numRuns: 100 },
    );
  });
});
