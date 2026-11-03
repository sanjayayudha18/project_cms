/**
 * fe-coverage C3: standalone vendor-branch edit page (maker-checker submit).
 * Router + data hooks mocked; the real form, schema and payload mapping run.
 */

import { useToastStore } from "@/lib/hooks/useToast";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const m = vi.hoisted(() => ({
  navigate: vi.fn(),
  branch: {} as Record<string, unknown>,
  update: vi.fn(),
}));
vi.mock("@tanstack/react-router", () => ({
  Link: ({ children }: { children: ReactNode }) => (
    <a href="/settings/admin/vendors/42">{children}</a>
  ),
  useNavigate: () => m.navigate,
}));
vi.mock("../hooks", () => ({
  useVendorBranch: () => m.branch,
  useUpdateVendorBranch: () => ({ mutateAsync: m.update, isPending: false }),
}));

import { VendorBranchEditPage } from "../components/VendorBranchEditPage";

const BRANCH = {
  id: 3,
  vendor_id: 42,
  branch_code: "JKT01",
  branch_name: "Cabang Jakarta",
  location_id: 15,
  region: "Jabodetabek",
  region_code: "JKT",
  category: "CASH",
};

function control(label: string): HTMLInputElement | HTMLSelectElement {
  const el = screen
    .getByText(label, { selector: "span" })
    .parentElement?.querySelector<HTMLInputElement | HTMLSelectElement>("input, select");
  if (!el) throw new Error(`control for "${label}" not found`);
  return el;
}

const backToVendor = { to: "/settings/admin/vendors/$vendorId", params: { vendorId: "42" } };

beforeEach(() => {
  m.navigate.mockReset();
  m.update.mockReset();
  m.branch = { isLoading: false, isError: false, data: BRANCH };
  useToastStore.setState({ toasts: [] });
});

describe("VendorBranchEditPage", () => {
  it("shows loading, then not-found", () => {
    m.branch = { isLoading: true, isError: false, data: undefined };
    const { rerender } = render(<VendorBranchEditPage vendorId={42} branchId={3} />);
    expect(screen.getByText("Memuat…")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Simpan" })).not.toBeInTheDocument();

    m.branch = { isLoading: false, isError: true, data: undefined };
    rerender(<VendorBranchEditPage vendorId={42} branchId={3} />);
    expect(screen.getByRole("alert")).toHaveTextContent("Cabang tidak ditemukan");
  });

  it("prefills the loaded branch and locks the branch code", async () => {
    render(<VendorBranchEditPage vendorId={42} branchId={3} />);

    expect(screen.getByText("Ubah Cabang · JKT01")).toBeInTheDocument();
    await waitFor(() => expect(control("Nama Cabang")).toHaveValue("Cabang Jakarta"));
    expect(control("Kode Cabang")).toHaveValue("JKT01");
    expect(control("Kode Cabang")).toBeDisabled();
    expect(control("Tipe")).toHaveValue("CASH");
    expect(control("ID Lokasi (opsional)")).toHaveValue("15");
  });

  it("submits the change for approval, upper-cases the region code, then goes back", async () => {
    m.update.mockResolvedValue({ change_request_id: 601, status: "pending" });
    render(<VendorBranchEditPage vendorId={42} branchId={3} />);
    await waitFor(() => expect(control("Nama Cabang")).toHaveValue("Cabang Jakarta"));

    fireEvent.change(control("Nama Cabang"), { target: { value: "Cabang Jakarta Pusat" } });
    fireEvent.change(control("Kode Region"), { target: { value: " jkp " } });
    fireEvent.change(control("Wilayah"), { target: { value: "" } });
    fireEvent.change(control("ID Lokasi (opsional)"), { target: { value: "" } });
    await userEvent.setup().click(screen.getByRole("button", { name: "Simpan" }));

    await waitFor(() => expect(m.update).toHaveBeenCalledTimes(1));
    expect(m.update).toHaveBeenCalledWith({
      branchId: 3,
      payload: {
        branch_name: "Cabang Jakarta Pusat",
        location_id: null,
        region: null,
        region_code: "JKP",
        category: "CASH",
      },
    });
    expect(useToastStore.getState().toasts[0]).toMatchObject({
      type: "success",
      message: "Perubahan diajukan dan menunggu persetujuan (#601)",
    });
    expect(m.navigate).toHaveBeenCalledWith(backToVendor);
  });

  it("an empty branch name is rejected client-side", async () => {
    render(<VendorBranchEditPage vendorId={42} branchId={3} />);
    await waitFor(() => expect(control("Nama Cabang")).toHaveValue("Cabang Jakarta"));

    fireEvent.change(control("Nama Cabang"), { target: { value: "" } });
    await userEvent.setup().click(screen.getByRole("button", { name: "Simpan" }));

    expect(await screen.findByText("Wajib diisi")).toBeInTheDocument();
    expect(m.update).not.toHaveBeenCalled();
  });

  it("a server field error lands on the field and the page stays", async () => {
    m.update.mockRejectedValue({
      status: 422,
      message: "Validasi gagal",
      details: [{ field: "region_code", message: "Kode region tidak dikenal" }],
    });
    render(<VendorBranchEditPage vendorId={42} branchId={3} />);
    await waitFor(() => expect(control("Nama Cabang")).toHaveValue("Cabang Jakarta"));

    await userEvent.setup().click(screen.getByRole("button", { name: "Simpan" }));

    expect(await screen.findByText("Kode region tidak dikenal")).toBeInTheDocument();
    expect(m.navigate).not.toHaveBeenCalled();
  });

  it("any other failure is a toast and the page stays", async () => {
    m.update.mockRejectedValue({ status: 409, message: "Masih ada perubahan pending" });
    render(<VendorBranchEditPage vendorId={42} branchId={3} />);
    await waitFor(() => expect(control("Nama Cabang")).toHaveValue("Cabang Jakarta"));

    await userEvent.setup().click(screen.getByRole("button", { name: "Simpan" }));

    await waitFor(() =>
      expect(useToastStore.getState().toasts[0]).toMatchObject({
        type: "error",
        message: "Masih ada perubahan pending",
      }),
    );
    expect(m.navigate).not.toHaveBeenCalled();
  });

  it("Batal goes back to the vendor without submitting", async () => {
    render(<VendorBranchEditPage vendorId={42} branchId={3} />);

    await userEvent.setup().click(screen.getByRole("button", { name: "Batal" }));

    expect(m.navigate).toHaveBeenCalledWith(backToVendor);
    expect(m.update).not.toHaveBeenCalled();
  });
});
