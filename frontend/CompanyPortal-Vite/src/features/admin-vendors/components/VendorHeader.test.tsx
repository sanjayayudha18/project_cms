import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { AdminVendor } from "../types";
import { VendorHeader } from "./VendorHeader";

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, to }: { children: React.ReactNode; to: string }) => (
    <a href={to}>{children}</a>
  ),
}));

vi.mock("../hooks", () => ({
  useVendorBranches: () => ({ isLoading: true, isError: false, data: undefined }),
  useVendorPics: () => ({ isLoading: true, isError: false, data: undefined }),
  useVendorPackagePrices: () => ({ isLoading: true, isError: false, data: undefined }),
}));

const vendor: AdminVendor = {
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
};

describe("VendorHeader", () => {
  it("renders title, subtitle, and an active status badge with icon and label", () => {
    render(<VendorHeader vendor={vendor} vendorId={1} />);

    expect(screen.getByText("V1 · Vendor Satu")).toBeTruthy();
    expect(screen.getByText("PT Vendor Satu")).toBeTruthy();

    const badge = screen.getByText("Aktif");
    expect(badge.parentElement?.querySelector("svg")).toBeTruthy();
  });

  it("renders a danger badge for an inactive vendor", () => {
    render(<VendorHeader vendor={{ ...vendor, is_active: false }} vendorId={1} />);

    const badge = screen.getByText("Nonaktif");
    expect(badge.parentElement?.querySelector("svg")).toBeTruthy();
  });

  it("renders a non-error placeholder title while the vendor is still loading", () => {
    render(<VendorHeader vendor={undefined} vendorId={1} />);

    expect(screen.getByText("Detail Vendor")).toBeTruthy();
    expect(screen.queryByText("Aktif")).toBeNull();
    expect(screen.queryByText("Nonaktif")).toBeNull();
  });
});
