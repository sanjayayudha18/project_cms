import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { axe } from "vitest-axe";

// jsdom has no canvas, so axe cannot compute colour contrast; that is checked visually.
const NO_CONTRAST = { rules: { "color-contrast": { enabled: false } } };

// Task 6.2: tabs, URL state, master-detail, search. Hooks mocked; the editor and the create
// dialog are stubbed (each has its own tests). Full 6.4 suite (axe etc.) comes later.
const q = vi.hoisted(() => ({
  roles: {} as Record<string, unknown>,
  catalog: {} as Record<string, unknown>,
  params: { portal: "main", role: null, q: "" } as {
    portal: "main" | "vendor";
    role: number | null;
    q: string;
  },
  setParams: vi.fn(),
  catalogFor: vi.fn(),
}));

vi.mock("../hooks/useRoleQueries", () => ({
  useRoles: () => q.roles,
  useCatalogForPortal: (portal: string) => {
    q.catalogFor(portal);
    return q.catalog;
  },
}));
vi.mock("../useRoleManagementUrlState", () => ({
  useRoleManagementUrlState: () => ({ params: q.params, setParams: q.setParams }),
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
vi.mock("@tanstack/react-router", () => ({
  Link: ({ to, children }: { to: string; children: import("react").ReactNode }) => (
    <a href={to}>{children}</a>
  ),
}));

import { RoleManagementPage } from "../components/RoleManagementPage";

const role = (id: number, name: string, portal: "main" | "vendor", description: string | null) => ({
  id,
  role: name,
  description,
  portal,
  party_scope: portal === "vendor" ? ["replenish"] : null,
  active_user_count: id * 2,
  permissions: [],
});

const roles = [
  role(1, "ADMIN", "main", "Administrator sistem"),
  role(2, "AUDITOR", "main", null),
  role(3, "VENDOR-ATM-USER", "vendor", "Pihak replenish"),
];

beforeEach(() => {
  q.params = { portal: "main", role: null, q: "" };
  q.setParams.mockReset();
  q.catalogFor.mockReset();
  q.roles = { isLoading: false, isError: false, data: { roles }, refetch: vi.fn() };
  q.catalog = { isLoading: false, isError: false, data: [{ id: 1 }, { id: 2 }], refetch: vi.fn() };
});

describe("RoleManagementPage", () => {
  it("shows the loading message while a query loads", () => {
    q.catalog = { isLoading: true, isError: false, data: undefined, refetch: vi.fn() };
    render(<RoleManagementPage />);

    expect(screen.getByText("Memuat daftar peran…")).toBeInTheDocument();
  });

  it("shows the error message with a retry button that refetches", async () => {
    const refetch = vi.fn();
    q.roles = { isLoading: false, isError: true, data: undefined, refetch };
    render(<RoleManagementPage />);

    expect(screen.getByText("Gagal memuat daftar peran. Coba lagi.")).toBeInTheDocument();
    await userEvent.setup().click(screen.getByRole("button", { name: "Coba lagi" }));
    expect(refetch).toHaveBeenCalled();
  });

  it("shows a per-tab empty message", () => {
    q.params = { portal: "vendor", role: null, q: "" };
    q.roles = {
      isLoading: false,
      isError: false,
      data: { roles: roles.slice(0, 2) },
      refetch: vi.fn(),
    };
    render(<RoleManagementPage />);

    expect(screen.getByText("Belum ada peran di Portal Cabang.")).toBeInTheDocument();
  });

  it("renders tabs with counts; the URL portal selects the tab", () => {
    render(<RoleManagementPage />);

    const tabs = screen.getAllByRole("tab");
    expect(tabs[0]).toHaveAttribute("aria-selected", "true");
    expect(tabs[0]).toHaveTextContent("Portal Utama");
    expect(tabs[0]).toHaveTextContent("2");
    expect(tabs[1]).toHaveAttribute("aria-selected", "false");
    expect(tabs[1]).toHaveTextContent("1");
  });

  it("changes portal via click and arrow keys, clearing role and search", async () => {
    const user = userEvent.setup();
    render(<RoleManagementPage />);

    await user.click(screen.getByRole("tab", { name: /Portal Cabang/ }));
    expect(q.setParams).toHaveBeenLastCalledWith({ portal: "vendor", role: null, q: "" });

    q.setParams.mockClear();
    screen.getByRole("tab", { name: /Portal Utama/ }).focus();
    await user.keyboard("{ArrowRight}");
    expect(q.setParams).toHaveBeenLastCalledWith({ portal: "vendor", role: null, q: "" });
  });

  it("lists only the active portal's roles and auto-selects the first", () => {
    render(<RoleManagementPage />);

    const list = screen.getByRole("list");
    expect(within(list).getByText("ADMIN")).toBeInTheDocument();
    expect(within(list).getByText("Administrator sistem")).toBeInTheDocument();
    expect(within(list).getByText("Belum ada deskripsi")).toBeInTheDocument();
    expect(screen.queryByText("VENDOR-ATM-USER")).not.toBeInTheDocument();
    expect(screen.getByTestId("editor")).toHaveTextContent("editor ADMIN (2 menu)");
  });

  it("selects the role named in the URL and writes the choice on click", async () => {
    q.params = { portal: "main", role: 2, q: "" };
    const user = userEvent.setup();
    render(<RoleManagementPage />);

    expect(screen.getByTestId("editor")).toHaveTextContent("editor AUDITOR");
    await user.click(within(screen.getByRole("list")).getByRole("button", { name: /ADMIN/ }));
    expect(q.setParams).toHaveBeenLastCalledWith({ role: 1 });
  });

  it("filters by name or description, case-insensitively", () => {
    q.params = { portal: "main", role: null, q: "SISTEM" };
    render(<RoleManagementPage />);

    const list = screen.getByRole("list");
    expect(within(list).getByText("ADMIN")).toBeInTheDocument();
    expect(within(list).queryByText("AUDITOR")).not.toBeInTheDocument();
  });

  it("shows 'tidak ada hasil' with a clear button when nothing matches", async () => {
    q.params = { portal: "main", role: null, q: "zzz" };
    render(<RoleManagementPage />);

    expect(screen.getByText(/Tidak ada peran yang cocok/)).toBeInTheDocument();
    expect(screen.queryByTestId("editor")).not.toBeInTheDocument();
    await userEvent.setup().click(screen.getByRole("button", { name: "Hapus pencarian" }));
    expect(q.setParams).toHaveBeenCalledWith({ q: "" });
  });

  it("shows scope chips with text for vendor roles", () => {
    q.params = { portal: "vendor", role: null, q: "" };
    render(<RoleManagementPage />);

    expect(within(screen.getByRole("list")).getByText("Replenish")).toBeInTheDocument();
  });

  it("loads only the active portal's catalog for the editor", () => {
    q.params = { portal: "vendor", role: null, q: "" };
    render(<RoleManagementPage />);

    expect(q.catalogFor).toHaveBeenCalledWith("vendor");
    expect(q.catalogFor).not.toHaveBeenCalledWith("main");
  });

  it.each(["main", "vendor"] as const)("has no axe violations on the %s tab", async (portal) => {
    q.params = { portal, role: null, q: "" };
    const { container } = render(<RoleManagementPage />);

    expect(await axe(container, NO_CONTRAST)).toHaveNoViolations();
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
