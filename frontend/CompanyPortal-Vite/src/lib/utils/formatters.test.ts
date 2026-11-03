import { describe, expect, it } from "vitest";
import {
  formatBadgeCount,
  formatDifference,
  formatFullDate,
  formatIDRAbbreviated,
  formatIDRFull,
  getGreeting,
  getInitials,
  progressPercent,
} from "./formatters";

// fe-coverage C5: shared display helpers (money always shown with "IDR", Sec 13).
describe("money display", () => {
  it.each([
    [12_800_000_000_000, "IDR 12.8T"],
    [-1_500_000_000_000, "IDR -1.5T"],
    [2_450_000_000, "IDR 2.5B"],
    [999_000_000, "IDR 999.0M"],
    [1_000_000, "IDR 1.0M"],
    [999_999, "IDR 999999"],
    [0, "IDR 0"],
  ])("formatIDRAbbreviated(%d) = %s", (value, expected) => {
    expect(formatIDRAbbreviated(value)).toBe(expected);
  });

  it("formatIDRFull groups thousands with dots, no decimals", () => {
    expect(formatIDRFull(12_800_000_000)).toBe("IDR 12.800.000.000");
    expect(formatIDRFull(0)).toBe("IDR 0");
  });

  it.each([
    [0, { text: "IDR 0", colorClass: "" }],
    [1_250_000, { text: "+ IDR 1.250.000", colorClass: "text-success" }],
    [-300_000, { text: "- IDR 300.000", colorClass: "text-danger" }],
  ])("formatDifference(%d) carries a sign, not only a colour", (value, expected) => {
    expect(formatDifference(value)).toEqual(expected);
  });
});

describe("other helpers", () => {
  it.each([
    [0, "Good morning"],
    [11, "Good morning"],
    [12, "Good afternoon"],
    [17, "Good afternoon"],
    [18, "Good evening"],
    [23, "Good evening"],
    [-1, "Good evening"],
  ])("getGreeting(%i) = %s", (hour, expected) => {
    expect(getGreeting(hour)).toBe(expected);
  });

  it("formatFullDate spells out weekday and month", () => {
    expect(formatFullDate(new Date(2026, 9, 15))).toBe("Thursday, 15 October 2026");
  });

  it.each([
    [0, 0, 0],
    [1, 3, 33],
    [2, 3, 67],
    [5, 5, 100],
  ])("progressPercent(%i, %i) = %i", (done, total, expected) => {
    expect(progressPercent(done, total)).toBe(expected);
  });

  it.each([
    [0, null],
    [-2, null],
    [1, "1"],
    [99, "99"],
    [100, "99+"],
  ])("formatBadgeCount(%i) = %j", (count, expected) => {
    expect(formatBadgeCount(count)).toBe(expected);
  });

  it.each([
    ["", ""],
    ["   ", ""],
    ["budi", "B"],
    ["budi santoso", "BS"],
    ["  siti  nur  aisyah ", "SA"],
  ])("getInitials(%j) = %j", (name, expected) => {
    expect(getInitials(name)).toBe(expected);
  });
});
