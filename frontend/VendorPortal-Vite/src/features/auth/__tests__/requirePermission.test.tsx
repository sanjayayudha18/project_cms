import { RequirePermission } from "@/features/auth/RequirePermission";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

// role-management-revamp 8.2: route guard + NoAccess.
let mockPermissions: string[] = [];
const logout = vi.fn();
vi.mock("@/features/auth/useAuth", () => ({
  useAuth: () => ({ state: { user: { permissions: mockPermissions } }, logout }),
}));

function renderAt(path: string) {
  const root = createRootRoute();
  const guarded = (p: string, key: string) =>
    createRoute({
      path: p,
      getParentRoute: () => root,
      component: () => (
        <RequirePermission permissionKey={key}>
          <span>page {p}</span>
        </RequirePermission>
      ),
    });
  const tree = root.addChildren([
    guarded("/dsr", "vendor-dsr"),
    guarded("/orders", "vendor-orders"),
    guarded("/realizations", "vendor-realizations"),
  ]);
  const router = createRouter({
    routeTree: tree,
    history: createMemoryHistory({ initialEntries: [path] }),
  });
  render(<RouterProvider router={router} />);
}

describe("RequirePermission", () => {
  beforeEach(() => logout.mockClear());

  it("renders the page when the key is held", async () => {
    mockPermissions = ["vendor-orders"];
    renderAt("/orders");
    expect(await screen.findByText("page /orders")).toBeInTheDocument();
  });

  it("redirects to the first held nav item when the key is missing", async () => {
    mockPermissions = ["vendor-orders", "vendor-realizations"];
    renderAt("/dsr");
    expect(await screen.findByText("page /orders")).toBeInTheDocument();
  });

  it("shows NoAccess with a working logout when no key is held", async () => {
    mockPermissions = [];
    renderAt("/realizations");
    expect(
      await screen.findByRole("heading", { name: /belum memiliki akses/i }),
    ).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /keluar/i }));
    expect(logout).toHaveBeenCalledOnce();
  });
});
