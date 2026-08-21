import * as fc from "fast-check";
import { describe, expect, it } from "vitest";
import { formatAtmDate, formatAtmTime, formatRupiah } from "./formatters";

const MONTH_NAMES = [
  "Jan",
  "Feb",
  "Mar",
  "Apr",
  "Mei",
  "Jun",
  "Jul",
  "Agu",
  "Sep",
  "Okt",
  "Nov",
  "Des",
];

/**
 * Feature: atm-portal
 * Property 8: Currency formatting
 * Validates: Requirements 4.3, 4.4
 *
 * For any numeric value >= 0 (including zero and values up to 10^12), the
 * Rupiah formatter SHALL produce a string matching the pattern
 * "Rp X.XXX.XXX" (dot-separated thousands, no decimal fraction). For any
 * null input, the formatter SHALL return "—" (em-dash).
 */
describe("formatRupiah — Property 8: Currency formatting", () => {
  const nonNegativeAmount = fc.integer({ min: 0, max: 1_000_000_000_000 });

  it("always starts with 'Rp ' prefix", () => {
    fc.assert(
      fc.property(nonNegativeAmount, (value) => {
        expect(formatRupiah(value).startsWith("Rp ")).toBe(true);
      }),
    );
  });

  it("contains only digits and dots after the prefix", () => {
    fc.assert(
      fc.property(nonNegativeAmount, (value) => {
        const afterPrefix = formatRupiah(value).slice(3);
        expect(afterPrefix).toMatch(/^[0-9.]+$/);
      }),
    );
  });

  it("groups digits in 3s from the right, first group 1-3 digits", () => {
    fc.assert(
      fc.property(nonNegativeAmount, (value) => {
        const afterPrefix = formatRupiah(value).slice(3);
        const groups = afterPrefix.split(".");
        expect(groups[0]?.length).toBeGreaterThanOrEqual(1);
        expect(groups[0]?.length).toBeLessThanOrEqual(3);
        for (let i = 1; i < groups.length; i++) {
          expect(groups[i]?.length).toBe(3);
        }
      }),
    );
  });

  it("has no decimal fraction (no comma, no period followed by <3 digits)", () => {
    fc.assert(
      fc.property(nonNegativeAmount, (value) => {
        expect(formatRupiah(value)).not.toMatch(/,/);
      }),
    );
  });

  it("round-trips: stripping 'Rp ' and dots reconstructs the original value", () => {
    fc.assert(
      fc.property(nonNegativeAmount, (value) => {
        const digitsOnly = formatRupiah(value).slice(3).replace(/\./g, "");
        expect(Number.parseInt(digitsOnly, 10)).toBe(value);
      }),
    );
  });

  it("returns '—' for null", () => {
    expect(formatRupiah(null)).toBe("—");
  });

  it("boundary: 0 -> 'Rp 0'", () => {
    expect(formatRupiah(0)).toBe("Rp 0");
  });

  it("boundary: 10^12 -> 'Rp 1.000.000.000.000'", () => {
    expect(formatRupiah(1_000_000_000_000)).toBe("Rp 1.000.000.000.000");
  });
});

/**
 * Feature: atm-portal
 * Property 9: Date formatting
 * Validates: Requirements 4.6, 7.2
 *
 * For any valid Date input, the Indonesian locale formatter SHALL produce a
 * string matching "dd MMM yyyy" (e.g., "15 Jul 2026"), and the HH:mm
 * variant SHALL produce a zero-padded 24-hour time string. For any null
 * input, both formatters SHALL return "—".
 */
describe("formatAtmDate / formatAtmTime — Property 9: Date formatting", () => {
  const validDate = fc.date({
    min: new Date("1900-01-01T00:00:00Z"),
    max: new Date("2200-12-31T00:00:00Z"),
    noInvalidDate: true,
  });

  it("formatAtmDate matches the 'dd MMM yyyy' shape with an Indonesian month", () => {
    fc.assert(
      fc.property(validDate, (date) => {
        const monthPattern = MONTH_NAMES.join("|");
        expect(formatAtmDate(date)).toMatch(new RegExp(`^\\d{2} (${monthPattern}) \\d{4}$`));
      }),
    );
  });

  it("formatAtmDate's day/month/year components match the date's UTC fields", () => {
    fc.assert(
      fc.property(validDate, (date) => {
        const [dayStr, monthStr, yearStr] = formatAtmDate(date).split(" ");
        expect(Number.parseInt(dayStr ?? "", 10)).toBe(date.getUTCDate());
        expect(monthStr).toBe(MONTH_NAMES[date.getUTCMonth()]);
        expect(Number.parseInt(yearStr ?? "", 10)).toBe(date.getUTCFullYear());
      }),
    );
  });

  it("formatAtmDate returns '—' for null", () => {
    expect(formatAtmDate(null)).toBe("—");
  });

  it("formatAtmDate boundary: 2026-07-15 -> '15 Jul 2026'", () => {
    expect(formatAtmDate(new Date("2026-07-15T00:00:00Z"))).toBe("15 Jul 2026");
  });

  it("formatAtmTime matches the zero-padded 'HH:mm' shape", () => {
    fc.assert(
      fc.property(validDate, (date) => {
        expect(formatAtmTime(date)).toMatch(/^\d{2}:\d{2}$/);
      }),
    );
  });

  it("formatAtmTime's hour/minute components match the date's local fields", () => {
    fc.assert(
      fc.property(validDate, (date) => {
        const [hourStr, minuteStr] = formatAtmTime(date).split(":");
        expect(Number.parseInt(hourStr ?? "", 10)).toBe(date.getHours());
        expect(Number.parseInt(minuteStr ?? "", 10)).toBe(date.getMinutes());
      }),
    );
  });

  it("formatAtmTime returns '—' for null", () => {
    expect(formatAtmTime(null)).toBe("—");
  });
});
