import { useAuthStore } from "@/lib/auth/store";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { VendorDetailPage } from "../components/VendorDetailPage";

let searchState: Record<string, unknown> = {};
const navigateMock = vi.fn();

vi.mock("@tanstack/react-router", () => ({
  useSearch: () => searchState,
  useNavigate: () => navigateMock,
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
  usePendingCreates: () => [],
}));

const noopMutation = { mutate: vi.fn(), mutateAsync: vi.fn(), isPending: false };

const VENDOR = {
  id: 1,
  code: "V1",
  name: "Vendor Satu",
  legal_name: "PT Vendor Satu",
  npwp: "012345678901000",
  contact_email: "",
  contact_phone: "",
  hq_address: "",
  is_active: true,
  deleted_at: null,
  kind: "FLM_VENDOR",
  director: null,
};
const defaultVendor = () => ({ isError: false, data: VENDOR });
const useVendorMock = vi.fn(defaultVendor);

const setRole = (role: "ADMIN" | "ATM-SPV") =>
  useAuthStore.setState({
    user: {
      id: 1,
      username: "u",
      fullName: "U",
      email: "u@x",
      role,
      isKaryawan: true,
      vendorId: null,
    },
  });

beforeEach(() => {
  searchState = {};
  navigateMock.mockReset();
  setRole("ADMIN");
});

afterEach(() => {
  useVendorMock.mockImplementation(defaultVendor);
});

vi.mock("../hooks", () => ({
  useVendor: (...args: unknown[]) => useVendorMock(...args),
  usePksTypesList: () => ({ data: [], isLoading: false }),
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
          vault_name: "VLT-A",
          region: "Jakarta",
          category: "ATM",
          currency_code: "IDR",
          alamat: null,
          min_balance: null,
          max_balance: "1000000.00",
          cis_limit_amount: null,
          escrow_cash: null,
          escrow_atm: null,
          operating_hours: "24 jam",
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
  useSetPicRoles: () => noopMutation,
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
  useVendorPackagePrices: () => ({
    isLoading: false,
    isError: false,
    data: { package_prices: [], page: 1, page_size: 100, total: 0 },
  }),
  useCreateVendorPackagePrice: () => noopMutation,
  useUpdateVendorPackagePrice: () => noopMutation,
  useDisableVendorPackagePrice: () => noopMutation,
}));

vi.mock("../../admin-pks/hooks", () => ({
  usePksList: () => ({
    isLoading: false,
    isError: false,
    data: { items: [], total: 0, page: 1, page_size: 100 },
    refetch: vi.fn(),
  }),
}));

describe("VendorDetailPage", () => {
  it("shows legal name and NPWP on Info", () => {
    render(<VendorDetailPage vendorId={1} />);
    expect(screen.getAllByText("PT Vendor Satu").length).toBeGreaterThan(0);
    expect(screen.getByText("012345678901000")).toBeTruthy();
  });

  it("lists branches on the Cabang tab, each linking to its own detail page", () => {
    searchState = { tab: "branches" };
    render(<VendorDetailPage vendorId={1} />);

    expect(screen.getByText("Cabang Satu")).toBeTruthy();

    const branchLink = screen.getByRole("link", { name: "BR1" });
    expect(branchLink.getAttribute("href")).toBe("/settings/admin/vendors/1/branches/10");
  });

  it("shows vendor-wide PIC tab independent of branch selection", () => {
    searchState = { tab: "vendorWidePics" };
    render(<VendorDetailPage vendorId={1} />);
    expect(screen.getByText("Belum ada PIC vendor-wide")).toBeTruthy();
  });

  it("FLM vendor: presents six tabs in order, PKS last", () => {
    render(<VendorDetailPage vendorId={1} />);
    const tabs = screen.getAllByRole("tab").map((t) => t.textContent);
    expect(tabs).toEqual([
      "Info",
      "Cabang",
      "PIC Vendor-wide",
      "Harga Paket",
      "Mapping DSR",
      "PKS",
    ]);
  });

  it("INTERNAL vendor: the PKS tab is hidden, the other five stay in order", () => {
    useVendorMock.mockImplementation(() => ({
      isError: false,
      data: { ...VENDOR, kind: "INTERNAL" },
    }));
    render(<VendorDetailPage vendorId={1} />);
    const tabs = screen.getAllByRole("tab").map((t) => t.textContent);
    expect(tabs).toEqual(["Info", "Cabang", "PIC Vendor-wide", "Harga Paket", "Mapping DSR"]);
  });

  it("opens the read-only PKS tab via ?tab=pks (no write button, link only for ADMIN)", () => {
    searchState = { tab: "pks" };
    render(<VendorDetailPage vendorId={1} />);
    expect(screen.getByText("Vendor ini belum memiliki PKS")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Catat PKS baru" })).toBeNull();
    expect(screen.getByRole("button", { name: "Kelola di Daftar PKS" })).toBeTruthy();
  });

  it("clicking a tab writes it to ?tab=", async () => {
    render(<VendorDetailPage vendorId={1} />);
    await userEvent.click(screen.getByRole("tab", { name: "PKS" }));
    expect(navigateMock).toHaveBeenCalledWith({ to: ".", search: { tab: "pks" } });
  });

  it("ATM-SPV sees only the PKS tab, no summary strip and no manage link (Req 8.4)", () => {
    setRole("ATM-SPV");
    render(<VendorDetailPage vendorId={1} />);
    expect(screen.getAllByRole("tab").map((t) => t.textContent)).toEqual(["PKS"]);
    expect(screen.getByText("Vendor ini belum memiliki PKS")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Kelola di Daftar PKS" })).toBeNull();
    expect(screen.queryByText("012345678901000")).toBeNull();
  });

  it("ATM-SPV cannot reach another tab through ?tab=", () => {
    setRole("ATM-SPV");
    searchState = { tab: "branches" };
    render(<VendorDetailPage vendorId={1} />);
    expect(screen.queryByText("Cabang Satu")).toBeNull();
    expect(screen.getByText("Vendor ini belum memiliki PKS")).toBeTruthy();
  });

  it("shows only the selected tab's panel (Info by default, ?tab= otherwise)", () => {
    const { unmount } = render(<VendorDetailPage vendorId={1} />);
    expect(screen.getByText("012345678901000")).toBeTruthy();
    unmount();

    searchState = { tab: "branches" };
    render(<VendorDetailPage vendorId={1} />);
    expect(screen.getByText("Cabang Satu")).toBeTruthy();
    expect(screen.queryByText("012345678901000")).toBeNull();
  });

  it("shows the vendor-not-found error when the vendor fails to load", () => {
    useVendorMock.mockReturnValueOnce({ isError: true, data: undefined });
    render(<VendorDetailPage vendorId={1} />);
    expect(screen.getByRole("alert")).toHaveTextContent("Vendor tidak ditemukan");
  });
});
