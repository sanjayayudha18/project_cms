import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { AdminVendor } from "../types";
import { InfoTab } from "./InfoTab";

const vendor: AdminVendor = {
  id: 1,
  code: "V1",
  name: "Vendor Satu",
  legal_name: "PT Vendor Satu",
  npwp: "",
  contact_email: "vendor@example.com",
  contact_phone: "",
  hq_address: "",
  is_active: true,
  deleted_at: null,
};

describe("InfoTab", () => {
  it("renders the vendor's profile fields", () => {
    render(<InfoTab vendor={vendor} />);

    expect(screen.getByText("V1")).toBeTruthy();
    expect(screen.getByText("Vendor Satu")).toBeTruthy();
    expect(screen.getByText("PT Vendor Satu")).toBeTruthy();
    expect(screen.getByText("vendor@example.com")).toBeTruthy();
  });

  it("renders a placeholder marker instead of an empty value for a blank field", () => {
    render(<InfoTab vendor={vendor} />);

    // npwp, contact_phone, hq_address are all "" on the fixture.
    expect(screen.getAllByText("—").length).toBeGreaterThanOrEqual(3);
  });

  it("renders the active status as a badge with an icon and a text label", () => {
    render(<InfoTab vendor={vendor} />);

    const badge = screen.getByText("Aktif");
    expect(badge.parentElement?.querySelector("svg")).toBeTruthy();
  });

  it("renders a danger badge with icon and label for an inactive vendor", () => {
    render(<InfoTab vendor={{ ...vendor, is_active: false }} />);

    const badge = screen.getByText("Nonaktif");
    expect(badge.parentElement?.querySelector("svg")).toBeTruthy();
  });

  it("exposes no edit control on this tab", () => {
    render(<InfoTab vendor={vendor} />);

    expect(screen.queryByRole("button")).toBeNull();
    expect(screen.queryByRole("link")).toBeNull();
  });
});
