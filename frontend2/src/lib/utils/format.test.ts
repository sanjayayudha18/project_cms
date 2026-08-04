import { describe, expect, it } from "vitest";
import { formatIDR } from "./format";

describe("formatIDR", () => {
  it("formats zero as Rp0", () => {
    expect(formatIDR(0)).toBe("Rp0");
  });

  it("formats values below 1000 without separator", () => {
    expect(formatIDR(500)).toBe("Rp500");
    expect(formatIDR(999)).toBe("Rp999");
  });

  it("formats 1000 with dot separator", () => {
    expect(formatIDR(1000)).toBe("Rp1.000");
  });

  it("formats millions with dot separators", () => {
    expect(formatIDR(1500000)).toBe("Rp1.500.000");
  });

  it("formats large numbers correctly", () => {
    expect(formatIDR(1000000000)).toBe("Rp1.000.000.000");
  });
});
