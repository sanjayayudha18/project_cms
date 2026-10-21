/**
 * vendor-party-scope Req 5.9: "Ubah peran" dialog -- immediate apply, clearly not maker-checker.
 * The hook is mocked; apiHooks.test.tsx covers the real request.
 */

import { useToastStore } from "@/lib/hooks/useToast";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

const m = vi.hoisted(() => ({ setRoles: vi.fn() }));
vi.mock("../hooks", () => ({
  useSetPicRoles: () => ({ mutateAsync: m.setRoles, isPending: false }),
}));

import { PicRolesDialog } from "../components/PicRolesDialog";
import type { AdminVendorPic } from "../types";

const PIC = {
  id: 6,
  vendor_id: 42,
  vendor_branch_id: 3,
  name: "Sari",
  position: null,
  phone: null,
  email: null,
  is_notification_recipient: false,
  roles: ["replenish"],
  is_active: true,
  deleted_at: null,
} as AdminVendorPic;

beforeEach(() => {
  m.setRoles.mockReset();
  useToastStore.setState({ toasts: [] });
});

describe("PicRolesDialog", () => {
  it("states that the change applies immediately without approval", () => {
    render(<PicRolesDialog open onClose={vi.fn()} vendorId={42} pic={PIC} />);

    expect(screen.getByText("Langsung berlaku, tanpa persetujuan.")).toBeInTheDocument();
  });

  it("prefills the current roles", async () => {
    render(<PicRolesDialog open onClose={vi.fn()} vendorId={42} pic={PIC} />);

    await waitFor(() =>
      expect(screen.getByRole("checkbox", { name: "Pengisian ATM" })).toBeChecked(),
    );
    expect(screen.getByRole("checkbox", { name: "Penyedia uang" })).not.toBeChecked();
    expect(screen.getByRole("checkbox", { name: "Cash count" })).not.toBeChecked();
  });

  it("saves the chosen roles, toasts and closes", async () => {
    m.setRoles.mockResolvedValue({ id: 6, roles: ["replenish", "vault"] });
    const onClose = vi.fn();
    const user = userEvent.setup();
    render(<PicRolesDialog open onClose={onClose} vendorId={42} pic={PIC} />);

    await waitFor(() =>
      expect(screen.getByRole("checkbox", { name: "Pengisian ATM" })).toBeChecked(),
    );
    await user.click(screen.getByRole("checkbox", { name: "Penyedia uang" }));
    await user.click(screen.getByRole("button", { name: "Simpan" }));

    await waitFor(() => expect(m.setRoles).toHaveBeenCalledTimes(1));
    expect(m.setRoles).toHaveBeenCalledWith({ picId: 6, roles: ["replenish", "vault"] });
    expect(useToastStore.getState().toasts[0]).toMatchObject({
      type: "success",
      message: "Peran PIC diperbarui",
    });
    expect(onClose).toHaveBeenCalled();
  });

  it("can save an empty set (contact only)", async () => {
    m.setRoles.mockResolvedValue({ id: 6, roles: [] });
    const user = userEvent.setup();
    render(<PicRolesDialog open onClose={vi.fn()} vendorId={42} pic={PIC} />);

    await waitFor(() =>
      expect(screen.getByRole("checkbox", { name: "Pengisian ATM" })).toBeChecked(),
    );
    await user.click(screen.getByRole("checkbox", { name: "Pengisian ATM" }));
    await user.click(screen.getByRole("button", { name: "Simpan" }));

    await waitFor(() => expect(m.setRoles).toHaveBeenCalledWith({ picId: 6, roles: [] }));
  });

  it("shows the server message and stays open when saving fails", async () => {
    m.setRoles.mockRejectedValue({ status: 404, message: "PIC tidak ditemukan" });
    const onClose = vi.fn();
    const user = userEvent.setup();
    render(<PicRolesDialog open onClose={onClose} vendorId={42} pic={PIC} />);

    await user.click(screen.getByRole("button", { name: "Simpan" }));

    await waitFor(() =>
      expect(useToastStore.getState().toasts[0]).toMatchObject({
        type: "error",
        message: "PIC tidak ditemukan",
      }),
    );
    expect(onClose).not.toHaveBeenCalled();
  });
});
