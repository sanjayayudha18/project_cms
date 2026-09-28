import { describe, expect, it } from "vitest";
import type { ATMAssignment } from "../api";
import { currentAssignment } from "./ATMAssignmentsDialog";

const a = (id: number, start: string, end: string | null, isActive = true): ATMAssignment => ({
  id,
  atm_id: 1,
  vendor_package_id: id * 10,
  package_code: `P${id}`,
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
