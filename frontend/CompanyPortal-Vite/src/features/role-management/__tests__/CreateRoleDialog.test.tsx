import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { CreateRoleDialog } from "../components/CreateRoleDialog";

const createMutateAsync = vi.fn();

vi.mock("../hooks/useRoleQueries", () => ({
  useCreateRole: () => ({ mutateAsync: createMutateAsync, isPending: false }),
}));

beforeEach(() => {
  createMutateAsync.mockReset();
});

describe("CreateRoleDialog", () => {
  it("blocks submit and shows a validation error when the role name is empty (Req 7.3, client-side)", async () => {
    const user = userEvent.setup();
    render(<CreateRoleDialog open onClose={vi.fn()} />);

    await user.click(screen.getByRole("button", { name: "Buat Peran" }));

    expect(await screen.findByText("Nama peran wajib diisi")).toBeInTheDocument();
    expect(createMutateAsync).not.toHaveBeenCalled();
  });

  it("blocks submit for a lowercase/invalid-format role name", async () => {
    const user = userEvent.setup();
    render(<CreateRoleDialog open onClose={vi.fn()} />);

    await user.type(screen.getByLabelText("Nama Peran"), "not-uppercase!");
    await user.click(screen.getByRole("button", { name: "Buat Peran" }));

    expect(
      await screen.findByText("Gunakan huruf kapital, angka, dan tanda hubung"),
    ).toBeInTheDocument();
    expect(createMutateAsync).not.toHaveBeenCalled();
  });

  it("submits valid values and closes on success", async () => {
    createMutateAsync.mockResolvedValue({ id: 1, role: "AUDITOR-VIEW", description: null });
    const onClose = vi.fn();
    const user = userEvent.setup();
    render(<CreateRoleDialog open onClose={onClose} />);

    await user.type(screen.getByLabelText("Nama Peran"), "AUDITOR-VIEW");
    await user.click(screen.getByRole("button", { name: "Buat Peran" }));

    await waitFor(() =>
      expect(createMutateAsync).toHaveBeenCalledWith({ role: "AUDITOR-VIEW", description: "" }),
    );
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });
});
