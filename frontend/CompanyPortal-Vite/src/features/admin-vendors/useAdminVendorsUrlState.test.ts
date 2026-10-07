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
