import * as fc from "fast-check";
// Feature: frontend-consolidation, Property 7: Reconciliation Filter Behavioral Equivalence
import { describe, expect, it } from "vitest";
import { filterExceptions } from "../reconciliation.utils";
import type { ReconciliationException } from "../types";

/**
 * Property 7: Reconciliation Filter Behavioral Equivalence
 *
 * For any array of reconciliation exceptions and any filter criteria,
 * applying Target_App's filterExceptions function SHALL produce results
 * matching the expected semantics:
 *   - severity "High" → only severity === 'high'
 *   - severity "Medium" → only severity === 'medium'
 *   - "All severity" → all records
 *   - "Open exceptions" → owner === null
 *   - "Resolved" → owner !== null
 *   - "All records" → all records
 *
 * **Validates: Requirements 8.4**
 */

const severityArb = fc.constantFrom<"high" | "medium">("high", "medium");

const ownerArb = fc.oneof(fc.constant(null), fc.string({ minLength: 1, maxLength: 20 }));

const reconciliationExceptionArb: fc.Arbitrary<ReconciliationException> = fc.record({
  id: fc.uuid(),
  atmId: fc.string({ minLength: 3, maxLength: 10 }),
  lastCountTime: fc.date().map((d) => d.toISOString()),
  location: fc.string({ minLength: 1, maxLength: 30 }),
  countedAmount: fc.integer({ min: 0, max: 100_000_000 }),
  escrowAmount: fc.integer({ min: 0, max: 100_000_000 }),
  difference: fc.integer({ min: -10_000_000, max: 10_000_000 }),
  severity: severityArb,
  owner: ownerArb,
});

const exceptionsArb = fc.array(reconciliationExceptionArb, { minLength: 0, maxLength: 50 });

const severityFilterArb = fc.constantFrom("All severity", "High", "Medium");
const exceptionTypeFilterArb = fc.constantFrom("Open exceptions", "All records", "Resolved");

describe("Feature: frontend-consolidation, Property 7: Reconciliation Filter Behavioral Equivalence", () => {
  it('severity "High" returns only records with severity === "high"', () => {
    fc.assert(
      fc.property(exceptionsArb, (records) => {
        const result = filterExceptions(records, "High", "All records");
        expect(result.every((r) => r.severity === "high")).toBe(true);
        const expected = records.filter((r) => r.severity === "high");
        expect(result).toEqual(expected);
      }),
      { numRuns: 100 },
    );
  });

  it('severity "Medium" returns only records with severity === "medium"', () => {
    fc.assert(
      fc.property(exceptionsArb, (records) => {
        const result = filterExceptions(records, "Medium", "All records");
        expect(result.every((r) => r.severity === "medium")).toBe(true);
        const expected = records.filter((r) => r.severity === "medium");
        expect(result).toEqual(expected);
      }),
      { numRuns: 100 },
    );
  });

  it('"All severity" returns all records regardless of severity', () => {
    fc.assert(
      fc.property(exceptionsArb, (records) => {
        const result = filterExceptions(records, "All severity", "All records");
        expect(result).toEqual(records);
      }),
      { numRuns: 100 },
    );
  });

  it('"Open exceptions" returns only records where owner is null', () => {
    fc.assert(
      fc.property(exceptionsArb, (records) => {
        const result = filterExceptions(records, "All severity", "Open exceptions");
        expect(result.every((r) => r.owner === null)).toBe(true);
        const expected = records.filter((r) => r.owner === null);
        expect(result).toEqual(expected);
      }),
      { numRuns: 100 },
    );
  });

  it('"Resolved" returns only records where owner is not null', () => {
    fc.assert(
      fc.property(exceptionsArb, (records) => {
        const result = filterExceptions(records, "All severity", "Resolved");
        expect(result.every((r) => r.owner !== null)).toBe(true);
        const expected = records.filter((r) => r.owner !== null);
        expect(result).toEqual(expected);
      }),
      { numRuns: 100 },
    );
  });

  it('"All records" returns all records regardless of owner', () => {
    fc.assert(
      fc.property(exceptionsArb, (records) => {
        const result = filterExceptions(records, "All severity", "All records");
        expect(result).toEqual(records);
      }),
      { numRuns: 100 },
    );
  });

  it("combined filters apply as AND — severity and exceptionType both constrain", () => {
    fc.assert(
      fc.property(
        exceptionsArb,
        severityFilterArb,
        exceptionTypeFilterArb,
        (records, severity, exceptionType) => {
          const result = filterExceptions(records, severity, exceptionType);

          // Verify every result matches both criteria
          for (const record of result) {
            // Check severity
            if (severity === "High") expect(record.severity).toBe("high");
            if (severity === "Medium") expect(record.severity).toBe("medium");

            // Check exception type
            if (exceptionType === "Open exceptions") expect(record.owner).toBeNull();
            if (exceptionType === "Resolved") expect(record.owner).not.toBeNull();
          }

          // Verify no matching record is excluded (no false negatives)
          const expected = records.filter((r) => {
            const passesSeverity =
              severity === "All severity" ||
              (severity === "High" && r.severity === "high") ||
              (severity === "Medium" && r.severity === "medium");
            const passesType =
              exceptionType === "All records" ||
              (exceptionType === "Open exceptions" && r.owner === null) ||
              (exceptionType === "Resolved" && r.owner !== null);
            return passesSeverity && passesType;
          });
          expect(result).toEqual(expected);
        },
      ),
      { numRuns: 100 },
    );
  });

  it("filter result is always a subset of the input — no records are invented", () => {
    fc.assert(
      fc.property(
        exceptionsArb,
        severityFilterArb,
        exceptionTypeFilterArb,
        (records, severity, exceptionType) => {
          const result = filterExceptions(records, severity, exceptionType);
          expect(result.length).toBeLessThanOrEqual(records.length);
          for (const r of result) {
            expect(records).toContainEqual(r);
          }
        },
      ),
      { numRuns: 100 },
    );
  });

  it("filter preserves original order of matching records", () => {
    fc.assert(
      fc.property(
        exceptionsArb,
        severityFilterArb,
        exceptionTypeFilterArb,
        (records, severity, exceptionType) => {
          const result = filterExceptions(records, severity, exceptionType);
          // For each consecutive pair in result, their indices in the original
          // array must be in ascending order
          const indices = result.map((r) => records.indexOf(r));
          for (let i = 1; i < indices.length; i++) {
            expect(indices[i]).toBeGreaterThan(indices[i - 1]);
          }
        },
      ),
      { numRuns: 100 },
    );
  });
});
