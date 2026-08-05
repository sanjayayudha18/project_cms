import * as fc from "fast-check";
import { describe, expect, it } from "vitest";
import { formatIDR } from "./format";

/**
 * Property 6: IDR Currency Formatting
 * Validates: Requirements 4.3, 7.6
 *
 * For any non-negative integer value, formatIDR SHALL produce a string
 * beginning with "Rp" followed by the number formatted with dot-separated
 * thousands (Indonesian locale).
 */
describe("formatIDR — Property 6: IDR Currency Formatting", () => {
  const nonNegativeInt = fc.integer({ min: 0, max: Number.MAX_SAFE_INTEGER });

  it("output always starts with 'Rp' prefix", () => {
    fc.assert(
      fc.property(nonNegativeInt, (value) => {
        const result = formatIDR(value);
        expect(result.startsWith("Rp")).toBe(true);
      }),
    );
  });

  it("output after 'Rp' contains only digits and dots", () => {
    fc.assert(
      fc.property(nonNegativeInt, (value) => {
        const result = formatIDR(value);
        const afterPrefix = result.slice(2);
        expect(afterPrefix).toMatch(/^[0-9.]+$/);
      }),
    );
  });

  it("dots are placed correctly as thousand separators (every 3 digits from the right)", () => {
    fc.assert(
      fc.property(nonNegativeInt, (value) => {
        const result = formatIDR(value);
        const afterPrefix = result.slice(2);
        const parts = afterPrefix.split(".");

        // First group can be 1-3 digits
        expect(parts[0]?.length).toBeGreaterThanOrEqual(1);
        expect(parts[0]?.length).toBeLessThanOrEqual(3);

        // All subsequent groups must be exactly 3 digits
        for (let i = 1; i < parts.length; i++) {
          expect(parts[i]?.length).toBe(3);
        }
      }),
    );
  });

  it("numeric value can be reconstructed by removing 'Rp' and dots", () => {
    fc.assert(
      fc.property(nonNegativeInt, (value) => {
        const result = formatIDR(value);
        const numericStr = result.slice(2).replace(/\./g, "");
        const reconstructed = Number.parseInt(numericStr, 10);
        expect(reconstructed).toBe(value);
      }),
    );
  });

  it("output has no trailing or leading whitespace", () => {
    fc.assert(
      fc.property(nonNegativeInt, (value) => {
        const result = formatIDR(value);
        expect(result).toBe(result.trim());
      }),
    );
  });

  it("formatIDR(0) returns 'Rp0' (boundary)", () => {
    expect(formatIDR(0)).toBe("Rp0");
  });

  it("formatIDR(1000) returns 'Rp1.000' (boundary)", () => {
    expect(formatIDR(1000)).toBe("Rp1.000");
  });
});
