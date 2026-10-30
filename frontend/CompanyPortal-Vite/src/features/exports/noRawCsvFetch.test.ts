import { describe, expect, it } from "vitest";

// Req 8.5 / 7.x: report screens must export via the async export flow, never a raw fetch of CSV.
const sources = import.meta.glob(
  [
    "../dsr-late-report/**/*.{ts,tsx}",
    "../replenishment/**/*.{ts,tsx}",
    "!../**/__tests__/**",
    "!../**/*.test.{ts,tsx}",
  ],
  { query: "?raw", import: "default", eager: true },
) as Record<string, string>;

describe("report screens have no raw fetch for CSV", () => {
  it("scans the screen sources", () => {
    expect(Object.keys(sources).length).toBeGreaterThan(4);
  });

  it.each(Object.entries(sources))("%s", (_path, src) => {
    expect(src).not.toMatch(/downloadReport/);
    expect(src).not.toMatch(/format=csv/);
    expect(src).not.toMatch(/\bfetch\s*\(/);
  });
});
