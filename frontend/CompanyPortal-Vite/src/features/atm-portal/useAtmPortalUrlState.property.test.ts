import * as fc from "fast-check";
import { describe, expect, it } from "vitest";
import type { AtmPortalParams } from "./types";
import { omitDefaults, parseSearchParams } from "./useAtmPortalUrlState";

/**
 * Feature: atm-portal
 * Property 11: URL-filter synchronization (round-trip)
 * Validates: Requirements 5.4
 *
 * For any valid filter state object, serializing it to URL search params
 * (omitDefaults) and then parsing those URL search params back into a
 * filter state object (parseSearchParams) SHALL produce a value equal to
 * the original. The URL encoding is lossless.
 *
 * Generators are constrained to the real valid domain — the same oneof
 * enums the backend validates (status, sort_by, sort_order) — rather than
 * arbitrary strings. An empty string for status/sort_by/sort_order is not
 * a value the app can ever actually produce (validated server-side to a
 * fixed enum), and the URL encoding necessarily treats "" as "absent, use
 * default" for those fields since it's also their sentinel for "not in the
 * URL" — testing that never-real edge case would be a false failure, not a
 * real bug. search/machine_type/brand/deployment_type/region are free
 * text where "" always and correctly means "no filter", so they're
 * generated including the empty string.
 */
describe("parseSearchParams / omitDefaults — Property 11: URL-filter round-trip", () => {
  const validParams: fc.Arbitrary<AtmPortalParams> = fc
    .constantFrom("replenish" as const, "cashpos" as const)
    .chain((mode) =>
      fc.record({
        mode: fc.constant(mode),
        page: fc.integer({ min: 1, max: 9999 }),
        page_size: fc.integer({ min: 1, max: 100 }),
        search: fc.string({ maxLength: 100 }),
        status: fc.constantFrom("all", "low", "critical", "normal", "unconfigured", "no_data"),
        machine_type: fc.string({ maxLength: 30 }),
        brand: fc.string({ maxLength: 30 }),
        deployment_type: fc.string({ maxLength: 30 }),
        region: fc.string({ maxLength: 30 }),
        date_from: fc.constantFrom("", "2024-01-01", "2026-08-01"),
        date_to: fc.constantFrom("", "2024-12-31", "2026-08-21"),
        sort_by:
          mode === "cashpos"
            ? fc.constantFrom(
                "cashpos_date",
                "terminal_id",
                "machine_type",
                "branch_code",
                "created_at",
                "id",
              )
            : fc.constantFrom(
                "terminal_id",
                "location",
                "last_replenish_date",
                "refund_total",
                "replenish_total",
                "status",
              ),
        sort_order: fc.constantFrom("asc" as const, "desc" as const),
      }),
    );

  it("round-trips: parseSearchParams(omitDefaults(params)) === params", () => {
    fc.assert(
      fc.property(validParams, (params) => {
        const roundTripped = parseSearchParams(omitDefaults(params));
        expect(roundTripped).toEqual(params);
      }),
    );
  });

  it("omitDefaults never includes a key whose value equals the default", () => {
    fc.assert(
      fc.property(validParams, (params) => {
        const serialized = omitDefaults(params);
        if (params.mode === "replenish") expect(serialized).not.toHaveProperty("mode");
        if (params.page === 1) expect(serialized).not.toHaveProperty("page");
        if (params.page_size === 25) expect(serialized).not.toHaveProperty("page_size");
        if (params.search === "") expect(serialized).not.toHaveProperty("search");
        if (params.status === "all") expect(serialized).not.toHaveProperty("status");
        const defaultSortBy = params.mode === "cashpos" ? "cashpos_date" : "terminal_id";
        const defaultSortOrder = params.mode === "cashpos" ? "desc" : "asc";
        if (params.sort_by === defaultSortBy) expect(serialized).not.toHaveProperty("sort_by");
        if (params.sort_order === defaultSortOrder)
          expect(serialized).not.toHaveProperty("sort_order");
      }),
    );
  });

  it("the all-defaults params object serializes to an empty object", () => {
    const defaults: AtmPortalParams = {
      mode: "replenish",
      page: 1,
      page_size: 25,
      search: "",
      status: "all",
      machine_type: "",
      brand: "",
      deployment_type: "",
      region: "",
      date_from: "",
      date_to: "",
      sort_by: "terminal_id",
      sort_order: "asc",
    };
    expect(omitDefaults(defaults)).toEqual({});
  });

  it("parseSearchParams({}) reconstructs the all-defaults params object", () => {
    expect(parseSearchParams({})).toEqual({
      mode: "replenish",
      page: 1,
      page_size: 25,
      search: "",
      status: "all",
      machine_type: "",
      brand: "",
      deployment_type: "",
      region: "",
      date_from: "",
      date_to: "",
      sort_by: "terminal_id",
      sort_order: "asc",
    });
  });

  it("invalid mode falls back to replenish", () => {
    expect(parseSearchParams({ mode: "nope" }).mode).toBe("replenish");
  });

  it("cashpos mode defaults sort to cashpos_date desc", () => {
    expect(parseSearchParams({ mode: "cashpos" })).toMatchObject({
      mode: "cashpos",
      sort_by: "cashpos_date",
      sort_order: "desc",
    });
  });
});
