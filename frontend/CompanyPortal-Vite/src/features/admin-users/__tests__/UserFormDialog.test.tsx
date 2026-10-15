import { useToastStore } from "@/lib/hooks/useToast";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { UserFormDialog } from "../components/UserFormDialog";
import type { AdminUser } from "../types";

const createMutateAsync = vi.fn();
const updateMutateAsync = vi.fn();

vi.mock("../hooks", () => ({
  useCreateUser: () => ({ mutateAsync: createMutateAsync, isPending: false }),
  useUpdateUser: () => ({ mutateAsync: updateMutateAsync, isPending: false }),
}));

vi.mock("@/features/admin-vendors/hooks", () => ({
  useVendorsList: () => ({
    data: { vendors: [{ id: 1, name: "PT Gardanet" }], page: 1, page_size: 100, total: 1 },
  }),
}));

const LOCAL_USER: AdminUser = {
  id: 5,
  username: "vendor.pic",
  full_name: "Vendor PIC",
  email: "pic@vendor.com",
  role: "VENDOR-USER",
  is_karyawan: false,
  auth_source: "local",
  vendor_id: 1,
  is_active: true,
  deleted_at: null,
  supervisor_id: null,
  approval_level: null,
  last_login_at: null,
};

beforeEach(() => {
  createMutateAsync.mockReset();
  updateMutateAsync.mockReset();
});

describe("UserFormDialog — Password Sementara visibility (Task 8)", () => {
  it("shows Password Sementara in create mode when auth_source is local", async () => {
    const user = userEvent.setup();
    render(<UserFormDialog open onClose={vi.fn()} user={null} />);

    const select = screen.getByText("LDAP (internal)").closest("select");
    if (!select) throw new Error("auth_source select not found");
    await user.selectOptions(select, "local");

    expect(screen.getByText("Password Sementara")).toBeInTheDocument();
  });

  it("hides Password Sementara in create mode when auth_source is ldap (default)", () => {
    render(<UserFormDialog open onClose={vi.fn()} user={null} />);

    expect(screen.queryByText("Password Sementara")).not.toBeInTheDocument();
  });

  it("hides Password Sementara in edit mode, even for a local (vendor) user", () => {
    render(<UserFormDialog open onClose={vi.fn()} user={LOCAL_USER} />);

    expect(screen.queryByText("Password Sementara")).not.toBeInTheDocument();
  });

  it("disables Nama Pengguna and Auth Source fields in edit mode", () => {
    render(<UserFormDialog open onClose={vi.fn()} user={LOCAL_USER} />);

    expect(screen.getByDisplayValue(LOCAL_USER.username)).toBeDisabled();
    const authSourceSelect = screen.getByText("Local (vendor)").closest("select");
    expect(authSourceSelect).toBeDisabled();
  });
});

// ─── fe-coverage C2: submit paths ────────────────────────────────────────────

/** Field labels are <span>s (not <label>), so find the control next to the label text. */
function control(label: string): HTMLInputElement | HTMLSelectElement {
  const el = screen
    .getByText(label, { selector: "span" })
    .parentElement?.querySelector<HTMLInputElement | HTMLSelectElement>("input, select");
  if (!el) throw new Error(`control for "${label}" not found`);
  return el;
}

async function fillCommon(user: ReturnType<typeof userEvent.setup>) {
  await user.type(control("Nama Pengguna"), "budi.s");
  await user.type(control("Nama Lengkap"), "Budi Santoso");
  await user.type(control("Email"), "budi@cimb.local");
  await user.selectOptions(control("Role"), "ATM-USER");
  await user.type(control("Employee ID"), "E123");
}

describe("UserFormDialog — submit (fe-coverage C2)", () => {
  beforeEach(() => {
    useToastStore.setState({ toasts: [] });
  });

  it("creates an LDAP user without a temporary password, toasts and closes", async () => {
    createMutateAsync.mockResolvedValue({ id: 9 });
    const onClose = vi.fn();
    const user = userEvent.setup();
    render(<UserFormDialog open onClose={onClose} user={null} />);

    await fillCommon(user);
    await user.click(screen.getByRole("button", { name: "Simpan" }));

    await waitFor(() => expect(createMutateAsync).toHaveBeenCalledTimes(1));
    expect(createMutateAsync.mock.calls[0]?.[0]).toMatchObject({
      username: "budi.s",
      role: "ATM-USER",
      auth_source: "ldap",
      temporary_password: undefined,
      vendor_id: null,
      employee_id: "E123",
    });
    expect(useToastStore.getState().toasts[0]).toMatchObject({
      type: "success",
      message: "Pengguna berhasil dibuat",
    });
    expect(onClose).toHaveBeenCalled();
  });

  it("a local user needs a vendor and a temporary password before submitting", async () => {
    const user = userEvent.setup();
    render(<UserFormDialog open onClose={vi.fn()} user={null} />);

    await fillCommon(user);
    await user.selectOptions(control("Auth Source"), "local");
    await user.click(screen.getByRole("button", { name: "Simpan" }));

    expect(await screen.findAllByText("Wajib diisi untuk auth_source local")).toHaveLength(2);
    expect(createMutateAsync).not.toHaveBeenCalled();
  });

  it("creates a local vendor user with temporary password and vendor", async () => {
    createMutateAsync.mockResolvedValue({ id: 10 });
    const user = userEvent.setup();
    render(<UserFormDialog open onClose={vi.fn()} user={null} />);

    await fillCommon(user);
    await user.selectOptions(control("Auth Source"), "local");
    await user.type(control("Password Sementara"), "Temp#12345");
    await user.selectOptions(control("Vendor"), "1");
    await user.click(screen.getByRole("button", { name: "Simpan" }));

    await waitFor(() => expect(createMutateAsync).toHaveBeenCalledTimes(1));
    expect(createMutateAsync.mock.calls[0]?.[0]).toMatchObject({
      auth_source: "local",
      temporary_password: "Temp#12345",
      vendor_id: 1,
    });
  });

  it("updates an existing user without username/auth_source/password", async () => {
    updateMutateAsync.mockResolvedValue({ id: 5 });
    const onClose = vi.fn();
    const user = userEvent.setup();
    render(
      <UserFormDialog
        open
        onClose={onClose}
        user={{ ...LOCAL_USER, employee_id: "V-1" } as AdminUser}
      />,
    );

    await user.clear(control("Nama Lengkap"));
    await user.type(control("Nama Lengkap"), "Vendor PIC Baru");
    await user.click(screen.getByRole("button", { name: "Simpan" }));

    await waitFor(() => expect(updateMutateAsync).toHaveBeenCalledTimes(1));
    const { id, payload } = updateMutateAsync.mock.calls[0]?.[0] as {
      id: number;
      payload: Record<string, unknown>;
    };
    expect(id).toBe(5);
    expect(payload).toMatchObject({ full_name: "Vendor PIC Baru", vendor_id: 1 });
    expect(payload).not.toHaveProperty("username");
    expect(payload).not.toHaveProperty("auth_source");
    expect(payload).not.toHaveProperty("temporary_password");
    expect(useToastStore.getState().toasts[0]).toMatchObject({
      message: "Pengguna berhasil diperbarui",
    });
    expect(onClose).toHaveBeenCalled();
  });

  it("maps a server field error onto the field and keeps the dialog open", async () => {
    createMutateAsync.mockRejectedValue({
      status: 422,
      message: "Validasi gagal",
      details: [{ field: "username", message: "Username sudah dipakai" }],
    });
    const onClose = vi.fn();
    const user = userEvent.setup();
    render(<UserFormDialog open onClose={onClose} user={null} />);

    await fillCommon(user);
    await user.click(screen.getByRole("button", { name: "Simpan" }));

    expect(await screen.findByText("Username sudah dipakai")).toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();
  });

  it("shows a toast for a non-field server error and keeps the dialog open", async () => {
    createMutateAsync.mockRejectedValue({ status: 500, message: "Terjadi kesalahan internal" });
    const onClose = vi.fn();
    const user = userEvent.setup();
    render(<UserFormDialog open onClose={onClose} user={null} />);

    await fillCommon(user);
    await user.click(screen.getByRole("button", { name: "Simpan" }));

    await waitFor(() =>
      expect(useToastStore.getState().toasts[0]).toMatchObject({
        type: "error",
        message: "Terjadi kesalahan internal",
      }),
    );
    expect(onClose).not.toHaveBeenCalled();
  });
});
