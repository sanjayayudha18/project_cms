import { useAuthStore } from "@/lib/auth/store";
import type { DbRole } from "@/lib/auth/store";
import { beforeEach, describe, expect, it } from "vitest";
import { requireRoles } from "../_protected";

/**
 * Route guard for /settings/roles, same pattern as
 * settings/admin/atms.test.ts. Pins down the role list this route
 * registers with (["APPACCESS"]), matching src/routes/settings/roles.tsx.
 * requireRoles() itself always bypasses ADMIN/ADMIN_PARAM regardless of the
 * list passed in (_protected.tsx) — that shared behavior is asserted here
 * too so a change to that helper doesn't silently widen this route's access.
 */
const ROLES_ROUTE_ROLES: DbRole[] = ["APPACCESS"];

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

describe("/settings/roles route guard", () => {
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

  it("denies a non-admin, non-APPACCESS role", () => {
    setUser("ATM-USER");
    expect(requireRoles(ROLES_ROUTE_ROLES)()).toEqual({ forbidden: true });
  });

  it("allows APPACCESS", () => {
    setUser("APPACCESS");
    expect(requireRoles(ROLES_ROUTE_ROLES)()).toBeUndefined();
  });

  it("allows ADMIN", () => {
    setUser("ADMIN");
    expect(requireRoles(ROLES_ROUTE_ROLES)()).toBeUndefined();
  });

  it("redirects to /login when unauthenticated", () => {
    expect(() => requireRoles(ROLES_ROUTE_ROLES)()).toThrow();
  });
});
