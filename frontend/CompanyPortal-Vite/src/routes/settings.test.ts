import { usePermissionStore } from "@/lib/auth/permissionStore";
import { useAuthStore } from "@/lib/auth/store";
import { beforeEach, describe, expect, it } from "vitest";
import { ForbiddenError } from "./_protected";
import { rbacDelegationsRoute } from "./settings/rbac/delegations";
import { rbacLeavesRoute } from "./settings/rbac/leaves";
import { rbacPoliciesRoute } from "./settings/rbac/policies";
import { rbacUsersRoute } from "./settings/rbac/users";

/**
 * Route guards for /settings/rbac/* (permission-catalog-coverage task 6.3): each route
 * calls requirePermission(<its screen key>). The hub itself is covered by settings.guard.test.ts.
 */

type Guard = () => Promise<void>;

function guardOf(route: unknown): Guard {
  return (route as { options: { beforeLoad: Guard } }).options.beforeLoad;
}

const SETTINGS_ROUTES = [
  { path: "/settings/rbac/users", route: rbacUsersRoute, key: "settings.rbac-users" },
  {
    path: "/settings/rbac/delegations",
    route: rbacDelegationsRoute,
    key: "settings.rbac-delegations",
  },
  { path: "/settings/rbac/leaves", route: rbacLeavesRoute, key: "settings.rbac-leaves" },
  { path: "/settings/rbac/policies", route: rbacPoliciesRoute, key: "settings.rbac-policies" },
];

function signIn() {
  useAuthStore.setState({
    user: {
      id: 1,
      username: "test.user",
      fullName: "Test User",
      email: "test@cimb.local",
      role: "ADMIN",
      isKaryawan: true,
      vendorId: null,
    },
    accessToken: "test-token",
    isAuthenticated: true,
    isAuthLoading: false,
    error: null,
    rateLimitRetryAfter: null,
  });
}

describe.each(SETTINGS_ROUTES)("settings route guard: $path", ({ route, key }) => {
  beforeEach(() => {
    signIn();
    usePermissionStore.setState({ keys: new Set(), status: "ready" });
  });

  it("allows a user holding the screen key", async () => {
    usePermissionStore.setState({ keys: new Set([key]), status: "ready" });
    await expect(guardOf(route)()).resolves.toBeUndefined();
  });

  it("denies a user without the screen key, even as ADMIN (no bypass)", async () => {
    await expect(guardOf(route)()).rejects.toBeInstanceOf(ForbiddenError);
  });

  it("redirects to /login when unauthenticated", async () => {
    useAuthStore.setState({ user: null, isAuthenticated: false });
    await expect(guardOf(route)()).rejects.toBeDefined();
  });
});
