import { useAuthStore } from "@/lib/auth/store";
import type { DbRole } from "@/lib/auth/store";
import { beforeEach, describe, expect, it } from "vitest";
import { requireRoles } from "../../_protected";

/** Pins the role list /settings/admin/exports registers with (see exports.tsx). */
const ROLES: DbRole[] = ["ADMIN", "ADMIN_PARAM"];

function setUser(role: DbRole) {
  useAuthStore.setState({
    user: {
      id: 1,
      username: "test.user",
      fullName: "Test User",
      email: "test@cimb.local",
      role,
      isKaryawan: true,
      vendorId: null,
    },
    accessToken: "t",
    isAuthenticated: true,
    isAuthLoading: false,
    error: null,
    rateLimitRetryAfter: null,
  });
}

describe("/settings/admin/exports route guard", () => {
  beforeEach(() => {
    useAuthStore.setState({ user: null, accessToken: null, isAuthenticated: false });
  });
  it("denies a non-admin role", () => {
    setUser("APPACCESS");
    expect(requireRoles(ROLES)()).toEqual({ forbidden: true });
  });
  it.each(["ADMIN", "ADMIN_PARAM"] as const)("allows %s", (role) => {
    setUser(role);
    expect(requireRoles(ROLES)()).toBeUndefined();
  });
  it("redirects to /login when unauthenticated", () => {
    expect(() => requireRoles(ROLES)()).toThrow();
  });
});
