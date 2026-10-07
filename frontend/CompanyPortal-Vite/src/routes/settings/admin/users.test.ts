import { useAuthStore } from "@/lib/auth/store";
import type { DbRole } from "@/lib/auth/store";
import { beforeEach, describe, expect, it } from "vitest";
import { requireRoles } from "../../_protected";

/**
 * Route guard for /settings/admin/users (Task 7 test checklist:
 * "requireRoles allow/deny/redirect"), pattern from
 * src/routes/audit-logs.test.ts. Pins down the role list this route
 * registers with (["APPACCESS"]), matching
 * src/routes/settings/admin/users.tsx.
 */
const ADMIN_USERS_ROLES: DbRole[] = ["APPACCESS"];

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

describe("/settings/admin/users route guard", () => {
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

  it("denies a non-APPACCESS role", () => {
    setUser("ATM-USER");
    expect(requireRoles(ADMIN_USERS_ROLES)()).toEqual({ forbidden: true });
  });

  it("allows APPACCESS", () => {
    setUser("APPACCESS");
    expect(requireRoles(ADMIN_USERS_ROLES)()).toBeUndefined();
  });

  it("allows ADMIN (bypasses all role checks)", () => {
    setUser("ADMIN");
    expect(requireRoles(ADMIN_USERS_ROLES)()).toBeUndefined();
  });

  it("allows ADMIN_PARAM (bypasses all role checks)", () => {
    setUser("ADMIN_PARAM");
    expect(requireRoles(ADMIN_USERS_ROLES)()).toBeUndefined();
  });

  it("redirects to /login when unauthenticated", () => {
    expect(() => requireRoles(ADMIN_USERS_ROLES)()).toThrow();
  });
});
