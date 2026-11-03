import { MASTER_DATA_IO_SEARCH_SCHEMA } from "@/features/master-data-io/searchSchema";
import { useAuthStore } from "@/lib/auth/store";
import type { DbRole } from "@/lib/auth/store";
import { beforeEach, describe, expect, it } from "vitest";
import { requireRoles } from "../../_protected";

/**
 * Route guard + search schema for /settings/admin/master-data-io, same pattern as
 * vendors.test.ts. Pins the role list in src/routes/settings/admin/master-data-io.tsx.
 */
const MASTER_DATA_IO_ROLES: DbRole[] = ["ADMIN", "ADMIN_PARAM"];

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

describe("/settings/admin/master-data-io route guard", () => {
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

  it.each<DbRole>(["APPACCESS", "ATM-USER", "VENDOR-USER"])("denies %s", (role) => {
    setUser(role);
    expect(requireRoles(MASTER_DATA_IO_ROLES)()).toEqual({ forbidden: true });
  });

  it.each<DbRole>(["ADMIN", "ADMIN_PARAM"])("allows %s", (role) => {
    setUser(role);
    expect(requireRoles(MASTER_DATA_IO_ROLES)()).toBeUndefined();
  });

  it("redirects to /login when unauthenticated", () => {
    expect(() => requireRoles(MASTER_DATA_IO_ROLES)()).toThrow();
  });
});

describe("MASTER_DATA_IO_SEARCH_SCHEMA", () => {
  it("accepts every known param", () => {
    const input = {
      tab: "history",
      batch: 12,
      h_entity: "vendor-pics",
      h_status: "stale",
      h_mine: true,
      h_page: 2,
    };
    expect(MASTER_DATA_IO_SEARCH_SCHEMA.parse(input)).toEqual(input);
  });

  it("drops unknown keys", () => {
    expect(MASTER_DATA_IO_SEARCH_SCHEMA.parse({ tab: "new", evil: "x" })).toEqual({ tab: "new" });
  });

  it.each([
    { tab: "other" },
    { batch: 0 },
    { h_entity: "users" },
    { h_status: "done" },
    { h_page: 0 },
  ])("rejects invalid value %j", (input) => {
    expect(MASTER_DATA_IO_SEARCH_SCHEMA.safeParse(input).success).toBe(false);
  });
});
