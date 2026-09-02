import {
  formatBadgeCount,
  formatIDR,
  formatRp,
  getBalanceStatus,
  truncate,
} from "@/lib/formatters";
import * as fc from "fast-check";

/**
 * Property 2: IDR Currency Formatting
 * Validates: Requirements 3.7, 5.5, 6.7, 7.8
 *
 * For any non-negative integer amount, formatIDR and formatRp produce
 * correct dot-separated format and parsing back yields the original.
 */
describe("Property 2: IDR Currency Formatting", () => {
  it('formatIDR produces string starting with "IDR " for any non-negative integer', () => {
    fc.assert(
      fc.property(fc.integer({ min: 0, max: 999_999_999_999 }), (amount) => {
        const result = formatIDR(amount);
        expect(result.startsWith("IDR ")).toBe(true);
      }),
      { numRuns: 100 },
    );
  });

  it('formatRp produces string starting with "Rp " for any non-negative integer', () => {
    fc.assert(
      fc.property(fc.integer({ min: 0, max: 999_999_999_999 }), (amount) => {
        const result = formatRp(amount);
        expect(result.startsWith("Rp ")).toBe(true);
      }),
      { numRuns: 100 },
    );
  });

  it("parsing formatIDR back (remove prefix, replace dots) yields original amount", () => {
    fc.assert(
      fc.property(fc.integer({ min: 0, max: 999_999_999_999 }), (amount) => {
        const result = formatIDR(amount);
        const numericPart = result.replace("IDR ", "").replace(/\./g, "");
        expect(Number.parseInt(numericPart, 10)).toBe(amount);
      }),
      { numRuns: 100 },
    );
  });

  it("parsing formatRp back (remove prefix, replace dots) yields original amount", () => {
    fc.assert(
      fc.property(fc.integer({ min: 0, max: 999_999_999_999 }), (amount) => {
        const result = formatRp(amount);
        const numericPart = result.replace("Rp ", "").replace(/\./g, "");
        expect(Number.parseInt(numericPart, 10)).toBe(amount);
      }),
      { numRuns: 100 },
    );
  });
});

/**
 * Property 3: Badge Count Formatting
 * Validates: Requirements 2.7, 8.5
 *
 * For any non-negative integer count: 0 → null, 1-99 → String(count), >99 → "99+"
 */
describe("Property 3: Badge Count Formatting", () => {
  it("returns null for count 0", () => {
    expect(formatBadgeCount(0)).toBeNull();
  });

  it("returns String(count) for count 1-99", () => {
    fc.assert(
      fc.property(fc.integer({ min: 1, max: 99 }), (count) => {
        const result = formatBadgeCount(count);
        expect(result).toBe(String(count));
      }),
      { numRuns: 100 },
    );
  });

  it('returns "99+" for count > 99', () => {
    fc.assert(
      fc.property(fc.integer({ min: 100, max: 1000 }), (count) => {
        const result = formatBadgeCount(count);
        expect(result).toBe("99+");
      }),
      { numRuns: 100 },
    );
  });
});

/**
 * Property 4: String Truncation
 * Validates: Requirements 2.2
 *
 * For any string and positive maxLength: preserves input when within maxLength,
 * appends "..." when exceeding.
 */
describe("Property 4: String Truncation", () => {
  it("preserves input when text.length <= maxLength", () => {
    fc.assert(
      fc.property(
        fc.string({ minLength: 0, maxLength: 200 }),
        fc.integer({ min: 1, max: 200 }),
        (text, maxLength) => {
          fc.pre(text.length <= maxLength);
          const result = truncate(text, maxLength);
          expect(result).toBe(text);
        },
      ),
      { numRuns: 100 },
    );
  });

  it('appends "..." when text.length > maxLength', () => {
    fc.assert(
      fc.property(
        fc.string({ minLength: 1, maxLength: 200 }),
        fc.integer({ min: 1, max: 200 }),
        (text, maxLength) => {
          fc.pre(text.length > maxLength);
          const result = truncate(text, maxLength);
          expect(result).toBe(`${text.slice(0, maxLength)}...`);
          expect(result.length).toBe(maxLength + 3);
        },
      ),
      { numRuns: 100 },
    );
  });
});

/**
 * Property 5: Balance Status Classification
 * Validates: Requirements 7.3
 *
 * For any non-negative integer endingBalance, getBalanceStatus returns:
 * - "Critical" when < 50,000,000
 * - "Low" when >= 50,000,000 and <= 150,000,000
 * - "Normal" when > 150,000,000
 * The three ranges are exhaustive and mutually exclusive.
 */
describe("Property 5: Balance Status Classification", () => {
  it('returns "Critical" for endingBalance < 50,000,000', () => {
    fc.assert(
      fc.property(fc.integer({ min: 0, max: 49_999_999 }), (endingBalance) => {
        expect(getBalanceStatus(endingBalance)).toBe("Critical");
      }),
      { numRuns: 100 },
    );
  });

  it('returns "Low" for endingBalance >= 50,000,000 and <= 150,000,000', () => {
    fc.assert(
      fc.property(fc.integer({ min: 50_000_000, max: 150_000_000 }), (endingBalance) => {
        expect(getBalanceStatus(endingBalance)).toBe("Low");
      }),
      { numRuns: 100 },
    );
  });

  it('returns "Normal" for endingBalance > 150,000,000', () => {
    fc.assert(
      fc.property(fc.integer({ min: 150_000_001, max: 500_000_000 }), (endingBalance) => {
        expect(getBalanceStatus(endingBalance)).toBe("Normal");
      }),
      { numRuns: 100 },
    );
  });

  it("ranges are exhaustive — every non-negative integer maps to exactly one status", () => {
    fc.assert(
      fc.property(fc.integer({ min: 0, max: 500_000_000 }), (endingBalance) => {
        const status = getBalanceStatus(endingBalance);
        expect(["Critical", "Low", "Normal"]).toContain(status);

        // Verify mutual exclusivity
        if (endingBalance < 50_000_000) {
          expect(status).toBe("Critical");
        } else if (endingBalance <= 150_000_000) {
          expect(status).toBe("Low");
        } else {
          expect(status).toBe("Normal");
        }
      }),
      { numRuns: 100 },
    );
  });
});
