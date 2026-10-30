import { describe, expect, it } from "vitest";
import { MAX_EXPORT_ROWS, exportButtonState } from "./exportButtonState";

describe("exportButtonState", () => {
  it("is enabled for a clean, loaded table within the row limit", () => {
    expect(exportButtonState({ dirty: false, loading: false, total: 1 })).toEqual({
      enabled: true,
      message: null,
    });
    expect(
      exportButtonState({ dirty: false, loading: false, total: MAX_EXPORT_ROWS }).enabled,
    ).toBe(true);
  });

  it("is disabled with a message when the filter is not applied yet", () => {
    const s = exportButtonState({ dirty: true, loading: false, total: 10 });
    expect(s.enabled).toBe(false);
    expect(s.message).toBe("Klik Search untuk memperbarui tabel sebelum mengekspor.");
  });

  it("is disabled while loading or when there are no rows", () => {
    expect(exportButtonState({ dirty: false, loading: true, total: 10 }).enabled).toBe(false);
    expect(exportButtonState({ dirty: false, loading: false, total: 0 }).enabled).toBe(false);
  });

  it("is disabled with a message above one million rows", () => {
    const s = exportButtonState({ dirty: false, loading: false, total: MAX_EXPORT_ROWS + 1 });
    expect(s.enabled).toBe(false);
    expect(s.message).toMatch(/1\.000\.000/);
  });
});
