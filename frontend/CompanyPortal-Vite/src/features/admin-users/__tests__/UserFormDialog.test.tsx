import { render, screen } from "@testing-library/react";
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
  useVendorsList: () => ({ data: { vendors: [], page: 1, page_size: 100, total: 0 } }),
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
