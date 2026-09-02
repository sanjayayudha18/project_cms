import * as fc from "fast-check";
import { describe, expect, it } from "vitest";

describe("Vitest setup smoke test", () => {
  it("runs in jsdom environment", () => {
    expect(document).toBeDefined();
    expect(window).toBeDefined();
  });

  it("fast-check property test works", () => {
    fc.assert(
      fc.property(fc.integer({ min: 0, max: 1000 }), (n) => {
        return n + 1 > n;
      }),
    );
  });

  it("path alias @/ resolves correctly", async () => {
    // Verifies that the @/ alias resolves to src/ by importing the test-setup file path
    // If the alias is broken, this import will fail
    const mod = await import("@/test-setup");
    expect(mod).toBeDefined();
  });
});
