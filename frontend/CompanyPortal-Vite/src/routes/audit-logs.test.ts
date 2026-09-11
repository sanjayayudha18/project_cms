import { useAuthStore } from "@/lib/auth/store";
import type { DbRole } from "@/lib/auth/store";
import { beforeEach, describe, expect, it } from "vitest";
import { requireRoles } from "./_protected";

/**
 * Route guard for /audit-logs (Task 6 test checklist: "route guard denies
 * non-admin"). requireRoles' own generic behavior (redirect when
 * unauthenticated, ADMIN/ADMIN_PARAM bypass) is exhaustively covered by
 * _protected.property.test.ts; this only pins down the specific role list
 * this route registers with (["ADMIN", "ADMIN_PARAM"]), matching
 * src/routes/audit-logs.tsx.
 */
const AUDIT_LOG_ROLES: DbRole[] = ["ADMIN", "ADMIN_PARAM"];

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

describe("/audit-logs route guard", () => {
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

  it("denies a non-admin role", () => {
    setUser("ATM-USER");
    expect(requireRoles(AUDIT_LOG_ROLES)()).toEqual({ forbidden: true });
  });

  it("denies a branch role", () => {
    setUser("BRANCH-USER");
    expect(requireRoles(AUDIT_LOG_ROLES)()).toEqual({ forbidden: true });
  });

  it("allows ADMIN", () => {
    setUser("ADMIN");
    expect(requireRoles(AUDIT_LOG_ROLES)()).toBeUndefined();
  });

  it("allows ADMIN_PARAM", () => {
    setUser("ADMIN_PARAM");
    expect(requireRoles(AUDIT_LOG_ROLES)()).toBeUndefined();
  });

  it("redirects to /login when unauthenticated", () => {
    expect(() => requireRoles(AUDIT_LOG_ROLES)()).toThrow();
  });
});
