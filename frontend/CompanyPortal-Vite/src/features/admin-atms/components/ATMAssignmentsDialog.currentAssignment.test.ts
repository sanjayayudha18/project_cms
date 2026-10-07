import { describe, expect, it } from "vitest";
import type { ATMAssignment } from "../api";
import { currentAssignment, isSameAsCurrent } from "./ATMAssignmentsDialog";

const a = (id: number, start: string, end: string | null, isActive = true): ATMAssignment => ({
  id,
  atm_id: 1,
  vendor_package_id: id * 10,
  package_code: `P${id}`,
  source: "branch",
  effective_start_date: start,
  effective_end_date: end,
  is_active: isActive,
});

describe("currentAssignment", () => {
  it("picks the active period covering today", () => {
    const list = [a(2, "2026-10-01", null), a(1, "2026-09-01", "2026-09-30")];
    expect(currentAssignment(list, "2026-09-25")?.id).toBe(1);
  });
  it("falls back to the most recent active period", () => {
    const list = [a(2, "2026-10-01", null), a(1, "2026-01-01", "2026-02-01")];
    expect(currentAssignment(list, "2026-09-25")?.id).toBe(2);
  });
  it("ignores inactive periods and returns undefined when none remain", () => {
    expect(currentAssignment([a(1, "2026-09-01", null, false)], "2026-09-25")).toBeUndefined();
    expect(currentAssignment([], "2026-09-25")).toBeUndefined();
  });
});

describe("isSameAsCurrent", () => {
  const branchCur = a(1, "2026-01-01", null);
  const vendorCur: ATMAssignment = {
    ...a(2, "2026-01-01", null),
    vendor_package_id: null,
    package_code: "PAKET 4",
    source: "vendor",
    vendor_branch_id: 5,
  };

  it("is false without a running period", () => {
    expect(isSameAsCurrent(undefined, "branch", "10", "5")).toBe(false);
  });
  it("branch: same package id = no change, another = change", () => {
    expect(isSameAsCurrent(branchCur, "branch", "10", "5")).toBe(true);
    expect(isSameAsCurrent(branchCur, "branch", "20", "5")).toBe(false);
  });
  it("vendor-wide: same label and same cabang = no change; another cabang or label = change", () => {
    expect(isSameAsCurrent(vendorCur, "vendor", "PAKET 4", "5")).toBe(true);
    expect(isSameAsCurrent(vendorCur, "vendor", "PAKET 4", "6")).toBe(false);
    expect(isSameAsCurrent(vendorCur, "vendor", "PAKET 5", "5")).toBe(false);
  });
  it("switching source is always a change, even with an equal-looking value", () => {
    expect(isSameAsCurrent(branchCur, "vendor", "10", "5")).toBe(false);
    expect(isSameAsCurrent(vendorCur, "branch", "PAKET 4", "5")).toBe(false);
  });
});
