import * as fc from "fast-check";
import { describe, expect, it } from "vitest";
import { formatMonitoring, formatValidity } from "./pks";

/**
 * Feature: vendor-pks
 * Property 3: Format masa berlaku dan monitoring
 * Validates: Requirements 7.2, 7.3, 7.4, 7.5
 */
describe("formatValidity", () => {
  it("0 bulan → '< 1 bulan'", () => {
    expect(formatValidity(0)).toBe("< 1 bulan");
  });

  it("contoh tetap", () => {
    expect(formatValidity(12)).toBe("1 tahun");
    expect(formatValidity(5)).toBe("5 bulan");
    expect(formatValidity(29)).toBe("2 tahun 5 bulan");
  });

  it("property: bagian tahun/bulan sesuai months", () => {
    fc.assert(
      fc.property(fc.integer({ min: 1, max: 1200 }), (months) => {
        const out = formatValidity(months);
        const years = Math.floor(months / 12);
        const rest = months % 12;
        const expected = [years > 0 ? `${years} tahun` : "", rest > 0 ? `${rest} bulan` : ""]
          .filter(Boolean)
          .join(" ");
        expect(out).toBe(expected);
        expect(out).not.toMatch(/(^|\s)0 /);
      }),
    );
  });
});

describe("formatMonitoring", () => {
  it("aktif/need_renewal → 'N hari' (0 di hari expired)", () => {
    fc.assert(
      fc.property(
        fc.constantFrom("aktif" as const, "need_renewal" as const),
        fc.integer({ min: 0, max: 5000 }),
        (s, d) => {
          expect(formatMonitoring(s, d)).toBe(`${d} hari`);
        },
      ),
    );
  });

  it("expired → 'lewat N hari', tidak pernah negatif", () => {
    fc.assert(
      fc.property(fc.integer({ min: -5000, max: -1 }), (d) => {
        const out = formatMonitoring("expired", d);
        expect(out).toBe(`lewat ${-d} hari`);
        expect(out).not.toContain("-");
      }),
    );
  });

  it("terminated, belum_diisi, null → '-'", () => {
    fc.assert(
      fc.property(fc.integer({ min: -5000, max: 5000 }), (d) => {
        expect(formatMonitoring("terminated", d)).toBe("-");
        expect(formatMonitoring("belum_diisi", d)).toBe("-");
        expect(formatMonitoring(null, d)).toBe("-");
      }),
    );
    expect(formatMonitoring("aktif", null)).toBe("-");
  });
});
