import { usePermissionStore } from "@/lib/auth/permissionStore";
import { useAuthStore } from "@/lib/auth/store";
import type { AuthUser } from "@/lib/auth/store";
import {
  RouterProvider,
  createMemoryHistory,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { render, screen, waitFor } from "@testing-library/react";
import * as fc from "fast-check";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { rootRoute } from "./__root";
import { ForbiddenError, protectedRoute, requirePermission } from "./_protected";

/**
 * Property 9 (spec permission-catalog-coverage): a route guarded by requirePermission renders
 * only when the role's permission set holds one of its keys — no ADMIN bypass; otherwise 403
 * and the route component is never mounted.
 */

const get = vi.fn();
vi.mock("@/lib/api/client", () => ({ api: { get: (...a: unknown[]) => get(...a) } }));
vi.mock("@/components/layout/AppShell", () => ({
  AppShell: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}));
vi.mock("@/features/notifications", () => ({ NotificationBell: () => null }));

const mounted = vi.fn();

function user(role: AuthUser["role"]): AuthUser {
  return {
    id: 1,
    username: "u",
    fullName: "U",
    email: "u@example.com",
    role,
    isKaryawan: true,
    vendorId: null,
  };
}

function setAuth(partial: Partial<ReturnType<typeof useAuthStore.getState>>) {
  useAuthStore.setState({
    user: null,
    accessToken: null,
    isAuthenticated: false,
    isAuthLoading: false,
    initialize: vi.fn().mockResolvedValue(undefined),
    ...partial,
  });
}

function setPerms(permissions: string[]) {
  get.mockResolvedValue({ data: { permissions } });
}

function mountAt(key: string | string[], opts?: { roles?: AuthUser["role"][] }) {
  const page = createRoute({
    getParentRoute: () => protectedRoute,
    path: "/secret",
    beforeLoad: requirePermission(key, opts),
    component: () => {
      mounted();
      return <div>SECRET PAGE</div>;
    },
  });
  const login = createRoute({
    getParentRoute: () => rootRoute,
    path: "/login",
    component: () => <div>LOGIN PAGE</div>,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([protectedRoute.addChildren([page]), login]),
    history: createMemoryHistory({ initialEntries: ["/secret"] }),
  });
  render(<RouterProvider router={router} />);
  return router;
}

beforeEach(() => {
  window.scrollTo = vi.fn(); // jsdom: router scroll restoration
  useAuthStore.setState({ isAuthenticated: false });
  get.mockReset();
  mounted.mockReset();
  usePermissionStore.getState().reset();
});

describe("requirePermission", () => {
  it("renders the page when the role holds the key", async () => {
    setPerms(["settings.atms"]);
    setAuth({ user: user("ATM-SPV"), isAuthenticated: true });
    mountAt("settings.atms");
    expect(await screen.findByText("SECRET PAGE")).toBeTruthy();
  });

  it("any-of: one matching key is enough", async () => {
    setPerms(["settings.vendors"]);
    setAuth({ user: user("ATM-SPV"), isAuthenticated: true });
    mountAt(["settings.atms", "settings.vendors"]);
    expect(await screen.findByText("SECRET PAGE")).toBeTruthy();
  });

  it("without the key: 403 page and the component is never mounted", async () => {
    setPerms(["settings.vendors"]);
    setAuth({ user: user("ATM-SPV"), isAuthenticated: true });
    mountAt("settings.atms");
    expect(await screen.findByText(/403/)).toBeTruthy();
    expect(screen.getByRole("link", { name: /dashboard/i })).toBeTruthy();
    expect(screen.queryByText("SECRET PAGE")).toBeNull();
    expect(mounted).not.toHaveBeenCalled();
  });

  it("ADMIN gets no bypass", async () => {
    setPerms([]);
    setAuth({ user: user("ADMIN"), isAuthenticated: true });
    mountAt("settings.atms");
    expect(await screen.findByText(/403/)).toBeTruthy();
    expect(mounted).not.toHaveBeenCalled();
  });

  it("opts.roles narrows even when the key is held", async () => {
    setPerms(["settings.atms"]);
    setAuth({ user: user("ATM-SPV"), isAuthenticated: true });
    mountAt("settings.atms", { roles: ["ADMIN", "ADMIN_PARAM"] });
    expect(await screen.findByText(/403/)).toBeTruthy();
    expect(mounted).not.toHaveBeenCalled();
  });

  it("opts.roles allows a listed role that holds the key", async () => {
    setPerms(["settings.atms"]);
    setAuth({ user: user("ADMIN"), isAuthenticated: true });
    mountAt("settings.atms", { roles: ["ADMIN", "ADMIN_PARAM"] });
    expect(await screen.findByText("SECRET PAGE")).toBeTruthy();
  });

  it("not signed in → /login", async () => {
    setAuth({});
    const router = mountAt("settings.atms");
    await waitFor(() => expect(router.state.location.pathname).toBe("/login"));
    expect(mounted).not.toHaveBeenCalled();
  });

  it("permission load error → only Always_On keys pass", async () => {
    get.mockRejectedValue(new Error("boom"));
    setAuth({ user: user("ATM-SPV"), isAuthenticated: true });
    mountAt("settings.atms");
    expect(await screen.findByText(/403/)).toBeTruthy();
    expect(mounted).not.toHaveBeenCalled();
  });
});

describe("requirePermission — waiting", () => {
  it("returns without deciding while the session is restoring", async () => {
    setAuth({ isAuthLoading: true });
    await expect(requirePermission("settings.atms")()).resolves.toBeUndefined();
    expect(get).not.toHaveBeenCalled();
  });

  it("waits for a pending permission load before deciding", async () => {
    let release: (v: unknown) => void = () => {};
    get.mockReturnValue(
      new Promise((r) => {
        release = r;
      }),
    );
    setAuth({ user: user("ATM-SPV"), isAuthenticated: true });
    let settled = false;
    const p = requirePermission("settings.atms")().then(() => {
      settled = true;
    });
    await Promise.resolve();
    expect(settled).toBe(false);
    release({ data: { permissions: ["settings.atms"] } });
    await p;
    expect(settled).toBe(true);
  });
});

describe("Property 9", () => {
  const KEYS = ["settings.atms", "settings.vendors", "monitoring.audit-logs", "settings.pks"];
  const ROLES: AuthUser["role"][] = ["ADMIN", "ADMIN_PARAM", "ATM-SPV", "VENDOR-USER"];

  it("passes iff the permission set intersects the route keys (any role)", async () => {
    await fc.assert(
      fc.asyncProperty(
        fc.constantFrom(...ROLES),
        fc.subarray(KEYS),
        fc.subarray(KEYS, { minLength: 1 }),
        async (role, granted, required) => {
          usePermissionStore.getState().reset();
          setPerms(granted);
          setAuth({ user: user(role), isAuthenticated: true });
          const guard = requirePermission(required);
          const allowed = required.some((k) => granted.includes(k));
          if (allowed) await expect(guard()).resolves.toBeUndefined();
          else await expect(guard()).rejects.toBeInstanceOf(ForbiddenError);
        },
      ),
      { numRuns: 100 },
    );
  });
});
