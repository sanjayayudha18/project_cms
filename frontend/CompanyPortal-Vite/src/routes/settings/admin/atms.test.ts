import { useAuthStore } from "@/lib/auth/store";
import type { DbRole } from "@/lib/auth/store";
import { beforeEach, describe, expect, it } from "vitest";
import { requireRoles } from "../../_protected";

/**
 * Route guard for /settings/admin/atms, same pattern as
 * users.test.ts / vendors.test.ts / audit-logs.test.ts. Pins down the role
 * list this route registers with (["ADMIN", "ADMIN_PARAM"]), matching
 * src/routes/settings/admin/atms.tsx.
 */
const ADMIN_ATMS_ROLES: DbRole[] = ["ADMIN", "ADMIN_PARAM"];

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

describe("/settings/admin/atms route guard", () => {
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
    setUser("APPACCESS");
    expect(requireRoles(ADMIN_ATMS_ROLES)()).toEqual({ forbidden: true });
  });

  it("allows ADMIN", () => {
    setUser("ADMIN");
    expect(requireRoles(ADMIN_ATMS_ROLES)()).toBeUndefined();
  });

  it("allows ADMIN_PARAM", () => {
    setUser("ADMIN_PARAM");
    expect(requireRoles(ADMIN_ATMS_ROLES)()).toBeUndefined();
  });

  it("redirects to /login when unauthenticated", () => {
    expect(() => requireRoles(ADMIN_ATMS_ROLES)()).toThrow();
  });
});
