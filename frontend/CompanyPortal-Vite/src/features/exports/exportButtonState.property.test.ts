import fc from "fast-check";
import { describe, it } from "vitest";
import { MAX_EXPORT_ROWS, exportButtonState } from "./exportButtonState";

// Feature: export-menu-revamp, Property 9: Status tombol ekspor
describe("exportButtonState property", () => {
  const input = fc.record({
    dirty: fc.boolean(),
    loading: fc.boolean(),
    total: fc.oneof(
      fc.integer({ min: -5, max: 5 }),
      fc.integer({ min: 0, max: 2_000_000 }),
      fc.constantFrom(0, 1, MAX_EXPORT_ROWS, MAX_EXPORT_ROWS + 1),
    ),
  });

  it("enabled iff !dirty && !loading && 0 < total <= 1_000_000", () => {
    fc.assert(
      fc.property(input, (i) => {
        const expected = !i.dirty && !i.loading && i.total > 0 && i.total <= 1_000_000;
        return exportButtonState(i).enabled === expected;
      }),
    );
  });

  it("dirty always shows the Search message", () => {
    fc.assert(
      fc.property(input, (i) => {
        const s = exportButtonState(i);
        return !i.dirty || s.message === "Klik Search untuk memperbarui tabel sebelum mengekspor.";
      }),
    );
  });

  it("enabled has no message; disabled by dirty or the row limit always has one", () => {
    fc.assert(
      fc.property(input, (i) => {
        const s = exportButtonState(i);
        if (s.enabled) return s.message === null;
        const mustExplain = i.dirty || i.total > MAX_EXPORT_ROWS;
        return !mustExplain || (s.message !== null && s.message.length > 0);
      }),
    );
  });
});
