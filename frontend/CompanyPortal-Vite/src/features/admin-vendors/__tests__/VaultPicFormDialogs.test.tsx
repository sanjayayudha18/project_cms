/**
 * fe-coverage C5: vault + PIC forms (master-data maker-checker submissions).
 * Mutations mocked; the real form, schema and payload mapping run.
 */

import { useToastStore } from "@/lib/hooks/useToast";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

const m = vi.hoisted(() => ({
  createVault: vi.fn(),
  updateVault: vi.fn(),
  createPic: vi.fn(),
  updatePic: vi.fn(),
}));
vi.mock("../hooks", () => ({
  useCreateVendorVault: () => ({ mutateAsync: m.createVault, isPending: false }),
  useUpdateVendorVault: () => ({ mutateAsync: m.updateVault, isPending: false }),
  useCreateVendorPic: () => ({ mutateAsync: m.createPic, isPending: false }),
  useUpdateVendorPic: () => ({ mutateAsync: m.updatePic, isPending: false }),
}));

import { VendorPicFormDialog } from "../components/VendorPicFormDialog";
import { VendorVaultFormDialog } from "../components/VendorVaultFormDialog";

function control(label: string): HTMLInputElement | HTMLSelectElement {
  const el = screen
    .getByText(label, { selector: "span" })
    .parentElement?.querySelector<HTMLInputElement | HTMLSelectElement>("input, select");
  if (!el) throw new Error(`control for "${label}" not found`);
  return el;
}
const set = (label: string, value: string) =>
  fireEvent.change(control(label), { target: { value } });
const save = () => userEvent.setup().click(screen.getByRole("button", { name: "Simpan" }));

const ACCEPTED = { change_request_id: 701, status: "pending" };
const PENDING_TOAST = "Perubahan diajukan dan menunggu persetujuan (#701)";

beforeEach(() => {
  for (const fn of Object.values(m)) fn.mockReset();
  useToastStore.setState({ toasts: [] });
});

describe("VendorVaultFormDialog", () => {
  it("creates a vault for the context branch; optionals → null, currency upper-cased", async () => {
    m.createVault.mockResolvedValue(ACCEPTED);
    const onClose = vi.fn();
    render(
      <VendorVaultFormDialog open onClose={onClose} vendorId={42} branchId={3} vault={null} />,
    );

    expect(screen.getByRole("dialog", { name: "Tambah Vault" })).toBeInTheDocument();
    set("Kode Vault", "VLT-JKT-1");
    set("Kategori", "CASH");
    set("Mata Uang", "idr");
    set("Kapasitas Maks", "5000000000.00");
    await save();

    await waitFor(() => expect(m.createVault).toHaveBeenCalledTimes(1));
    expect(m.createVault).toHaveBeenCalledWith({
      vendor_branch_id: 3,
      vault_code: "VLT-JKT-1",
      category: "CASH",
      currency_code: "IDR",
      min_capacity_amount: null,
      max_capacity_amount: "5000000000.00",
      latitude: null,
      longitude: null,
      operating_hours: null,
      location_id: null,
    });
    expect(useToastStore.getState().toasts[0]).toMatchObject({
      type: "success",
      message: PENDING_TOAST,
    });
    expect(onClose).toHaveBeenCalled();
  });

  it("edit is prefilled, keeps the vault code locked and sends no code", async () => {
    m.updateVault.mockResolvedValue(ACCEPTED);
    const vault = {
      id: 9,
      vault_code: "VLT-OLD",
      category: "ATM_CASH",
      currency_code: "IDR",
      min_capacity_amount: "100.00",
      max_capacity_amount: null,
      latitude: "-6.2",
      longitude: "106.8",
      operating_hours: "08-17",
      location_id: 15,
    };
    render(
      <VendorVaultFormDialog
        open
        onClose={vi.fn()}
        vendorId={42}
        branchId={3}
        vault={vault as never}
      />,
    );

    expect(screen.getByRole("dialog", { name: "Ubah Vault" })).toBeInTheDocument();
    await waitFor(() => expect(control("Kode Vault")).toHaveValue("VLT-OLD"));
    expect(control("Kode Vault")).toBeDisabled();
    expect(control("ID Lokasi (opsional)")).toHaveValue("15");
    set("Jam Operasional", "24 jam");
    await save();

    await waitFor(() => expect(m.updateVault).toHaveBeenCalledTimes(1));
    expect(m.updateVault).toHaveBeenCalledWith({
      vaultId: 9,
      payload: {
        category: "ATM_CASH",
        currency_code: "IDR",
        min_capacity_amount: "100.00",
        max_capacity_amount: null,
        latitude: "-6.2",
        longitude: "106.8",
        operating_hours: "24 jam",
        location_id: 15,
      },
    });
  });

  it("validates the code and currency client-side", async () => {
    render(
      <VendorVaultFormDialog open onClose={vi.fn()} vendorId={42} branchId={3} vault={null} />,
    );

    set("Mata Uang", "ID1");
    await save();

    expect(await screen.findByText("Wajib diisi")).toBeInTheDocument();
    expect(screen.getByText("Hanya huruf")).toBeInTheDocument();
    expect(m.createVault).not.toHaveBeenCalled();
  });

  it.each([
    [
      {
        status: 422,
        message: "x",
        details: [{ field: "vault_code", message: "Kode vault sudah ada" }],
      },
      "field",
      "Kode vault sudah ada",
    ],
    [
      { status: 409, message: "Masih ada perubahan pending" },
      "toast",
      "Masih ada perubahan pending",
    ],
    ["boom", "toast", "Terjadi kesalahan yang tidak diketahui"],
  ])(
    "a failed submit (%j) shows the problem and keeps the dialog open",
    async (err, where, text) => {
      m.createVault.mockRejectedValue(err);
      const onClose = vi.fn();
      render(
        <VendorVaultFormDialog open onClose={onClose} vendorId={42} branchId={3} vault={null} />,
      );

      set("Kode Vault", "VLT-1");
      await save();

      if (where === "field") {
        expect(await screen.findByText(text)).toBeInTheDocument();
      } else {
        await waitFor(() =>
          expect(useToastStore.getState().toasts[0]).toMatchObject({
            type: "error",
            message: text,
          }),
        );
      }
      expect(onClose).not.toHaveBeenCalled();
    },
  );
});

describe("VendorPicFormDialog", () => {
  it("creates a vendor-wide PIC (no branch) with optional fields as null", async () => {
    m.createPic.mockResolvedValue(ACCEPTED);
    const onClose = vi.fn();
    render(<VendorPicFormDialog open onClose={onClose} vendorId={42} branchId={null} pic={null} />);

    expect(screen.getByText("PIC berlaku untuk seluruh vendor")).toBeInTheDocument();
    set("Nama", "Andi");
    await userEvent.setup().click(screen.getByRole("checkbox", { name: "Penerima notifikasi" }));
    await save();

    await waitFor(() => expect(m.createPic).toHaveBeenCalledTimes(1));
    expect(m.createPic).toHaveBeenCalledWith({
      vendor_branch_id: null,
      name: "Andi",
      position: null,
      phone: null,
      email: null,
      is_notification_recipient: true,
      roles: ["replenish", "vault", "cashcount"],
    });
    expect(useToastStore.getState().toasts[0]).toMatchObject({ message: PENDING_TOAST });
    expect(onClose).toHaveBeenCalled();
  });

  it("create: roles default to all three and the unchecked ones are not sent", async () => {
    m.createPic.mockResolvedValue(ACCEPTED);
    const user = userEvent.setup();
    render(<VendorPicFormDialog open onClose={vi.fn()} vendorId={42} branchId={3} pic={null} />);

    for (const name of ["Pengisian ATM", "Penyedia uang", "Cash count"]) {
      expect(screen.getByRole("checkbox", { name })).toBeChecked();
    }
    set("Nama", "Andi");
    await user.click(screen.getByRole("checkbox", { name: "Cash count" }));
    await user.click(screen.getByRole("checkbox", { name: "Penyedia uang" }));
    await save();

    await waitFor(() => expect(m.createPic).toHaveBeenCalledTimes(1));
    expect(m.createPic.mock.calls[0]?.[0]).toMatchObject({ roles: ["replenish"] });
  });

  it("edit: the role checkboxes are hidden (roles change via 'Ubah peran')", async () => {
    const pic = {
      id: 6,
      name: "Sari",
      position: null,
      phone: null,
      email: null,
      is_notification_recipient: false,
      roles: ["vault"],
    };
    render(
      <VendorPicFormDialog open onClose={vi.fn()} vendorId={42} branchId={3} pic={pic as never} />,
    );

    expect(screen.queryByRole("checkbox", { name: "Penyedia uang" })).not.toBeInTheDocument();
    expect(screen.getByRole("checkbox", { name: "Penerima notifikasi" })).toBeInTheDocument();
  });

  it("edits a branch PIC, prefilled", async () => {
    m.updatePic.mockResolvedValue(ACCEPTED);
    const pic = {
      id: 6,
      name: "Sari",
      position: null,
      phone: "0812",
      email: "sari@vendor.co.id",
      is_notification_recipient: false,
      roles: ["replenish"],
    };
    render(
      <VendorPicFormDialog open onClose={vi.fn()} vendorId={42} branchId={3} pic={pic as never} />,
    );

    expect(screen.getByRole("dialog", { name: "Ubah PIC" })).toBeInTheDocument();
    expect(screen.getByText("PIC untuk cabang ini")).toBeInTheDocument();
    await waitFor(() => expect(control("Nama")).toHaveValue("Sari"));
    set("Jabatan", "Supervisor");
    await save();

    await waitFor(() => expect(m.updatePic).toHaveBeenCalledTimes(1));
    expect(m.updatePic).toHaveBeenCalledWith({
      picId: 6,
      payload: {
        vendor_branch_id: 3,
        name: "Sari",
        position: "Supervisor",
        phone: "0812",
        email: "sari@vendor.co.id",
        is_notification_recipient: false,
      },
    });
  });

  it("a bad email is never sent; server errors land on the field or a toast", async () => {
    render(<VendorPicFormDialog open onClose={vi.fn()} vendorId={42} branchId={3} pic={null} />);
    set("Nama", "Andi");
    set("Email", "andi@");
    await save();
    // type="email": the browser's own constraint check stops the submit before zod runs
    expect(m.createPic).not.toHaveBeenCalled();

    m.createPic.mockRejectedValueOnce({
      status: 422,
      message: "x",
      details: [{ field: "name", message: "Nama sudah dipakai" }],
    });
    set("Email", "");
    await save();
    expect(await screen.findByText("Nama sudah dipakai")).toBeInTheDocument();

    m.createPic.mockRejectedValueOnce({ status: 500, message: "Gagal" });
    await save();
    await waitFor(() =>
      expect(useToastStore.getState().toasts.at(-1)).toMatchObject({
        type: "error",
        message: "Gagal",
      }),
    );
  });
});
