import { describe, expect, it } from "vitest";
import { pageNumbers } from "./PageNumbers";

describe("pageNumbers", () => {
  it("lists every page when there are few", () => {
    expect(pageNumbers(1, 3)).toEqual([1, 2, 3]);
  });
  it("collapses distant pages into ellipses", () => {
    expect(pageNumbers(5, 12)).toEqual([1, null, 4, 5, 6, null, 12]);
  });
  it("fills a single-page gap with the number instead of an ellipsis", () => {
    expect(pageNumbers(4, 12)).toEqual([1, 2, 3, 4, 5, null, 12]);
  });
  it("handles first and last page", () => {
    expect(pageNumbers(1, 12)).toEqual([1, 2, null, 12]);
    expect(pageNumbers(12, 12)).toEqual([1, null, 11, 12]);
  });
});
