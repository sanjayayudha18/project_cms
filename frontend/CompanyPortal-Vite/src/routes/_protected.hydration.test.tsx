import { usePermissionStore } from "@/lib/auth/permissionStore";
import { useAuthStore } from "@/lib/auth/store";
import type { AuthUser } from "@/lib/auth/store";
import { render, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { rootRoute } from "./__root";
import { ForbiddenError, requirePermission } from "./_protected";

const getMock = vi.fn();
vi.mock("@/lib/api/client", () => ({ api: { get: (...a: unknown[]) => getMock(...a) } }));

/**
 * Hard-load regression: on a refresh / pasted URL the session is still being restored from the
 * refresh cookie (isAuthLoading = true, user = null) when the route guards first run. The guard
 * used to throw redirect("/login") there, so a signed-in user ended up on "/" instead of the
 * page they asked for.
 */

const invalidateMock = vi.fn();

vi.mock("@tanstack/react-router", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return { ...actual, useRouter: () => ({ invalidate: invalidateMock }), Outlet: () => null };
});

const ATM_USER: AuthUser = {
  id: 1,
  username: "atm.user",
  fullName: "ATM User",
  email: "atm@example.com",
  role: "ATM-USER",
  isKaryawan: true,
  vendorId: null,
};

function setAuth(partial: Partial<ReturnType<typeof useAuthStore.getState>>) {
  useAuthStore.setState({
    user: null,
    accessToken: null,
    isAuthenticated: false,
    isAuthLoading: false,
    ...partial,
  });
}

beforeEach(() => invalidateMock.mockReset());

describe("requirePermission while the session is being restored", () => {
  beforeEach(() => usePermissionStore.getState().reset());

  it("neither redirects nor blocks while auth is loading, even though user is still null", async () => {
    setAuth({ isAuthLoading: true });

    await expect(requirePermission("settings.atms")()).resolves.toBeUndefined();
  });

  it("still redirects an unauthenticated user once loading has finished", async () => {
    setAuth({ isAuthLoading: false });

    await expect(requirePermission("settings.atms")()).rejects.toBeDefined();
  });

  it("applies the permission rules unchanged once the user is restored", async () => {
    getMock.mockResolvedValue({ data: { permissions: ["replenish.schedule"] } });
    setAuth({ user: ATM_USER, isAuthenticated: true });

    await expect(requirePermission("replenish.schedule")()).resolves.toBeUndefined();
    await expect(requirePermission("settings.atms")()).rejects.toBeInstanceOf(ForbiddenError);
  });
});

describe("RootComponent", () => {
  it("re-runs the route guards after the session has been restored", async () => {
    const initialize = vi.fn().mockResolvedValue(undefined);
    setAuth({ isAuthLoading: false, initialize });
    const Root = rootRoute.options.component as React.ComponentType;

    render(<Root />);

    await waitFor(() => expect(invalidateMock).toHaveBeenCalled());
    expect(initialize).toHaveBeenCalled();
    // invalidate must come after initialize resolved, not before
    expect(initialize.mock.invocationCallOrder[0]).toBeLessThan(
      invalidateMock.mock.invocationCallOrder[0] ?? Number.POSITIVE_INFINITY,
    );
  });
});
