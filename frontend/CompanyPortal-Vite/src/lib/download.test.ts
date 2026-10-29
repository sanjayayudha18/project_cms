import { afterEach, describe, expect, it, vi } from "vitest";
import { saveBlobResponse } from "./download";

function run(disposition: string | null) {
  const createObjectURL = vi.fn(() => "blob:fake");
  const revokeObjectURL = vi.fn();
  vi.stubGlobal("URL", Object.assign(URL, { createObjectURL, revokeObjectURL }));
  const clicked: string[] = [];
  vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(function (
    this: HTMLAnchorElement,
  ) {
    clicked.push(this.download);
  });
  const headers = new Headers(disposition ? { "Content-Disposition": disposition } : {});
  saveBlobResponse({ data: new Blob(["x"]), headers }, "fallback.csv");
  return { clicked, revokeObjectURL };
}

describe("saveBlobResponse", () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("uses the Content-Disposition filename and revokes the object URL", () => {
    const { clicked, revokeObjectURL } = run('attachment; filename="vendors.csv"');
    expect(clicked).toEqual(["vendors.csv"]);
    expect(revokeObjectURL).toHaveBeenCalledWith("blob:fake");
  });

  it("falls back when there is no Content-Disposition", () => {
    expect(run(null).clicked).toEqual(["fallback.csv"]);
  });
});
