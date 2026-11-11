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

describe("admin-vendors URL state: pks_status (vendor-pks Req 1.10)", () => {
  const base = { page: 1, page_size: 25, status: "active" as const, q: "" };

  it("round-trips a valid pks_status", () => {
    const params = { ...base, pks_status: "need_renewal" as const };
    const url = omitDefaults(params);

    expect(url).toEqual({ pks_status: "need_renewal" });
    expect(parseParams(url)).toEqual(params);
  });

  it("drops an unknown pks_status instead of passing it to the API", () => {
    expect(parseParams({ pks_status: "bogus" }).pks_status).toBeUndefined();
    expect(parseParams({ pks_status: 3 }).pks_status).toBeUndefined();
  });

  it("omits pks_status when unset", () => {
    expect(omitDefaults(base)).toEqual({});
  });

  it("the search schema accepts only the five statuses", async () => {
    const { ADMIN_VENDORS_SEARCH_SCHEMA } = await import("./useAdminVendorsUrlState");

    for (const v of ["aktif", "need_renewal", "expired", "terminated", "belum_diisi"]) {
      expect(ADMIN_VENDORS_SEARCH_SCHEMA.safeParse({ pks_status: v }).success).toBe(true);
    }
    expect(ADMIN_VENDORS_SEARCH_SCHEMA.safeParse({ pks_status: "x" }).success).toBe(false);
  });
});
