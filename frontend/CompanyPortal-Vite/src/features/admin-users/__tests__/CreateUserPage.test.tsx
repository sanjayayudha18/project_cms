import { useToastStore } from "@/lib/hooks/useToast";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { CreateUserPage } from "../components/CreateUserPage";

// small-works 2026-11-20 — Tambah Pengguna on its own page (AC 3, 4).
const createMutateAsync = vi.fn();
const navigate = vi.fn();

vi.mock("../hooks", () => ({
  useCreateUser: () => ({ mutateAsync: createMutateAsync, isPending: false }),
  useUpdateUser: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useVendorOptions: () => ({ data: { vendors: [], branches: [] } }),
}));

vi.mock("@tanstack/react-router", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    useNavigate: () => navigate,
    Link: ({ to, children }: { to: string; children: React.ReactNode }) => (
      <a href={to}>{children}</a>
    ),
  };
});

const LIST = { to: "/settings/admin/users" };

beforeEach(() => {
  createMutateAsync.mockReset();
  navigate.mockReset();
  useToastStore.setState({ toasts: [] });
});

async function fillAndSave(user: ReturnType<typeof userEvent.setup>) {
  await user.type(screen.getByLabelText("Nama Pengguna"), "budi.s");
  await user.type(screen.getByLabelText("Nama Lengkap"), "Budi Santoso");
  await user.type(screen.getByLabelText("Email"), "budi@cimb.local");
  await user.selectOptions(screen.getByLabelText("Role"), "ATM-USER");
  await user.type(screen.getByLabelText("Employee ID"), "E123");
  await user.click(screen.getByRole("button", { name: "Simpan" }));
}

describe("CreateUserPage", () => {
  it("renders the form as a page with a back link to the list, not a dialog", () => {
    render(<CreateUserPage />);
    expect(screen.getByRole("heading", { name: "Tambah Pengguna" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Kembali ke Manajemen Pengguna/ })).toHaveAttribute(
      "href",
      "/settings/admin/users",
    );
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(screen.getByLabelText("Nama Pengguna")).toBeEnabled();
  });

  it("creates the user, toasts and returns to the list", async () => {
    createMutateAsync.mockResolvedValue({ id: 9 });
    const user = userEvent.setup();
    render(<CreateUserPage />);

    await fillAndSave(user);

    await waitFor(() => expect(navigate).toHaveBeenCalledWith(LIST));
    expect(createMutateAsync.mock.calls[0]?.[0]).toMatchObject({ username: "budi.s" });
    expect(useToastStore.getState().toasts[0]).toMatchObject({
      type: "success",
      message: "Pengguna berhasil dibuat",
    });
  });

  it("stays on the page with the server field error when save fails", async () => {
    createMutateAsync.mockRejectedValue({
      status: 422,
      message: "Validasi gagal",
      details: [{ field: "username", message: "Username sudah dipakai" }],
    });
    const user = userEvent.setup();
    render(<CreateUserPage />);

    await fillAndSave(user);

    expect(await screen.findByText("Username sudah dipakai")).toBeInTheDocument();
    expect(navigate).not.toHaveBeenCalled();
  });

  it("Batal returns to the list without saving", async () => {
    render(<CreateUserPage />);
    await userEvent.setup().click(screen.getByRole("button", { name: "Batal" }));
    expect(navigate).toHaveBeenCalledWith(LIST);
    expect(createMutateAsync).not.toHaveBeenCalled();
  });
});
