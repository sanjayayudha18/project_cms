import { todayISO } from "@/features/atm-portal/lib/formatters";
import * as fc from "fast-check";
import { describe, expect, it } from "vitest";
import type { DmaaForecastParams } from "./types";
import { DMAA_FORECAST_SORT_COLUMNS } from "./types";
import { omitDefaults, parseSearchParams } from "./useDmaaForecastUrlState";

/**
 * Feature: dmaa-forecast
 * Property 14: URL state round-trip
 * Validates: Requirements 10.1, 10.2
 *
 * For any valid search state object, encoding it to URL search params
 * (omitDefaults) and parsing those params back (parseSearchParams) SHALL
 * produce the original state. Generators are constrained to the valid
 * domain: sort_by is the backend-allowlisted column set, sort_order is
 * asc/desc, dates are YYYY-MM-DD or empty.
 */
describe("parseSearchParams / omitDefaults — Property 14: URL state round-trip", () => {
  const validParams: fc.Arbitrary<DmaaForecastParams> = fc.record({
    page: fc.integer({ min: 1, max: 9999 }),
    pageSize: fc.integer({ min: 1, max: 100 }),
    dateFrom: fc.constantFrom("", "2026-08-01", "2026-09-15", "2027-01-31"),
    dateTo: fc.constantFrom("", "2026-08-31", "2026-12-31", "2027-02-28"),
    terminalId: fc.string({ maxLength: 100 }),
    sortBy: fc.constantFrom(...DMAA_FORECAST_SORT_COLUMNS),
    sortOrder: fc.constantFrom("asc" as const, "desc" as const),
  });

  it("encode → parse is lossless for any valid state", () => {
    fc.assert(
      fc.property(validParams, (params) => {
        const encoded = omitDefaults(params);
        const parsed = parseSearchParams(encoded as Record<string, unknown>);
        expect(parsed).toEqual(params);
      }),
    );
  });

  it("defaults are applied when the URL is empty", () => {
    expect(parseSearchParams({})).toEqual({
      page: 1,
      pageSize: 25,
      dateFrom: todayISO(),
      dateTo: todayISO(),
      terminalId: "",
      sortBy: "periode_pred",
      sortOrder: "desc",
    });
  });

  it("defaults are omitted from the encoded URL", () => {
    expect(omitDefaults(parseSearchParams({}))).toEqual({});
  });

  it("date filters default to today; cleared dates round-trip as empty strings", () => {
    // Absent from URL → today's date (the "now" default)
    expect(parseSearchParams({}).dateFrom).toBe(todayISO());
    expect(parseSearchParams({}).dateTo).toBe(todayISO());
    // Explicitly cleared ("?dateFrom=") → "" (no filter), preserved in URL
    const cleared = parseSearchParams({ dateFrom: "", dateTo: "" });
    expect(cleared.dateFrom).toBe("");
    expect(cleared.dateTo).toBe("");
    expect(omitDefaults(cleared)).toEqual({ dateFrom: "", dateTo: "" });
  });

  it("invalid sortOrder falls back to desc", () => {
    const parsed = parseSearchParams({ sortOrder: "up" });
    expect(parsed.sortOrder).toBe("desc");
  });
});
