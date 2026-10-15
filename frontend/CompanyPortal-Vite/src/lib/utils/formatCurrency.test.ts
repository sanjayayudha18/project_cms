import { describe, expect, it } from "vitest";
import { formatThousands, parseIDR } from "./formatCurrency";

// fe-auth-unify U3: locks the exact output before the rename, so screens show the same strings.
describe("formatCurrency", () => {
  it.each([
    [0, "0"],
    [1000, "1.000"],
    [1250000, "1.250.000"],
    [-1250000, "-1.250.000"],
    [1499.6, "1.500"],
  ])("formats %s as %s (thousands only, no currency, no decimals)", (value, expected) => {
    expect(formatThousands(value)).toBe(expected);
  });

  it("parseIDR round-trips a formatted non-negative integer", () => {
    expect(parseIDR(formatThousands(1250000))).toBe(1250000);
    expect(parseIDR("0")).toBe(0);
  });
});
