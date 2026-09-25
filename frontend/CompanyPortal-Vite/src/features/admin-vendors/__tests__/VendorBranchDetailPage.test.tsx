import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { VendorBranchDetailPage } from "../components/VendorBranchDetailPage";
import { useBranchATMs } from "../hooks";

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children }: { children: React.ReactNode }) => <a href="/x">{children}</a>,
}));

vi.mock("../../master-data/pending", () => ({
  usePendingEntityIds: () => new Set<number>(),
}));

const noopMutation = { mutate: vi.fn(), mutateAsync: vi.fn(), isPending: false };

vi.mock("../hooks", () => ({
  useVendorBranch: () => ({
    isError: false,
    data: {
      id: 10,
      vendor_id: 1,
      branch_code: "BR1",
      branch_name: "Cabang Satu",
      location_id: null,
      region: "Jakarta",
      category: "ATM",
      is_active: true,
      deleted_at: null,
    },
  }),
  useVendorVaults: () => ({
    isLoading: false,
    isError: false,
    data: {
      vaults: [
        {
          id: 20,
          vendor_branch_id: 10,
          vault_code: "VLT-A",
          category: "ATM",
          currency_code: "IDR",
          min_capacity_amount: null,
          max_capacity_amount: "1000000.00",
          latitude: null,
          longitude: null,
          operating_hours: "24 jam",
          location_id: null,
          is_active: true,
          deleted_at: null,
        },
      ],
      page: 1,
      page_size: 100,
      total: 1,
    },
  }),
  useCreateVendorVault: () => noopMutation,
  useUpdateVendorVault: () => noopMutation,
  useDisableVendorVault: () => noopMutation,
  useEnableVendorVault: () => noopMutation,
  useVendorPics: () => ({
    isLoading: false,
    isError: false,
    data: { pics: [], page: 1, page_size: 100, total: 0, warnings: [] },
  }),
  useCreateVendorPic: () => noopMutation,
  useUpdateVendorPic: () => noopMutation,
  useDisableVendorPic: () => noopMutation,
  useEnableVendorPic: () => noopMutation,
  useVendorPackages: () => ({
    isLoading: false,
    isError: false,
    data: { packages: [], page: 1, page_size: 100, total: 0 },
  }),
  useCreateVendorPackage: () => noopMutation,
  useDisableVendorPackage: () => noopMutation,
  useEnableVendorPackage: () => noopMutation,
  useBranchATMs: vi.fn(() => ({
    isLoading: false,
    isError: false,
    data: { atms: [], page: 1, page_size: 100, total: 0 },
  })),
}));

describe("VendorBranchDetailPage", () => {
  it("shows the branch header and its vaults by default", () => {
    render(<VendorBranchDetailPage vendorId={1} branchId={10} />);
    expect(screen.getByText("BR1 · Cabang Satu")).toBeTruthy();
    expect(screen.getByText("VLT-A")).toBeTruthy();
  });

  it("switches to the PIC Cabang sub-tab", async () => {
    render(<VendorBranchDetailPage vendorId={1} branchId={10} />);
    await userEvent.click(screen.getByRole("tab", { name: "PIC Cabang" }));
    expect(screen.getByText("Cabang ini belum punya PIC")).toBeTruthy();
  });

  it("shows an ATM sub-tab alongside the others and fetches on selection", async () => {
    render(<VendorBranchDetailPage vendorId={1} branchId={10} />);
    expect(screen.getByRole("tab", { name: "ATM" })).toBeTruthy();

    await userEvent.click(screen.getByRole("tab", { name: "ATM" }));
    expect(screen.getByText("Cabang ini belum mengelola ATM")).toBeTruthy();
    expect(useBranchATMs).toHaveBeenCalledWith(1, 10, { page: 1, page_size: 100 });
  });
});
