import { describe, expect, it } from "vitest";
import { omitDefaults, parseParams } from "./useAdminUsersUrlState";

describe("admin-users URL state round-trip", () => {
  it("parses raw search params with defaults filled in", () => {
    expect(parseParams({})).toEqual({
      page: 1,
      page_size: 25,
      status: "active",
      vendor_id: undefined,
      role: undefined,
      q: "",
    });
  });

  it("round-trips a full set of filters through omitDefaults -> parseParams", () => {
    const params = {
      page: 3,
      page_size: 50,
      status: "disabled" as const,
      vendor_id: 7,
      role: "APPACCESS",
      q: "budi",
    };
    const url = omitDefaults(params);
    expect(parseParams(url)).toEqual(params);
  });

  it("omits fields that equal their default, keeping shareable URLs clean", () => {
    const url = omitDefaults({
      page: 1,
      page_size: 25,
      status: "active",
      vendor_id: undefined,
      role: undefined,
      q: "",
    });
    expect(url).toEqual({});
  });
});
