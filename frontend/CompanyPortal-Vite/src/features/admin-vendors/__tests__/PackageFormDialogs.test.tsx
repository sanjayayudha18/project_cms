/**
 * fe-coverage C3: the two price forms (money + maker-checker).
 * - VendorPackageFormDialog: a branch's own special price (vendor_packages_branch).
 * - VendorPackagePriceFormDialog: the vendor-wide tariff tree (vendor_package_prices).
 * Mutations are mocked; the real form, schema, payload mapping and toasts run.
 */

import { useToastStore } from "@/lib/hooks/useToast";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactElement } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const m = vi.hoisted(() => ({
  createPkg: vi.fn(),
  updatePkg: vi.fn(),
  createPrice: vi.fn(),
  updatePrice: vi.fn(),
  listChildren: vi.fn(),
}));
vi.mock("../hooks", () => ({
  useCreateVendorPackage: () => ({ mutateAsync: m.createPkg, isPending: false }),
  useUpdateVendorPackage: () => ({ mutateAsync: m.updatePkg, isPending: false }),
  useCreateVendorPackagePrice: () => ({ mutateAsync: m.createPrice, isPending: false }),
  useUpdateVendorPackagePrice: () => ({ mutateAsync: m.updatePrice, isPending: false }),
}));
vi.mock("../api", () => ({ listVendorChildren: m.listChildren }));

import { VendorPackageFormDialog } from "../components/VendorPackageFormDialog";
import { VendorPackagePriceFormDialog } from "../components/VendorPackagePriceFormDialog";

function renderDialog(ui: ReactElement) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(<QueryClientProvider client={client}>{ui}</QueryClientProvider>);
}

/** Labels are <span>s next to the control (not <label>), so find the sibling control. */
function control(label: string): HTMLInputElement | HTMLSelectElement {
  const el = screen
    .getByText(label, { selector: "span" })
    .parentElement?.querySelector<HTMLInputElement | HTMLSelectElement>("input, select, textarea");
  if (!el) throw new Error(`control for "${label}" not found`);
  return el;
}

function set(label: string, value: string) {
  fireEvent.change(control(label), { target: { value } });
}

const ACCEPTED = { change_request_id: 501, status: "pending" };
const PENDING_TOAST = "Perubahan diajukan dan menunggu persetujuan (#501)";

beforeEach(() => {
  for (const fn of Object.values(m)) fn.mockReset();
  m.listChildren.mockResolvedValue([
    { id: 3, branch_code: "JKT01", branch_name: "Cabang Jakarta", is_active: true },
    { id: 4, branch_code: "OLD01", branch_name: "Cabang Lama", is_active: false },
  ]);
  useToastStore.setState({ toasts: [] });
});

describe("VendorPackageFormDialog — harga khusus cabang", () => {
  it("creates for the context branch; empty optionals become null, price stays a decimal string", async () => {
    m.createPkg.mockResolvedValue(ACCEPTED);
    const onClose = vi.fn();
    renderDialog(<VendorPackageFormDialog open onClose={onClose} vendorId={42} branchId={3} />);

    expect(screen.getByRole("dialog", { name: "Tambah Harga Khusus Cabang" })).toBeInTheDocument();
    set("Kode Paket", "PKG-KHUSUS");
    set("Mulai Berlaku", "2026-11-01");
    set("Harga Khusus (kosong = ikut harga vendor biasa)", "1500000.50");
    await userEvent.setup().click(screen.getByRole("button", { name: "Simpan" }));

    await waitFor(() => expect(m.createPkg).toHaveBeenCalledTimes(1));
    expect(m.createPkg).toHaveBeenCalledWith({
      vendor_branch_id: 3,
      package_code: "PKG-KHUSUS",
      machine_group: "ATM",
      price_class: "REGULAR",
      tier_min: 1,
      tier_max: null,
      atm_id: null,
      currency: "IDR",
      effective_start_date: "2026-11-01",
      base_price: "1500000.50",
      sla_note: null,
      effective_end_date: null,
    });
    expect(useToastStore.getState().toasts[0]).toMatchObject({
      type: "success",
      message: PENDING_TOAST,
    });
    expect(onClose).toHaveBeenCalled();
  });

  it("blocks a price with more than 2 decimals without calling the API", async () => {
    renderDialog(<VendorPackageFormDialog open onClose={vi.fn()} vendorId={42} branchId={3} />);

    set("Kode Paket", "PKG-1");
    set("Mulai Berlaku", "2026-11-01");
    set("Harga Khusus (kosong = ikut harga vendor biasa)", "10.999");
    await userEvent.setup().click(screen.getByRole("button", { name: "Simpan" }));

    expect(await screen.findByText("Harus angka desimal tidak negatif")).toBeInTheDocument();
    expect(m.createPkg).not.toHaveBeenCalled();
  });

  it("edit sends only the content fields; grain fields are locked", async () => {
    m.updatePkg.mockResolvedValue(ACCEPTED);
    const pkg = {
      id: 9,
      package_code: "PKG-1",
      machine_group: "CDM_CRM",
      price_class: "VIP_INDUSTRI",
      tier_min: 2,
      tier_max: 5,
      atm_id: 77,
      currency: "IDR",
      effective_start_date: "2026-01-01",
      base_price: "200000.00",
      sla_note: "SLA 4 jam",
      effective_end_date: null,
    };
    renderDialog(
      <VendorPackageFormDialog
        open
        onClose={vi.fn()}
        vendorId={42}
        branchId={3}
        pkg={pkg as never}
      />,
    );

    expect(control("Kode Paket")).toBeDisabled();
    expect(control("Mulai Berlaku")).toBeDisabled();
    set("Harga Khusus (kosong = ikut harga vendor biasa)", "250000.00");
    set("Berakhir (kosong = terbuka)", "2026-12-31");
    await userEvent.setup().click(screen.getByRole("button", { name: "Simpan" }));

    await waitFor(() => expect(m.updatePkg).toHaveBeenCalledTimes(1));
    expect(m.updatePkg).toHaveBeenCalledWith({
      packageId: 9,
      payload: { base_price: "250000.00", sla_note: "SLA 4 jam", effective_end_date: "2026-12-31" },
    });
  });

  it("a server field error lands on the field; the dialog stays open", async () => {
    m.createPkg.mockRejectedValue({
      status: 422,
      message: "Validasi gagal",
      details: [{ field: "package_code", message: "Kode paket sudah ada" }],
    });
    const onClose = vi.fn();
    renderDialog(<VendorPackageFormDialog open onClose={onClose} vendorId={42} branchId={3} />);

    set("Kode Paket", "PKG-1");
    set("Mulai Berlaku", "2026-11-01");
    await userEvent.setup().click(screen.getByRole("button", { name: "Simpan" }));

    expect(await screen.findByText("Kode paket sudah ada")).toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();
  });

  it("any other failure is a toast; the dialog stays open", async () => {
    m.createPkg.mockRejectedValue({ status: 409, message: "Periode tumpang tindih" });
    const onClose = vi.fn();
    renderDialog(<VendorPackageFormDialog open onClose={onClose} vendorId={42} branchId={3} />);

    set("Kode Paket", "PKG-1");
    set("Mulai Berlaku", "2026-11-01");
    await userEvent.setup().click(screen.getByRole("button", { name: "Simpan" }));

    await waitFor(() =>
      expect(useToastStore.getState().toasts[0]).toMatchObject({
        type: "error",
        message: "Periode tumpang tindih",
      }),
    );
    expect(onClose).not.toHaveBeenCalled();
  });
});

describe("VendorPackagePriceFormDialog — tarif vendor-wide", () => {
  it("offers only active branches as overrides and creates a branch override", async () => {
    m.createPrice.mockResolvedValue(ACCEPTED);
    renderDialog(
      <VendorPackagePriceFormDialog open onClose={vi.fn()} vendorId={42} price={null} />,
    );

    expect(
      await screen.findByRole("option", { name: "JKT01 · Cabang Jakarta" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "OLD01 · Cabang Lama" })).not.toBeInTheDocument();
    expect(m.listChildren).toHaveBeenCalledWith(42, "branches");

    set("Paket", "PAKET 3");
    set("Kelompok Mesin", "CDM_CRM");
    set("Tingkat Maks (kosong = terbuka)", "10");
    set("Cabang (opsional, override cabang)", "3");
    set("Mulai Berlaku", "2026-11-01");
    set("Harga Dasar (kosong = ikut tingkat di atas)", "350000");
    await userEvent.setup().click(screen.getByRole("button", { name: "Simpan" }));

    await waitFor(() => expect(m.createPrice).toHaveBeenCalledTimes(1));
    expect(m.createPrice).toHaveBeenCalledWith({
      package: "PAKET 3",
      machine_group: "CDM_CRM",
      price_class: "REGULAR",
      tier_min: 1,
      tier_max: 10,
      vendor_branch_id: 3,
      atm_id: null,
      currency: "IDR",
      effective_start_date: "2026-11-01",
      base_price: "350000",
      sla_note: null,
      effective_end_date: null,
    });
    expect(m.createPrice.mock.calls[0]?.[0]).not.toHaveProperty("package_code");
    expect(useToastStore.getState().toasts[0]).toMatchObject({ message: PENDING_TOAST });
  });

  it("rejects a branch AND an ATM override together", async () => {
    renderDialog(
      <VendorPackagePriceFormDialog open onClose={vi.fn()} vendorId={42} price={null} />,
    );
    await screen.findByRole("option", { name: "JKT01 · Cabang Jakarta" });

    set("Paket", "PAKET 3");
    set("Cabang (opsional, override cabang)", "3");
    set("ID ATM (opsional, Harga Khusus)", "77");
    set("Mulai Berlaku", "2026-11-01");
    await userEvent.setup().click(screen.getByRole("button", { name: "Simpan" }));

    expect(
      await screen.findByText("Tidak boleh diisi bersamaan dengan cabang"),
    ).toBeInTheDocument();
    expect(m.createPrice).not.toHaveBeenCalled();
  });

  it("edit sends only price, SLA note and end date", async () => {
    m.updatePrice.mockResolvedValue(ACCEPTED);
    const price = {
      id: 12,
      package: "PAKET 3",
      machine_group: "ATM",
      price_class: "REGULAR",
      tier_min: 1,
      tier_max: null,
      vendor_branch_id: null,
      atm_id: null,
      currency: "IDR",
      effective_start_date: "2026-01-01",
      base_price: null,
      sla_note: null,
      effective_end_date: null,
    };
    const onClose = vi.fn();
    renderDialog(
      <VendorPackagePriceFormDialog open onClose={onClose} vendorId={42} price={price as never} />,
    );

    expect(screen.getByRole("dialog", { name: "Ubah Harga Paket" })).toBeInTheDocument();
    expect(control("Paket")).toBeDisabled();
    set("Harga Dasar (kosong = ikut tingkat di atas)", "99000.5");
    await userEvent.setup().click(screen.getByRole("button", { name: "Simpan" }));

    await waitFor(() => expect(m.updatePrice).toHaveBeenCalledTimes(1));
    expect(m.updatePrice).toHaveBeenCalledWith({
      priceId: 12,
      payload: { base_price: "99000.5", sla_note: null, effective_end_date: null },
    });
    expect(onClose).toHaveBeenCalled();
  });

  it("an unknown failure shows a generic toast", async () => {
    m.createPrice.mockRejectedValue("boom");
    renderDialog(
      <VendorPackagePriceFormDialog open onClose={vi.fn()} vendorId={42} price={null} />,
    );

    set("Paket", "PAKET 3");
    set("Mulai Berlaku", "2026-11-01");
    await userEvent.setup().click(screen.getByRole("button", { name: "Simpan" }));

    await waitFor(() =>
      expect(useToastStore.getState().toasts[0]).toMatchObject({
        type: "error",
        message: "Terjadi kesalahan yang tidak diketahui",
      }),
    );
  });
});
