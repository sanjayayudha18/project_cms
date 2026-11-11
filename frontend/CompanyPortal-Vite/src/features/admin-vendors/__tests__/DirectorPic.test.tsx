/**
 * vendor-pks 11.5 (Req 9.1-9.3): Direktur Utama as a vendor-wide PIC -- the form checkbox locks
 * notification + roles, the list badges it and hides "Ubah peran", and a 409 shows as a banner.
 * Hooks mocked; the real form, schema and payload mapping run.
 */

import { useToastStore } from "@/lib/hooks/useToast";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

const m = vi.hoisted(() => ({
  createPic: vi.fn(),
  updatePic: vi.fn(),
  query: {} as Record<string, unknown>,
}));
vi.mock("../hooks", () => ({
  useCreateVendorPic: () => ({ mutateAsync: m.createPic, isPending: false }),
  useUpdateVendorPic: () => ({ mutateAsync: m.updatePic, isPending: false }),
  useVendorPics: () => m.query,
  useDisableVendorPic: () => ({ mutate: vi.fn(), isPending: false }),
  useEnableVendorPic: () => ({ mutate: vi.fn(), isPending: false }),
  useSetPicRoles: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));
vi.mock("../../master-data/pending", () => ({
  usePendingEntityIds: () => new Set<number>(),
  usePendingCreates: () => [],
}));

import { PicsPanel } from "../components/PicsPanel";
import { VendorPicFormDialog } from "../components/VendorPicFormDialog";

const DIRECTOR = {
  id: 1,
  name: "Pak Direktur",
  position: "Pemilik",
  phone: null,
  email: null,
  is_notification_recipient: false,
  is_director: true,
  roles: [] as string[],
  is_active: true,
};
const REGULAR = {
  id: 2,
  name: "Andi",
  position: null,
  phone: null,
  email: null,
  is_notification_recipient: true,
  is_director: false,
  roles: ["replenish"],
  is_active: true,
};

beforeEach(() => {
  m.createPic.mockReset();
  m.updatePic.mockReset();
  m.query = {
    isLoading: false,
    isError: false,
    data: { pics: [DIRECTOR, REGULAR], total: 2, warnings: [] },
  };
  useToastStore.setState({ toasts: [] });
});

function nameInput(): HTMLInputElement {
  const el = screen.getByText("Nama", { selector: "span" }).parentElement?.querySelector("input");
  if (!el) throw new Error("name input not found");
  return el;
}
const rowOf = (name: string) => screen.getByText(name).closest("tr") as HTMLElement;

describe("VendorPicFormDialog: Direktur Utama", () => {
  const renderForm = (branchId: number | null = null, pic: unknown = null, onClose = vi.fn()) =>
    render(
      <VendorPicFormDialog
        open
        onClose={onClose}
        vendorId={42}
        branchId={branchId}
        pic={pic as never}
      />,
    );

  it("offers the checkbox only on the vendor-wide form", () => {
    const { unmount } = renderForm(null);
    expect(screen.getByRole("checkbox", { name: "Direktur Utama" })).not.toBeChecked();
    unmount();

    renderForm(3);
    expect(screen.queryByRole("checkbox", { name: "Direktur Utama" })).toBeNull();
  });

  it("checking it locks notification, clears it and hides the roles", async () => {
    const user = userEvent.setup();
    renderForm(null);

    await user.click(screen.getByRole("checkbox", { name: "Penerima notifikasi" }));
    expect(screen.getByRole("checkbox", { name: "Pengisian ATM" })).toBeChecked();

    await user.click(screen.getByRole("checkbox", { name: "Direktur Utama" }));

    const notif = screen.getByRole("checkbox", { name: "Penerima notifikasi" });
    expect(notif).toBeDisabled();
    expect(notif).not.toBeChecked();
    expect(screen.queryByRole("checkbox", { name: "Pengisian ATM" })).toBeNull();
    expect(screen.getByText(/tanpa login, tanpa peran PIC, tanpa notifikasi/)).toBeVisible();
  });

  it("unchecking restores the controls and the default roles on create", async () => {
    const user = userEvent.setup();
    renderForm(null);

    await user.click(screen.getByRole("checkbox", { name: "Direktur Utama" }));
    await user.click(screen.getByRole("checkbox", { name: "Direktur Utama" }));

    expect(screen.getByRole("checkbox", { name: "Penerima notifikasi" })).toBeEnabled();
    expect(screen.getByRole("checkbox", { name: "Pengisian ATM" })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: "Penyedia uang" })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: "Cash count" })).toBeChecked();
  });

  it("submits is_director: true with no roles and no notification", async () => {
    m.createPic.mockResolvedValue({ change_request_id: 801 });
    const onClose = vi.fn();
    const user = userEvent.setup();
    renderForm(null, null, onClose);

    await user.type(nameInput(), "Pak Direktur");
    await user.click(screen.getByRole("checkbox", { name: "Direktur Utama" }));
    await user.click(screen.getByRole("button", { name: "Simpan" }));

    await waitFor(() => expect(m.createPic).toHaveBeenCalledTimes(1));
    expect(m.createPic).toHaveBeenCalledWith({
      vendor_branch_id: null,
      name: "Pak Direktur",
      position: null,
      phone: null,
      email: null,
      is_notification_recipient: false,
      is_director: true,
      roles: [],
    });
    expect(onClose).toHaveBeenCalled();
  });

  it("a regular PIC omits the is_director flag", async () => {
    m.createPic.mockResolvedValue({ change_request_id: 802 });
    const user = userEvent.setup();
    renderForm(null);

    await user.type(nameInput(), "Budi");
    await user.click(screen.getByRole("button", { name: "Simpan" }));

    await waitFor(() => expect(m.createPic).toHaveBeenCalledTimes(1));
    expect(m.createPic.mock.calls[0][0].is_director).toBeUndefined();
  });

  it("editing the director opens with the box checked and no roles section", () => {
    renderForm(null, { ...DIRECTOR, vendor_id: 42, vendor_branch_id: null });

    expect(screen.getByRole("checkbox", { name: "Direktur Utama" })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: "Penerima notifikasi" })).toBeDisabled();
    expect(screen.queryByText("Peran")).toBeNull();
  });

  it("409 (a director already exists) shows a banner and keeps the dialog open", async () => {
    m.createPic.mockRejectedValue({
      status: 409,
      message: "Vendor ini sudah punya Direktur Utama",
    });
    const onClose = vi.fn();
    const user = userEvent.setup();
    renderForm(null, null, onClose);

    await user.type(nameInput(), "Direktur Kedua");
    await user.click(screen.getByRole("checkbox", { name: "Direktur Utama" }));
    await user.click(screen.getByRole("button", { name: "Simpan" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Vendor ini sudah punya Direktur Utama",
    );
    expect(onClose).not.toHaveBeenCalled();
  });
});

describe("PicsPanel: Direktur Utama", () => {
  it("badges the director with text and hides 'Ubah peran' for that row only", () => {
    render(<PicsPanel vendorId={42} branchId={null} />);

    const director = within(rowOf("Pak Direktur"));
    expect(director.getByText("Direktur Utama")).toBeInTheDocument();
    expect(director.queryByRole("button", { name: "Ubah peran" })).toBeNull();
    expect(director.getByRole("button", { name: "Ubah" })).toBeEnabled();
    expect(director.getByRole("button", { name: "Nonaktifkan" })).toBeEnabled();

    const regular = within(rowOf("Andi"));
    expect(regular.queryByText("Direktur Utama")).toBeNull();
    expect(regular.getByRole("button", { name: "Ubah peran" })).toBeEnabled();
  });
});
