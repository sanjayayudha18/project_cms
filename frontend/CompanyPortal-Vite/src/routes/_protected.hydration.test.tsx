import { useAuthStore } from "@/lib/auth/store";
import type { AuthUser } from "@/lib/auth/store";
import { render, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { rootRoute } from "./__root";
import { requireRoles } from "./_protected";

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

describe("requireRoles while the session is being restored", () => {
  it("neither redirects nor blocks while auth is loading, even though user is still null", () => {
    setAuth({ isAuthLoading: true });

    expect(() => requireRoles(["ADMIN"])()).not.toThrow();
    expect(requireRoles(["ADMIN"])()).toBeUndefined();
  });

  it("still redirects an unauthenticated user once loading has finished", () => {
    setAuth({ isAuthLoading: false });

    expect(() => requireRoles(["ADMIN"])()).toThrow();
  });

  it("applies the role rules unchanged once the user is restored", () => {
    setAuth({ user: ATM_USER, isAuthenticated: true });

    expect(requireRoles(["ATM-USER"])()).toBeUndefined();
    expect(requireRoles(["BRANCH-USER"])()).toEqual({ forbidden: true });
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
