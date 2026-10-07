import { useAuthStore } from "@/lib/auth/store";
import type { DbRole } from "@/lib/auth/store";
import { beforeEach, describe, expect, it } from "vitest";
import { settingsRoute } from "./settings";
import { rbacDelegationsRoute } from "./settings/rbac/delegations";
import { rbacLeavesRoute } from "./settings/rbac/leaves";
import { rbacPoliciesRoute } from "./settings/rbac/policies";
import { rbacUsersRoute } from "./settings/rbac/users";

/**
 * Route guards for /settings and /settings/rbac/* (Task 16.3, Requirements
 * 1.2, 1.3, 1.4, 3.2). Unlike audit-logs.test.ts (which duplicates the role
 * list), this calls each registered route's actual beforeLoad so a role-list
 * regression in any route file fails here directly. requireRoles' generic
 * behavior is exhaustively covered by _protected.property.test.ts.
 */

type GuardResult = { forbidden?: boolean } | undefined;
type Guard = () => GuardResult;

function guardOf(route: unknown): Guard {
  const beforeLoad = (route as { options: { beforeLoad: unknown } }).options.beforeLoad;
  return beforeLoad as Guard;
}

const SETTINGS_ROUTES: { path: string; route: unknown }[] = [
  { path: "/settings", route: settingsRoute },
  { path: "/settings/rbac/users", route: rbacUsersRoute },
  { path: "/settings/rbac/delegations", route: rbacDelegationsRoute },
  { path: "/settings/rbac/leaves", route: rbacLeavesRoute },
  { path: "/settings/rbac/policies", route: rbacPoliciesRoute },
];

const NON_AUTHORIZED_ROLES: DbRole[] = ["ATM-USER", "BRANCH-USER", "VENDOR-USER"];
const AUTHORIZED_ROLES: DbRole[] = ["ADMIN", "ADMIN_PARAM", "APPACCESS"];

function setUser(role: DbRole) {
  useAuthStore.setState({
    user: {
      id: 1,
      username: "test.user",
      fullName: "Test User",
      email: "test@cimb.local",
      role,
      isKaryawan: role !== "VENDOR-USER",
      vendorId: role === "VENDOR-USER" ? 1 : null,
    },
    accessToken: "test-token",
    isAuthenticated: true,
    isAuthLoading: false,
    error: null,
    rateLimitRetryAfter: null,
  });
}

describe.each(SETTINGS_ROUTES)("settings route guard: $path", ({ route }) => {
  beforeEach(() => {
    useAuthStore.setState({
      user: null,
      accessToken: null,
      isAuthenticated: false,
      isAuthLoading: false,
      error: null,
      rateLimitRetryAfter: null,
    });
  });

  it.each(NON_AUTHORIZED_ROLES)("denies non-authorized role %s", (role) => {
    setUser(role);
    expect(guardOf(route)()).toEqual({ forbidden: true });
  });

  it.each(AUTHORIZED_ROLES)("allows authorized role %s", (role) => {
    setUser(role);
    expect(guardOf(route)()).toBeUndefined();
  });

  it("redirects to /login when unauthenticated", () => {
    expect(() => guardOf(route)()).toThrow();
  });
});
