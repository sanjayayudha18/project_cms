import { useToastStore } from "@/lib/hooks/useToast";
import { render, screen, waitFor, within } from "@testing-library/react";
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
  vendor_branch_id: null,
  vendor_branch_name: null,
  is_active: true,
  deleted_at: null,
  supervisor_id: null,
  approval_level: null,
  last_login_at: null,
};

const nav = vi.hoisted(() => ({ navigate: vi.fn(), search: {} as Record<string, unknown> }));
const useUsersListMock = vi.fn();
const disableMutate = vi.fn();
const enableMutate = vi.fn();

vi.mock("../hooks", () => ({
  useUsersList: (...args: unknown[]) => useUsersListMock(...args),
  useDisableUser: () => ({ mutate: disableMutate, isPending: false }),
  useEnableUser: () => ({ mutate: enableMutate, isPending: false }),
  useCreateUser: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpdateUser: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useVendorOptions: () => ({ data: { vendors: [], branches: [] } }),
}));

vi.mock("@tanstack/react-router", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    useNavigate: () => nav.navigate,
    useSearch: () => nav.search,
  };
});

beforeEach(() => {
  nav.navigate.mockReset();
  nav.search = {};
  useToastStore.setState({ toasts: [] });
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

// ─── fe-coverage C2: states, paging, filters, enable, toasts, form ───────────

const INACTIVE_USER: AdminUser = {
  ...ACTIVE_USER,
  id: 8,
  username: "sari",
  full_name: "Sari Dewi",
  is_active: false,
};

describe("AdminUsersPage — states and navigation (fe-coverage C2)", () => {
  it("shows the loading, error (retry) and empty states", async () => {
    const refetch = vi.fn();
    useUsersListMock.mockReturnValue({ isLoading: true, isError: false, data: undefined, refetch });
    const { rerender } = render(<AdminUsersPage />);
    expect(screen.queryByRole("table")).not.toBeInTheDocument();

    useUsersListMock.mockReturnValue({ isLoading: false, isError: true, data: undefined, refetch });
    rerender(<AdminUsersPage />);
    await userEvent.setup().click(screen.getByRole("button", { name: "Coba Lagi" }));
    expect(refetch).toHaveBeenCalled();

    useUsersListMock.mockReturnValue({
      isLoading: false,
      isError: false,
      data: { users: [], total: 0 },
      refetch,
    });
    rerender(<AdminUsersPage />);
    expect(
      screen.getByText("Tidak ada pengguna yang sesuai dengan filter saat ini"),
    ).toBeInTheDocument();
  });

  it("pages forward and back through the URL", async () => {
    nav.search = { page: 2 };
    useUsersListMock.mockReturnValue({
      isLoading: false,
      isError: false,
      data: { users: [ACTIVE_USER], total: 75 },
      refetch: vi.fn(),
    });
    const user = userEvent.setup();
    render(<AdminUsersPage />);

    expect(screen.getByText("Halaman 2 dari 3 (75 pengguna)")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Berikutnya" }));
    expect(nav.navigate).toHaveBeenLastCalledWith({ to: ".", search: { page: 3 } });
    await user.click(screen.getByRole("button", { name: "Sebelumnya" }));
    expect(nav.navigate).toHaveBeenLastCalledWith({ to: ".", search: {} });
  });

  it("toasts the server message after disabling, and an error when it fails", async () => {
    const user = userEvent.setup();
    disableMutate.mockImplementationOnce(
      (_id, opts: { onSuccess: (r: { message: string }) => void }) =>
        opts.onSuccess({ message: "Pengguna dinonaktifkan" }),
    );
    render(<AdminUsersPage />);

    await user.click(screen.getByRole("button", { name: "Nonaktifkan" }));
    await user.click(
      within(screen.getByRole("dialog")).getByRole("button", { name: "Nonaktifkan" }),
    );
    expect(useToastStore.getState().toasts.at(-1)).toMatchObject({
      type: "success",
      message: "Pengguna dinonaktifkan",
    });

    disableMutate.mockImplementationOnce((_id, opts: { onError: (e: Error) => void }) =>
      opts.onError(new Error("Tidak boleh menonaktifkan diri sendiri")),
    );
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    await user.click(screen.getByRole("button", { name: "Nonaktifkan" }));
    await user.click(
      within(screen.getByRole("dialog")).getByRole("button", { name: "Nonaktifkan" }),
    );
    expect(useToastStore.getState().toasts.at(-1)).toMatchObject({
      type: "error",
      message: "Tidak boleh menonaktifkan diri sendiri",
    });
  });

  it("enables an inactive user after confirmation", async () => {
    useUsersListMock.mockReturnValue({
      isLoading: false,
      isError: false,
      data: { users: [INACTIVE_USER], total: 1 },
      refetch: vi.fn(),
    });
    const user = userEvent.setup();
    render(<AdminUsersPage />);

    await user.click(screen.getByRole("button", { name: "Aktifkan" }));
    const dialog = screen.getByRole("dialog");
    expect(dialog).toHaveTextContent('Aktifkan kembali akun "Sari Dewi"?');
    await user.click(within(dialog).getByRole("button", { name: "Aktifkan" }));

    expect(enableMutate).toHaveBeenCalledWith(8, expect.anything());
  });

  it("opens the user form for create and for edit", async () => {
    const user = userEvent.setup();
    render(<AdminUsersPage />);

    await user.click(screen.getByRole("button", { name: "Tambah Pengguna" }));
    expect(screen.getByRole("dialog", { name: "Tambah Pengguna" })).toBeInTheDocument();
    await user.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Batal" }));

    await user.click(screen.getByRole("button", { name: "Ubah" }));
    expect(await screen.findByRole("dialog", { name: "Ubah Pengguna" })).toBeInTheDocument();
    expect(screen.getByDisplayValue("budi.santoso")).toBeDisabled();
  });

  it("changing a filter resets to page 1", async () => {
    nav.search = { page: 4 };
    render(<AdminUsersPage />);

    await userEvent.setup().selectOptions(screen.getByLabelText("Status"), "all");

    expect(nav.navigate).toHaveBeenLastCalledWith({ to: ".", search: { status: "all" } });
  });
});
