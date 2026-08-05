import { describe, expect, it } from "vitest";

describe("test setup", () => {
  it("should run tests with vitest", () => {
    expect(1 + 1).toBe(2);
  });

  it("should support jsdom environment", () => {
    const div = document.createElement("div");
    div.textContent = "CMS";
    expect(div.textContent).toBe("CMS");
  });
});
