import type { AuthUser } from "@/lib/types";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  Outlet,
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

// role-management-revamp 8.4, Property 9: sidebar = held keys ∪ {Notifikasi}, for every key set.
let mockUser: AuthUser | null = null;
vi.mock("@/features/auth/useAuth", () => ({
  useAuth: () => ({ state: { user: mockUser }, logout: vi.fn() }),
}));
vi.mock("@/features/notifications/useNotifications", () => ({
  useUnreadCount: () => ({ data: 0 }),
}));

import { AppShell } from "@/app/AppShell";

const ITEMS = [
  { key: "vendor-dsr", label: /DSR Menu/ },
  { key: "vendor-orders", label: /CIT Orders/ },
  { key: "vendor-realizations", label: /Realisasi/ },
] as const;

// All 2^3 permission subsets as bitmasks (exhaustive, so no generator needed).
const MASKS = [0, 1, 2, 3, 4, 5, 6, 7];

function renderShell(permissions: string[]) {
  mockUser = {
    id: 1,
    username: "v",
    fullName: "Budi",
    email: "b@v.com",
    role: "VENDOR-USER",
    isKaryawan: false,
    vendorId: 1,
    vendorName: "Gardanet",
    vendorBranchId: null,
    vendorBranchName: null,
    partyScope: ["replenish", "vault"],
    permissions,
  } as AuthUser;
  const root = createRootRoute({
    component: () => (
      <QueryClientProvider client={new QueryClient()}>
        <AppShell>
          <Outlet />
        </AppShell>
      </QueryClientProvider>
    ),
  });
  const index = createRoute({ path: "/", getParentRoute: () => root, component: () => null });
  const router = createRouter({
    routeTree: root.addChildren([index]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
  });
  render(<RouterProvider router={router} />);
}

describe("Property 9: VendorPortal sidebar = permissions ∪ Notifikasi", () => {
  it.each(MASKS)("permission subset bitmask %i", async (mask) => {
    const held = ITEMS.filter((_, i) => mask & (1 << i));
    renderShell(held.map((h) => h.key));
    expect(await screen.findByRole("link", { name: /Notifications/ })).toBeInTheDocument();
    for (const item of ITEMS) {
      const link = screen.queryByRole("link", { name: item.label });
      if (held.includes(item)) expect(link).toBeInTheDocument();
      else expect(link).not.toBeInTheDocument();
    }
    expect(screen.getAllByRole("link")).toHaveLength(held.length + 1);
  });
});
