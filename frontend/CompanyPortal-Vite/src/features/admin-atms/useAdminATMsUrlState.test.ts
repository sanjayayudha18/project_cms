import { describe, expect, it } from "vitest";
import { omitDefaults, parseParams } from "./useAdminATMsUrlState";

describe("admin-atms URL state round-trip", () => {
  it("parses raw search params with defaults filled in", () => {
    expect(parseParams({})).toEqual({
      page: 1,
      page_size: 25,
      status: "active",
      q: "",
      brand: "",
      machine_type: "",
      deployment_type: "",
      priority_class: "",
    });
  });

  it("round-trips a full set of filters through omitDefaults -> parseParams", () => {
    const params = {
      page: 3,
      page_size: 50,
      status: "disabled" as const,
      q: "TATM",
      brand: "NCR",
      machine_type: "ATM",
      deployment_type: "ONSITE",
      priority_class: "VIP" as const,
    };
    const url = omitDefaults(params);
    expect(parseParams(url)).toEqual(params);
  });

  it("omits fields that equal their default, keeping shareable URLs clean", () => {
    const url = omitDefaults({
      page: 1,
      page_size: 25,
      status: "active",
      q: "",
      brand: "",
      machine_type: "",
      deployment_type: "",
      priority_class: "",
    });
    expect(url).toEqual({});
  });
});
