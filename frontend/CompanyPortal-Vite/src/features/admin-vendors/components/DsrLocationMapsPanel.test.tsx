import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { DsrLocationMapsPanel } from "./DsrLocationMapsPanel";

const mutate = vi.fn();
vi.mock("../dsrLocationMaps", () => ({
  useDsrLocationMaps: () => ({
    isLoading: false,
    isError: false,
    data: [
      {
        id: 1,
        vendor_id: 3,
        dsr_location: "BINTARO",
        vendor_vault_id: 11,
        vault_code: "V-BTR",
        vendor_branch_id: 5,
        branch_name: "Cabang Bintaro",
        is_active: true,
        deleted_at: null,
      },
    ],
  }),
  useUnmappedDsrLocations: () => ({
    data: [{ dsr_location: "LENTENG AGUNG", last_report_date: "2026-10-07" }],
  }),
  useDsrLocationMapMutation: () => ({ mutate, isPending: false }),
}));
vi.mock("../hooks", () => ({
  useVendorVaults: () => ({
    data: { vaults: [{ id: 12, vault_code: "V-LA", category: "CASH" }] },
  }),
}));
vi.mock("../../master-data/pending", () => ({
  usePendingEntityIds: () => new Set<number>(),
  usePendingCreates: () => [],
}));
vi.mock("@/lib/hooks/useToast", () => ({ useToast: () => ({ toast: vi.fn() }) }));

describe("DsrLocationMapsPanel", () => {
  it("lists mappings and the unmapped DSR labels", () => {
    render(<DsrLocationMapsPanel vendorId={3} />);
    expect(screen.getByText("BINTARO")).toBeTruthy();
    expect(screen.getByText("V-BTR")).toBeTruthy();
    expect(screen.getByText(/Lokasi DSR belum terpetakan \(1\)/)).toBeTruthy();
    expect(screen.getByText("LENTENG AGUNG")).toBeTruthy();
  });

  it("maps an unmapped label: prefilled label + chosen vault -> create submit", () => {
    render(<DsrLocationMapsPanel vendorId={3} />);
    fireEvent.click(screen.getByRole("button", { name: "Petakan" }));
    fireEvent.change(screen.getByLabelText("Vault"), { target: { value: "12" } });
    fireEvent.click(screen.getByRole("button", { name: "Ajukan" }));
    expect(mutate).toHaveBeenCalledWith(
      { op: "create", dsr_location: "LENTENG AGUNG", vendor_vault_id: 12 },
      expect.anything(),
    );
  });

  it("disables via a staged change, not a direct edit", () => {
    render(<DsrLocationMapsPanel vendorId={3} />);
    fireEvent.click(screen.getByRole("button", { name: "Nonaktifkan" }));
    expect(mutate).toHaveBeenCalledWith({ op: "disable", id: 1 }, expect.anything());
  });
});
