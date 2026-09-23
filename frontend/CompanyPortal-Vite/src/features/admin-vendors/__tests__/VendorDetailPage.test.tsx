import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { VendorDetailPage } from "../components/VendorDetailPage";

vi.mock("@tanstack/react-router", () => ({
  Link: ({
    children,
    to,
    params,
  }: {
    children: React.ReactNode;
    to: string;
    params?: Record<string, string>;
  }) => {
    const href = params
      ? Object.entries(params).reduce((path, [key, value]) => path.replace(`$${key}`, value), to)
      : to;
    return <a href={href}>{children}</a>;
  },
}));

vi.mock("../../master-data/pending", () => ({
  usePendingEntityIds: () => new Set<number>(),
}));

const noopMutation = { mutate: vi.fn(), mutateAsync: vi.fn(), isPending: false };

vi.mock("../hooks", () => ({
  useVendor: () => ({
    data: {
      id: 1,
      code: "V1",
      name: "Vendor Satu",
      legal_name: "PT Vendor Satu",
      npwp: "012345678901000",
      contact_email: "",
      contact_phone: "",
      hq_address: "",
      is_active: true,
    },
  }),
  useVendorBranches: () => ({
    isLoading: false,
    isError: false,
    data: {
      branches: [
        {
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
      ],
      page: 1,
      page_size: 100,
      total: 1,
    },
  }),
  useCreateVendorBranch: () => noopMutation,
  useUpdateVendorBranch: () => noopMutation,
  useDisableVendorBranch: () => noopMutation,
  useEnableVendorBranch: () => noopMutation,
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
    data: {
      packages: [
        {
          id: 5,
          vendor_branch_id: 10,
          code: "PKG-A",
          is_active: false,
          deleted_at: "2026-01-01T00:00:00Z",
        },
      ],
      page: 1,
      page_size: 100,
      total: 1,
    },
  }),
  useCreateVendorPackage: () => noopMutation,
  useDisableVendorPackage: () => noopMutation,
  useEnableVendorPackage: () => noopMutation,
}));

describe("VendorDetailPage", () => {
  it("shows legal name and NPWP on Info", () => {
    render(<VendorDetailPage vendorId={1} />);
    expect(screen.getByText("PT Vendor Satu")).toBeTruthy();
    expect(screen.getByText("012345678901000")).toBeTruthy();
  });

  it("lists branches on the Cabang tab, each linking to its own detail page", async () => {
    render(<VendorDetailPage vendorId={1} />);

    await userEvent.click(screen.getByRole("tab", { name: "Cabang" }));
    expect(screen.getByText("Cabang Satu")).toBeTruthy();

    const branchLink = screen.getByRole("link", { name: "BR1" });
    expect(branchLink.getAttribute("href")).toBe("/settings/admin/vendors/1/branches/10");
  });

  it("shows vendor-wide PIC tab independent of branch selection", async () => {
    render(<VendorDetailPage vendorId={1} />);
    await userEvent.click(screen.getByRole("tab", { name: "PIC Vendor-wide" }));
    expect(screen.getByText("Belum ada PIC vendor-wide")).toBeTruthy();
  });
});
