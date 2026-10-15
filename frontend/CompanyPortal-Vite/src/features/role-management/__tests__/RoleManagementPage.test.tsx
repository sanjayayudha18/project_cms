import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

// fe-coverage C2: role list + selection. Query hooks mocked; the editor and the create
// dialog are stubbed (each has its own tests).
const q = vi.hoisted(() => ({
  roles: {} as Record<string, unknown>,
  catalog: {} as Record<string, unknown>,
}));

vi.mock("../hooks/useRoleQueries", () => ({
  useRoles: () => q.roles,
  useCatalog: () => q.catalog,
}));
vi.mock("../components/PermissionEditor", () => ({
  PermissionEditor: ({ role, catalog }: { role: { role: string }; catalog: unknown[] }) => (
    <div data-testid="editor">
      editor {role.role} ({catalog.length} menu)
    </div>
  ),
}));
vi.mock("../components/CreateRoleDialog", () => ({
  CreateRoleDialog: ({ open, onClose }: { open: boolean; onClose: () => void }) =>
    open ? (
      <dialog open>
        create
        <button type="button" onClick={onClose}>
          tutup
        </button>
      </dialog>
    ) : null,
}));

import { RoleManagementPage } from "../components/RoleManagementPage";

const roles = [
  { id: 1, role: "ADMIN", description: "Administrator", permissions: [1, 2, 3] },
  { id: 2, role: "AUDITOR", description: null, permissions: [] },
];

beforeEach(() => {
  q.roles = { isLoading: false, isError: false, data: { roles } };
  q.catalog = { isLoading: false, isError: false, data: { catalog: [{ id: 1 }, { id: 2 }] } };
});

describe("RoleManagementPage", () => {
  it("shows the loading message while either query loads", () => {
    q.catalog = { isLoading: true, isError: false, data: undefined };
    render(<RoleManagementPage />);

    expect(screen.getByText("Memuat daftar peran…")).toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });

  it("shows the error message when a query fails", () => {
    q.roles = { isLoading: false, isError: true, data: undefined };
    render(<RoleManagementPage />);

    expect(screen.getByText("Gagal memuat daftar peran. Coba lagi.")).toBeInTheDocument();
  });

  it("shows the empty message when there are no roles", () => {
    q.roles = { isLoading: false, isError: false, data: { roles: [] } };
    render(<RoleManagementPage />);

    expect(screen.getByText("Belum ada peran.")).toBeInTheDocument();
  });

  it("lists roles with description (or -) and permission count", () => {
    render(<RoleManagementPage />);

    expect(screen.getByText("Administrator")).toBeInTheDocument();
    expect(screen.getByText("-")).toBeInTheDocument();
    expect(screen.getByText("3")).toBeInTheDocument();
    expect(screen.getByText("Pilih peran untuk mengatur izin")).toBeInTheDocument();
    expect(screen.queryByTestId("editor")).not.toBeInTheDocument();
  });

  it("opens the permission editor for the chosen role with the catalog", async () => {
    render(<RoleManagementPage />);

    await userEvent
      .setup()
      .click(screen.getAllByRole("button", { name: "Atur Izin" })[1] as HTMLElement);

    expect(screen.getByText("Izin — AUDITOR")).toBeInTheDocument();
    expect(screen.getByTestId("editor")).toHaveTextContent("editor AUDITOR (2 menu)");
  });

  it("opens and closes the create-role dialog", async () => {
    const user = userEvent.setup();
    render(<RoleManagementPage />);

    await user.click(screen.getByRole("button", { name: "Buat Peran" }));
    expect(screen.getByRole("dialog")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "tutup" }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });
});
