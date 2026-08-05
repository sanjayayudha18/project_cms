import { describe, expect, it } from "vitest";
import { formatDate, formatDateShort, formatDateTime } from "./date";

describe("formatDate", () => {
  it("formats a date in Indonesian locale with long month", () => {
    const date = new Date(2024, 0, 2); // 2 January 2024
    expect(formatDate(date)).toBe("2 Januari 2024");
  });

  it("accepts custom options", () => {
    const date = new Date(2024, 5, 15); // 15 June 2024
    const result = formatDate(date, { day: "numeric", month: "short", year: "numeric" });
    expect(result).toContain("2024");
    expect(result).toContain("15");
  });
});

describe("formatDateShort", () => {
  it("formats as DD/MM/YYYY", () => {
    const date = new Date(2024, 0, 2); // 2 January 2024
    expect(formatDateShort(date)).toBe("02/01/2024");
  });
});

describe("formatDateTime", () => {
  it("includes date and time components", () => {
    const date = new Date(2024, 0, 2, 14, 30); // 2 Jan 2024, 14:30
    const result = formatDateTime(date);
    expect(result).toContain("Januari");
    expect(result).toContain("2024");
    expect(result).toContain("14");
    expect(result).toContain("30");
  });
});
