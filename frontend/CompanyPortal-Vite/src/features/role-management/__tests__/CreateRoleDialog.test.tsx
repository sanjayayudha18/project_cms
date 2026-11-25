import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { axe } from "vitest-axe";

// jsdom has no canvas, so axe cannot compute colour contrast; that is checked visually.
const NO_CONTRAST = { rules: { "color-contrast": { enabled: false } } };
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

  it("vendor portal: shows the locked VENDOR- prefix, title and scope radios", () => {
    render(<CreateRoleDialog open onClose={vi.fn()} portal="vendor" />);

    expect(screen.getByText("Buat Peran Portal Cabang")).toBeInTheDocument();
    expect(screen.getByText("VENDOR-")).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /Replenish/ })).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /Vault/ })).toBeInTheDocument();
  });

  it.each(["main", "vendor"] as const)("has no axe violations (%s)", async (portal) => {
    render(<CreateRoleDialog open onClose={vi.fn()} portal={portal} />);

    expect(await axe(document.body, NO_CONTRAST)).toHaveNoViolations();
  });

  it("vendor portal: requires a party scope", async () => {
    const user = userEvent.setup();
    render(<CreateRoleDialog open onClose={vi.fn()} portal="vendor" />);

    await user.type(screen.getByLabelText("Nama Peran"), "REPLENISH-CHECKER");
    await user.click(screen.getByRole("button", { name: "Buat Peran" }));

    expect(await screen.findByText("Pilih cakupan pihak")).toBeInTheDocument();
    expect(createMutateAsync).not.toHaveBeenCalled();
  });

  it("vendor portal: submits the prefixed name with the chosen scope", async () => {
    createMutateAsync.mockResolvedValue({ id: 2, role: "VENDOR-REPLENISH-CHECKER" });
    const user = userEvent.setup();
    render(<CreateRoleDialog open onClose={vi.fn()} portal="vendor" />);

    await user.type(screen.getByLabelText("Nama Peran"), "REPLENISH-CHECKER");
    await user.click(screen.getByRole("radio", { name: /Vault/ }));
    await user.click(screen.getByRole("button", { name: "Buat Peran" }));

    await waitFor(() =>
      expect(createMutateAsync).toHaveBeenCalledWith({
        role: "VENDOR-REPLENISH-CHECKER",
        description: "",
        portal: "vendor",
        party_scope: ["vault"],
      }),
    );
  });

  it("submits valid values and closes on success", async () => {
    createMutateAsync.mockResolvedValue({ id: 1, role: "AUDITOR-VIEW", description: null });
    const onClose = vi.fn();
    const user = userEvent.setup();
    render(<CreateRoleDialog open onClose={onClose} />);

    await user.type(screen.getByLabelText("Nama Peran"), "AUDITOR-VIEW");
    await user.click(screen.getByRole("button", { name: "Buat Peran" }));

    await waitFor(() =>
      expect(createMutateAsync).toHaveBeenCalledWith({
        role: "AUDITOR-VIEW",
        description: "",
        portal: "main",
      }),
    );
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });
});
