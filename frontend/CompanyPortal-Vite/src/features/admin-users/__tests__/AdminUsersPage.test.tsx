import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AdminUsersPage } from "../components/AdminUsersPage";
import type { AdminUser } from "../types";

const ACTIVE_USER: AdminUser = {
  id: 7,
  username: "budi.santoso",
  full_name: "Budi Santoso",
  email: "budi@example.com",
  role: "APPACCESS",
  is_karyawan: true,
  auth_source: "ldap",
  vendor_id: null,
  is_active: true,
  deleted_at: null,
  supervisor_id: null,
  approval_level: null,
  last_login_at: null,
};

const useUsersListMock = vi.fn();
const disableMutate = vi.fn();
const enableMutate = vi.fn();

vi.mock("../hooks", () => ({
  useUsersList: (...args: unknown[]) => useUsersListMock(...args),
  useDisableUser: () => ({ mutate: disableMutate, isPending: false }),
  useEnableUser: () => ({ mutate: enableMutate, isPending: false }),
  useCreateUser: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpdateUser: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

vi.mock("@/features/admin-vendors/hooks", () => ({
  useVendorsList: () => ({ data: { vendors: [], page: 1, page_size: 100, total: 0 } }),
}));

vi.mock("@tanstack/react-router", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    useNavigate: () => vi.fn(),
    useSearch: () => ({}),
  };
});

beforeEach(() => {
  useUsersListMock.mockReset();
  disableMutate.mockReset();
  enableMutate.mockReset();
  useUsersListMock.mockReturnValue({
    data: { users: [ACTIVE_USER], page: 1, page_size: 25, total: 1 },
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  });
});

describe("AdminUsersPage — disable confirmation flow (Task 8)", () => {
  it("opens a confirmation dialog before disabling, and only calls the mutation on confirm", async () => {
    const user = userEvent.setup();
    render(<AdminUsersPage />);

    await user.click(screen.getByRole("button", { name: "Nonaktifkan" }));

    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByText(/Nonaktifkan akun "Budi Santoso"\?/)).toBeInTheDocument();
    expect(disableMutate).not.toHaveBeenCalled();

    await user.click(within(dialog).getByRole("button", { name: "Nonaktifkan" }));

    expect(disableMutate).toHaveBeenCalledWith(ACTIVE_USER.id, expect.anything());
  });

  it("cancel closes the dialog without calling the mutation", async () => {
    const user = userEvent.setup();
    render(<AdminUsersPage />);

    await user.click(screen.getByRole("button", { name: "Nonaktifkan" }));
    await user.click(screen.getByRole("button", { name: "Batal" }));

    expect(disableMutate).not.toHaveBeenCalled();
    expect(screen.queryByText(/Nonaktifkan akun/)).not.toBeInTheDocument();
  });
});
