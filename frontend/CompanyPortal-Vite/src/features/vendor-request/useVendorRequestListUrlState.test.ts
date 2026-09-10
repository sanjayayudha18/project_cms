import { describe, expect, it } from "vitest";
import { type ListParams, omitDefaults, parseParams } from "./useVendorRequestListUrlState";

describe("parseParams", () => {
  it("falls back to defaults for an empty raw object", () => {
    expect(parseParams({})).toEqual({
      status: [],
      forecastDate: "",
      search: "",
      page: 1,
      pageSize: 10,
    });
  });

  it("reads through valid raw values", () => {
    expect(
      parseParams({
        status: ["draft", "approved"],
        forecastDate: "2026-09-12",
        search: "VR-2026",
        page: 3,
        pageSize: 20,
      }),
    ).toEqual({
      status: ["draft", "approved"],
      forecastDate: "2026-09-12",
      search: "VR-2026",
      page: 3,
      pageSize: 20,
    });
  });

  it("ignores malformed values and falls back to defaults", () => {
    expect(parseParams({ status: "not-an-array", page: "three" })).toEqual({
      status: [],
      forecastDate: "",
      search: "",
      page: 1,
      pageSize: 10,
    });
  });
});

describe("omitDefaults", () => {
  const allDefaults: ListParams = {
    status: [],
    forecastDate: "",
    search: "",
    page: 1,
    pageSize: 10,
  };

  it("returns an empty object when every field is at its default (clean URL)", () => {
    expect(omitDefaults(allDefaults)).toEqual({});
  });

  it("keeps only fields that differ from the default", () => {
    expect(omitDefaults({ ...allDefaults, page: 2 })).toEqual({ page: 2 });
    expect(omitDefaults({ ...allDefaults, status: ["rejected"] })).toEqual({
      status: ["rejected"],
    });
  });

  it("round-trips through parseParams for a non-default state", () => {
    const state: ListParams = {
      status: ["pending_approval"],
      forecastDate: "2026-09-12",
      search: "VR-2026",
      page: 2,
      pageSize: 50,
    };
    expect(parseParams(omitDefaults(state))).toEqual(state);
  });
});
