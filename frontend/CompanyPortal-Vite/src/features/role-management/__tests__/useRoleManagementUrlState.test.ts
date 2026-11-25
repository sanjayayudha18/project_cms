import { describe, expect, it } from "vitest";
import { parseParams, toSearch } from "../useRoleManagementUrlState";

describe("role management URL state", () => {
  it("falls back to Portal Utama for a missing or invalid portal", () => {
    expect(parseParams({}).portal).toBe("main");
    expect(parseParams({ portal: "bogus" }).portal).toBe("main");
    expect(parseParams({ portal: ["vendor"] }).portal).toBe("main");
  });

  it("reads portal, role and q from the URL", () => {
    expect(parseParams({ portal: "vendor", role: "7", q: "abc" })).toEqual({
      portal: "vendor",
      role: 7,
      q: "abc",
    });
  });

  it("ignores an invalid role id", () => {
    expect(parseParams({ role: "x" }).role).toBeNull();
    expect(parseParams({ role: -1 }).role).toBeNull();
    expect(parseParams({ role: 1.5 }).role).toBeNull();
  });

  it("writes the tab to the URL and omits defaults", () => {
    expect(toSearch({ portal: "vendor", role: 3, q: "x" })).toEqual({
      portal: "vendor",
      role: 3,
      q: "x",
    });
    expect(toSearch({ portal: "main", role: null, q: "" })).toEqual({});
  });
});
