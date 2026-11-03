import { cleanup, render, screen, within } from "@testing-library/react";
import fc from "fast-check";
import { afterEach, describe, expect, it, vi } from "vitest";

const hooks = vi.hoisted(() => ({ useVendorsList: vi.fn() }));
vi.mock("@/features/admin-vendors/hooks", () => ({
  useVendorsList: hooks.useVendorsList,
  useVendorBranches: () => ({ data: { branches: [] } }),
}));

import { VendorSelect } from "./exportFormParts";

const vendorArb = fc.record({
  id: fc.integer({ min: 1, max: 1_000_000 }),
  name: fc.stringMatching(/^[A-Za-z0-9]{1,12}$/),
  code: fc.stringMatching(/^[A-Z0-9]{1,6}$/),
});

afterEach(() => {
  cleanup();
  hooks.useVendorsList.mockReset();
});

describe("VendorSelect", () => {
  // Feature: export-preview-simplify, Property 2
  it("renders one 'Vendor' dropdown with exactly the endpoint's vendors and never sends q", () => {
    fc.assert(
      fc.property(
        fc.uniqueArray(vendorArb, { selector: (v) => v.id, maxLength: 100 }),
        (vendors) => {
          hooks.useVendorsList.mockReset();
          hooks.useVendorsList.mockReturnValue({ data: { vendors } });
          const { unmount } = render(<VendorSelect value={null} onChange={() => {}} />);

          const combos = screen.getAllByRole("combobox");
          expect(combos).toHaveLength(1);
          expect(screen.getByLabelText("Vendor")).toBe(combos[0]);
          expect(screen.queryByRole("searchbox")).not.toBeInTheDocument();

          const labels = within(combos[0] as HTMLElement)
            .getAllByRole("option")
            .map((o) => o.textContent);
          expect(labels).toEqual(["Pilih vendor", ...vendors.map((v) => `${v.name} (${v.code})`)]);

          for (const call of hooks.useVendorsList.mock.calls) {
            expect(call[0]).not.toHaveProperty("q");
          }
          unmount();
          cleanup();
        },
      ),
      { numRuns: 50 },
    );
  });

  it("shows only the placeholder while the vendor list is not loaded", () => {
    hooks.useVendorsList.mockReturnValue({ data: undefined });
    render(<VendorSelect value={null} onChange={() => {}} />);
    expect(screen.getAllByRole("option")).toHaveLength(1);
  });
});
