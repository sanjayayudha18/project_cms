import { describe, expect, it } from "vitest";
import { nextBusinessDayISO } from "./nextBusinessDay";

describe("nextBusinessDayISO", () => {
  it("returns the next day when it's a weekday", () => {
    // Tuesday 2026-09-08 -> Wednesday 2026-09-09
    expect(nextBusinessDayISO(new Date(2026, 8, 8))).toBe("2026-09-09");
  });

  it("skips Saturday, landing on Monday", () => {
    // Friday 2026-09-11 -> next day is Saturday, skip to Monday 2026-09-14
    expect(nextBusinessDayISO(new Date(2026, 8, 11))).toBe("2026-09-14");
  });

  it("skips Sunday, landing on Monday", () => {
    // Saturday 2026-09-12 -> next day is Sunday, skip to Monday 2026-09-14
    expect(nextBusinessDayISO(new Date(2026, 8, 12))).toBe("2026-09-14");
  });

  it("Sunday -> Monday (single skip)", () => {
    // Sunday 2026-09-13 -> Monday 2026-09-14
    expect(nextBusinessDayISO(new Date(2026, 8, 13))).toBe("2026-09-14");
  });
});
