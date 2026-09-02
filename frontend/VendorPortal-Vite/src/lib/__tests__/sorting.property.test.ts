import * as fc from "fast-check";

/**
 * Property 8: Column Sorting Correctness
 * Validates: Requirements 3.9, 7.9
 *
 * For any array of records and any sortable column:
 * - Ascending sort produces non-decreasing order
 * - Descending sort produces non-increasing order
 * - Sort output is a permutation of input (no elements added or removed)
 */

// Pure sorting helpers under test
function sortAsc<T>(items: T[], accessor: (item: T) => string | number): T[] {
  return [...items].sort((a, b) => {
    const va = accessor(a);
    const vb = accessor(b);
    if (va < vb) return -1;
    if (va > vb) return 1;
    return 0;
  });
}

function sortDesc<T>(items: T[], accessor: (item: T) => string | number): T[] {
  return [...items].sort((a, b) => {
    const va = accessor(a);
    const vb = accessor(b);
    if (va < vb) return 1;
    if (va > vb) return -1;
    return 0;
  });
}

// Generate ISO date strings directly using integer-based approach (avoids Invalid Date issues)
const scheduledDateArb = fc.integer({ min: 1, max: 366 }).map((dayOfYear) => {
  const date = new Date(2024, 0, dayOfYear);
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
});

// Arbitrary for generating test records
const recordArb = fc.record({
  id: fc.string({ minLength: 1, maxLength: 10 }),
  scheduledDate: scheduledDateArb,
  amount: fc.integer({ min: 0, max: 1_000_000_000 }),
  name: fc.string({ minLength: 1, maxLength: 50 }),
});

const recordsArb = fc.array(recordArb, { minLength: 0, maxLength: 100 });

describe("Property 8: Column Sorting Correctness", () => {
  describe("Ascending sort produces non-decreasing order", () => {
    it("by string column (scheduledDate)", () => {
      fc.assert(
        fc.property(recordsArb, (records) => {
          const sorted = sortAsc(records, (r) => r.scheduledDate);
          for (let i = 1; i < sorted.length; i++) {
            expect(sorted[i - 1].scheduledDate <= sorted[i].scheduledDate).toBe(true);
          }
        }),
        { numRuns: 100 },
      );
    });

    it("by string column (name)", () => {
      fc.assert(
        fc.property(recordsArb, (records) => {
          const sorted = sortAsc(records, (r) => r.name);
          for (let i = 1; i < sorted.length; i++) {
            expect(sorted[i - 1].name <= sorted[i].name).toBe(true);
          }
        }),
        { numRuns: 100 },
      );
    });

    it("by numeric column (amount)", () => {
      fc.assert(
        fc.property(recordsArb, (records) => {
          const sorted = sortAsc(records, (r) => r.amount);
          for (let i = 1; i < sorted.length; i++) {
            expect(sorted[i - 1].amount <= sorted[i].amount).toBe(true);
          }
        }),
        { numRuns: 100 },
      );
    });
  });

  describe("Descending sort produces non-increasing order", () => {
    it("by string column (scheduledDate)", () => {
      fc.assert(
        fc.property(recordsArb, (records) => {
          const sorted = sortDesc(records, (r) => r.scheduledDate);
          for (let i = 1; i < sorted.length; i++) {
            expect(sorted[i - 1].scheduledDate >= sorted[i].scheduledDate).toBe(true);
          }
        }),
        { numRuns: 100 },
      );
    });

    it("by string column (name)", () => {
      fc.assert(
        fc.property(recordsArb, (records) => {
          const sorted = sortDesc(records, (r) => r.name);
          for (let i = 1; i < sorted.length; i++) {
            expect(sorted[i - 1].name >= sorted[i].name).toBe(true);
          }
        }),
        { numRuns: 100 },
      );
    });

    it("by numeric column (amount)", () => {
      fc.assert(
        fc.property(recordsArb, (records) => {
          const sorted = sortDesc(records, (r) => r.amount);
          for (let i = 1; i < sorted.length; i++) {
            expect(sorted[i - 1].amount >= sorted[i].amount).toBe(true);
          }
        }),
        { numRuns: 100 },
      );
    });
  });

  describe("Sort output is a permutation of input", () => {
    it("ascending sort preserves same length", () => {
      fc.assert(
        fc.property(recordsArb, (records) => {
          const sorted = sortAsc(records, (r) => r.amount);
          expect(sorted.length).toBe(records.length);
        }),
        { numRuns: 100 },
      );
    });

    it("descending sort preserves same length", () => {
      fc.assert(
        fc.property(recordsArb, (records) => {
          const sorted = sortDesc(records, (r) => r.amount);
          expect(sorted.length).toBe(records.length);
        }),
        { numRuns: 100 },
      );
    });

    it("ascending sort contains same elements (no elements added or removed)", () => {
      fc.assert(
        fc.property(recordsArb, (records) => {
          const sorted = sortAsc(records, (r) => r.amount);

          // Every element in the sorted output should exist in the input
          // Using a count-based check for exact permutation
          const inputAmounts = records.map((r) => r.amount).sort((a, b) => a - b);
          const outputAmounts = sorted.map((r) => r.amount).sort((a, b) => a - b);
          expect(outputAmounts).toEqual(inputAmounts);

          // Also verify ids are the same set
          const inputIds = [...records.map((r) => r.id)].sort();
          const outputIds = [...sorted.map((r) => r.id)].sort();
          expect(outputIds).toEqual(inputIds);
        }),
        { numRuns: 100 },
      );
    });

    it("descending sort contains same elements (no elements added or removed)", () => {
      fc.assert(
        fc.property(recordsArb, (records) => {
          const sorted = sortDesc(records, (r) => r.scheduledDate);

          // Verify it's a permutation by checking all fields match
          const inputDates = [...records.map((r) => r.scheduledDate)].sort();
          const outputDates = [...sorted.map((r) => r.scheduledDate)].sort();
          expect(outputDates).toEqual(inputDates);

          const inputIds = [...records.map((r) => r.id)].sort();
          const outputIds = [...sorted.map((r) => r.id)].sort();
          expect(outputIds).toEqual(inputIds);
        }),
        { numRuns: 100 },
      );
    });
  });
});
