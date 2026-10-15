import { describe, expect, it } from "vitest";
import { formatRp } from "./format";

describe("formatRp", () => {
  it("formats zero as Rp0", () => {
    expect(formatRp(0)).toBe("Rp0");
  });

  it("formats values below 1000 without separator", () => {
    expect(formatRp(500)).toBe("Rp500");
    expect(formatRp(999)).toBe("Rp999");
  });

  it("formats 1000 with dot separator", () => {
    expect(formatRp(1000)).toBe("Rp1.000");
  });

  it("formats millions with dot separators", () => {
    expect(formatRp(1500000)).toBe("Rp1.500.000");
  });

  it("formats large numbers correctly", () => {
    expect(formatRp(1000000000)).toBe("Rp1.000.000.000");
  });
});
