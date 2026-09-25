import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { AdminVendorBranch, AdminVendorPackagePrice, AdminVendorPic } from "../types";
import { SummaryStrip } from "./SummaryStrip";

const useVendorBranchesMock = vi.fn();
const useVendorPicsMock = vi.fn();
const useVendorPackagePricesMock = vi.fn();

vi.mock("../hooks", () => ({
  useVendorBranches: (...args: unknown[]) => useVendorBranchesMock(...args),
  useVendorPics: (...args: unknown[]) => useVendorPicsMock(...args),
  useVendorPackagePrices: (...args: unknown[]) => useVendorPackagePricesMock(...args),
}));

function daysFromToday(offset: number): string {
  const d = new Date();
  d.setDate(d.getDate() + offset);
  return d.toISOString().slice(0, 10);
}

const branch: AdminVendorBranch = {
  id: 1,
  vendor_id: 1,
  branch_code: "BR1",
  branch_name: "Cabang Satu",
  location_id: null,
  region: "Jakarta",
  category: "ATM",
  is_active: true,
  deleted_at: null,
};

const pic: AdminVendorPic = {
  id: 1,
  vendor_id: 1,
  vendor_branch_id: null,
  name: "Budi",
  position: "Manager",
  email: "budi@example.com",
  phone: "0812",
  is_notification_recipient: true,
  is_active: true,
  deleted_at: null,
};

function packagePrice(overrides: Partial<AdminVendorPackagePrice> = {}): AdminVendorPackagePrice {
  return {
    id: 1,
    vendor_id: 1,
    package: "PAKET 3",
    package_code: "PKG3_ABA_001",
    machine_group: "ATM",
    price_class: "REGULAR",
    tier_min: 1,
    tier_max: 50,
    base_price: "1000000.00",
    vendor_branch_id: null,
    atm_id: null,
    sla_note: null,
    currency: "IDR",
    effective_start_date: daysFromToday(-30),
    effective_end_date: null,
    ...overrides,
  };
}

describe("SummaryStrip", () => {
  it("renders all four metrics once every source has loaded", () => {
    useVendorBranchesMock.mockReturnValue({
      isLoading: false,
      data: {
        branches: [branch, { ...branch, id: 2, is_active: false }],
        page: 1,
        page_size: 100,
        total: 2,
      },
    });
    useVendorPicsMock.mockReturnValue({
      isLoading: false,
      data: { pics: [pic], page: 1, page_size: 100, total: 1, warnings: [] },
    });
    useVendorPackagePricesMock.mockReturnValue({
      isLoading: false,
      data: { package_prices: [packagePrice()], page: 1, page_size: 100, total: 1 },
    });

    render(<SummaryStrip vendorId={1} />);

    expect(screen.getByText("1/2")).toBeTruthy();
    expect(screen.getAllByText("1").length).toBe(2); // PIC count and Paket berlaku count
    expect(screen.getByText("IDR 1.000.000,00 - IDR 1.000.000,00")).toBeTruthy();
  });

  it("shows a neutral placeholder per metric while its own query is still loading, not a zero", () => {
    useVendorBranchesMock.mockReturnValue({ isLoading: true, data: undefined });
    useVendorPicsMock.mockReturnValue({ isLoading: true, data: undefined });
    useVendorPackagePricesMock.mockReturnValue({ isLoading: true, data: undefined });

    render(<SummaryStrip vendorId={1} />);

    const placeholders = screen.getAllByText("—");
    expect(placeholders.length).toBe(4);
    expect(screen.queryByText("0")).toBeNull();
  });

  it("shows a range placeholder when there are no Berlaku package prices", () => {
    useVendorBranchesMock.mockReturnValue({
      isLoading: false,
      data: { branches: [], page: 1, page_size: 100, total: 0 },
    });
    useVendorPicsMock.mockReturnValue({
      isLoading: false,
      data: { pics: [], page: 1, page_size: 100, total: 0, warnings: [] },
    });
    useVendorPackagePricesMock.mockReturnValue({
      isLoading: false,
      data: {
        package_prices: [packagePrice({ effective_start_date: daysFromToday(10) })],
        page: 1,
        page_size: 100,
        total: 1,
      },
    });

    render(<SummaryStrip vendorId={1} />);

    expect(screen.getByText("—")).toBeTruthy();
  });
});
