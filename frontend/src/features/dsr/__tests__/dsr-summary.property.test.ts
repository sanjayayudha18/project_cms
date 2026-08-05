// Feature: frontend-consolidation, Property 5: DSR Summary Aggregation Invariant

import * as fc from "fast-check";
import { describe, expect, it } from "vitest";
import { computeDsrTotals } from "../DsrSummary";
import type { DsrStatus, EnrichedDsrRecord } from "../types";

/**
 * Property 5: DSR Summary Aggregation Invariant
 * **Validates: Requirements 5.2**
 *
 * For any non-empty array of DSR records, the DsrSummary totals SHALL satisfy:
 * totalBeginningBalance == sum(records.map(r => r.beginningBalance))
 * totalCashIn == sum(records.map(r => r.cashIn))
 * totalCashOut == sum(records.map(r => r.cashOut))
 * totalEndingBalance == sum(records.map(r => r.endingBalance))
 */

const ALL_STATUSES: DsrStatus[] = ["Critical", "Low", "Normal"];

const arbStatus: fc.Arbitrary<DsrStatus> = fc.constantFrom(...ALL_STATUSES);

const arbEnrichedDsrRecord: fc.Arbitrary<EnrichedDsrRecord> = fc.record({
  id: fc.uuid(),
  atmId: fc.stringMatching(/^ATM-\d{3,5}$/),
  date: fc
    .date({ min: new Date("2024-01-01"), max: new Date("2025-12-31") })
    .map((d) => d.toISOString().slice(0, 10)),
  beginningBalance: fc.integer({ min: 0, max: 1_000_000_000 }),
  cashIn: fc.integer({ min: 0, max: 500_000_000 }),
  cashOut: fc.integer({ min: 0, max: 500_000_000 }),
  endingBalance: fc.integer({ min: 0, max: 1_000_000_000 }),
  status: arbStatus,
  location: fc.string({ minLength: 1, maxLength: 50 }),
  vendorName: fc.string({ minLength: 1, maxLength: 30 }),
});

const arbDsrRecordArray: fc.Arbitrary<EnrichedDsrRecord[]> = fc.array(
  arbEnrichedDsrRecord,
  { minLength: 1, maxLength: 50 },
);

describe("Property 5: DSR Summary Aggregation Invariant", () => {
  it("totalBeginningBalance equals sum of all records' beginningBalance", () => {
    fc.assert(
      fc.property(arbDsrRecordArray, (records) => {
        const totals = computeDsrTotals(records);
        const expectedSum = records.reduce((sum, r) => sum + r.beginningBalance, 0);
        expect(totals.beginningBalance).toBe(expectedSum);
      }),
      { numRuns: 100 },
    );
  });

  it("totalCashIn equals sum of all records' cashIn", () => {
    fc.assert(
      fc.property(arbDsrRecordArray, (records) => {
        const totals = computeDsrTotals(records);
        const expectedSum = records.reduce((sum, r) => sum + r.cashIn, 0);
        expect(totals.cashIn).toBe(expectedSum);
      }),
      { numRuns: 100 },
    );
  });

  it("totalCashOut equals sum of all records' cashOut", () => {
    fc.assert(
      fc.property(arbDsrRecordArray, (records) => {
        const totals = computeDsrTotals(records);
        const expectedSum = records.reduce((sum, r) => sum + r.cashOut, 0);
        expect(totals.cashOut).toBe(expectedSum);
      }),
      { numRuns: 100 },
    );
  });

  it("totalEndingBalance equals sum of all records' endingBalance", () => {
    fc.assert(
      fc.property(arbDsrRecordArray, (records) => {
        const totals = computeDsrTotals(records);
        const expectedSum = records.reduce((sum, r) => sum + r.endingBalance, 0);
        expect(totals.endingBalance).toBe(expectedSum);
      }),
      { numRuns: 100 },
    );
  });

  it("all four totals are consistent simultaneously for any record set", () => {
    fc.assert(
      fc.property(arbDsrRecordArray, (records) => {
        const totals = computeDsrTotals(records);

        const expectedBeginning = records.reduce((sum, r) => sum + r.beginningBalance, 0);
        const expectedCashIn = records.reduce((sum, r) => sum + r.cashIn, 0);
        const expectedCashOut = records.reduce((sum, r) => sum + r.cashOut, 0);
        const expectedEnding = records.reduce((sum, r) => sum + r.endingBalance, 0);

        expect(totals.beginningBalance).toBe(expectedBeginning);
        expect(totals.cashIn).toBe(expectedCashIn);
        expect(totals.cashOut).toBe(expectedCashOut);
        expect(totals.endingBalance).toBe(expectedEnding);
      }),
      { numRuns: 100 },
    );
  });
});
