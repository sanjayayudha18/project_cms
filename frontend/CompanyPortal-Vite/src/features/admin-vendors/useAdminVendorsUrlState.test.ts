import { describe, expect, it } from "vitest";
import { omitDefaults, parseParams } from "./useAdminVendorsUrlState";

describe("admin-vendors URL state round-trip", () => {
  it("parses raw search params with defaults filled in", () => {
    expect(parseParams({})).toEqual({
      page: 1,
      page_size: 25,
      status: "active",
      q: "",
    });
  });

  it("round-trips a full set of filters through omitDefaults -> parseParams", () => {
    const params = {
      page: 3,
      page_size: 50,
      status: "disabled" as const,
      q: "cimb cit",
    };
    const url = omitDefaults(params);
    expect(parseParams(url)).toEqual(params);
  });

  it("omits fields that equal their default, keeping shareable URLs clean", () => {
    const url = omitDefaults({ page: 1, page_size: 25, status: "active", q: "" });
    expect(url).toEqual({});
  });
});

describe("admin-vendors URL state: kind (settings-menu-review Req 2.6/2.7)", () => {
  const base = { page: 1, page_size: 25, status: "active" as const, q: "" };

  it("round-trips a valid kind", () => {
    const params = { ...base, kind: "INTERNAL" as const };
    const url = omitDefaults(params);
    expect(url).toEqual({ kind: "INTERNAL" });
    expect(parseParams(url)).toEqual(params);
  });

  it("drops an invalid kind", () => {
    expect(parseParams({ kind: "bogus" }).kind).toBeUndefined();
    expect(parseParams({ kind: 3 }).kind).toBeUndefined();
  });

  it("the search schema accepts only the two kinds", async () => {
    const { ADMIN_VENDORS_SEARCH_SCHEMA } = await import("./useAdminVendorsUrlState");
    for (const v of ["FLM_VENDOR", "INTERNAL"]) {
      expect(ADMIN_VENDORS_SEARCH_SCHEMA.safeParse({ kind: v }).success).toBe(true);
    }
    expect(ADMIN_VENDORS_SEARCH_SCHEMA.safeParse({ kind: "x" }).success).toBe(false);
  });
});

describe("admin-vendors URL state: pks_type + pks_status pair (vendor-pks-type Req 5.3)", () => {
  const base = { page: 1, page_size: 25, status: "active" as const, q: "" };

  it("round-trips a valid pks_type + pks_status pair", () => {
    const params = { ...base, pks_type: "FLM", pks_status: "need_renewal" as const };
    const url = omitDefaults(params);

    expect(url).toEqual({ pks_type: "FLM", pks_status: "need_renewal" });
    expect(parseParams(url)).toEqual(params);
  });

  it("drops orphaned pks_type when pks_status is missing", () => {
    const parsed = parseParams({ pks_type: "FLM" });
    expect(parsed.pks_type).toBeUndefined();
    expect(parsed.pks_status).toBeUndefined();
  });

  it("drops orphaned pks_status when pks_type is missing", () => {
    const parsed = parseParams({ pks_status: "aktif" });
    expect(parsed.pks_type).toBeUndefined();
    expect(parsed.pks_status).toBeUndefined();
  });

  it("drops an unknown pks_status instead of passing it to the API", () => {
    expect(parseParams({ pks_type: "FLM", pks_status: "bogus" }).pks_status).toBeUndefined();
    expect(parseParams({ pks_type: "FLM", pks_status: 3 }).pks_status).toBeUndefined();
  });

  it("omits pks_type and pks_status when unset or partial", () => {
    expect(omitDefaults(base)).toEqual({});
    expect(omitDefaults({ ...base, pks_type: "FLM" })).toEqual({});
    expect(omitDefaults({ ...base, pks_status: "aktif" })).toEqual({});
  });

  it("the search schema requires both or neither", async () => {
    const { ADMIN_VENDORS_SEARCH_SCHEMA } = await import("./useAdminVendorsUrlState");

    expect(
      ADMIN_VENDORS_SEARCH_SCHEMA.safeParse({ pks_type: "FLM", pks_status: "aktif" }).success,
    ).toBe(true);
    expect(ADMIN_VENDORS_SEARCH_SCHEMA.safeParse({}).success).toBe(true);
    expect(ADMIN_VENDORS_SEARCH_SCHEMA.safeParse({ pks_type: "FLM" }).success).toBe(false);
    expect(ADMIN_VENDORS_SEARCH_SCHEMA.safeParse({ pks_status: "aktif" }).success).toBe(false);
    expect(
      ADMIN_VENDORS_SEARCH_SCHEMA.safeParse({ pks_type: "FLM", pks_status: "invalid" }).success,
    ).toBe(false);
  });
});
