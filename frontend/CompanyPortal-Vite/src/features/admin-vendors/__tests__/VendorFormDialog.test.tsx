import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { VendorFormDialog } from "../components/VendorFormDialog";
import type { AdminVendor } from "../types";

const createMutateAsync = vi.fn();
const updateMutateAsync = vi.fn();

vi.mock("../hooks", () => ({
  useCreateVendor: () => ({ mutateAsync: createMutateAsync, isPending: false }),
  useUpdateVendor: () => ({ mutateAsync: updateMutateAsync, isPending: false }),
}));

const VENDOR: AdminVendor = {
  id: 3,
  code: "VDR003",
  name: "PT Vendor Sejahtera",
  contact_email: "vendor@example.com",
  contact_phone: "021-555-0100",
  hq_address: "Jl. Sudirman No. 1",
  is_active: true,
  deleted_at: null,
};

beforeEach(() => {
  createMutateAsync.mockReset();
  updateMutateAsync.mockReset();
});

describe("VendorFormDialog — Kode immutability (Task 9)", () => {
  it("leaves Kode editable in create mode", () => {
    render(<VendorFormDialog open onClose={vi.fn()} vendor={null} />);

    expect(screen.getByText("Kode").nextElementSibling).not.toBeDisabled();
  });

  it("disables Kode in edit mode", () => {
    render(<VendorFormDialog open onClose={vi.fn()} vendor={VENDOR} />);

    expect(screen.getByDisplayValue(VENDOR.code)).toBeDisabled();
  });
});
