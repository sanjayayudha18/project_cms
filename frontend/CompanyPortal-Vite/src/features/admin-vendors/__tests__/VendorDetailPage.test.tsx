import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { VendorDetailPage } from "../components/VendorDetailPage";

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children }: { children: React.ReactNode }) => <a href="/x">{children}</a>,
}));
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
  useVendorChildren: (_id: number, kind: string) => ({
    isLoading: false,
    isError: false,
    data:
      kind === "packages"
        ? [{ id: 5, code: "PKG-A", priority_class: "VIP", price: "1500000.00", is_active: false }]
        : [],
  }),
}));

describe("VendorDetailPage", () => {
  it("shows legal name and NPWP on Info, and package rows with a text status", async () => {
    render(<VendorDetailPage vendorId={1} />);
    expect(screen.getByText("PT Vendor Satu")).toBeTruthy();
    expect(screen.getByText("012345678901000")).toBeTruthy();

    await userEvent.click(screen.getByRole("tab", { name: "Paket" }));
    expect(screen.getByText("PKG-A")).toBeTruthy();
    expect(screen.getByText("Nonaktif")).toBeTruthy();
  });
});
