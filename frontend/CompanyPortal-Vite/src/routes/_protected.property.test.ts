import { SCREEN_KEYS } from "@/lib/auth/permissionKeys";
import { usePermissionStore } from "@/lib/auth/permissionStore";
import { useAuthStore } from "@/lib/auth/store";
import type { DbRole } from "@/lib/auth/store";
import { NAV_CONFIG } from "@/lib/config/navigation";
import * as fc from "fast-check";
import { beforeEach, describe, expect, it } from "vitest";
import { ForbiddenError, requirePermission } from "./_protected";

/**
 * Property 3: Unauthenticated Route Protection (Requirements 2.1)
 * Property 4: Unauthorized Route Blocking (Requirements 3.6)
 *
 * Since permission-catalog-coverage the route guard is requirePermission(<key>):
 * no key => ForbiddenError (403 page), whatever the role, ADMIN included (no bypass).
 */

const ALL_DB_ROLES: DbRole[] = [
  "ADMIN",
  "ADMIN_PARAM",
  "ATM-USER",
  "ATM-SPV",
  "BRANCH-USER",
  "BRANCH-SPV",
  "BRANCH-ATM-USER",
  "BRANCH-ATM-SPV",
  "VENDOR-USER",
];

const NAV_KEYS = [
  ...new Set(
    NAV_CONFIG.flatMap((i) => (Array.isArray(i.permission) ? i.permission : [i.permission])),
  ),
] as string[];

function signIn(role: DbRole) {
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

function grant(keys: readonly string[]) {
  usePermissionStore.setState({ keys: new Set(keys), status: "ready" });
}

beforeEach(() => {
  useAuthStore.setState({
    user: null,
    accessToken: null,
    isAuthenticated: false,
    isAuthLoading: false,
    error: null,
    rateLimitRetryAfter: null,
  });
  grant([]);
});

describe("Route Protection — Property 3: Unauthenticated Route Protection", () => {
  it("requirePermission redirects (throws, not Forbidden) when there is no user, for any key", async () => {
    await fc.assert(
      fc.asyncProperty(fc.constantFrom(...SCREEN_KEYS), async (key) => {
        useAuthStore.setState({ user: null, isAuthenticated: false, isAuthLoading: false });
        const err = await requirePermission(key)().then(
          () => undefined,
          (e: unknown) => e,
        );
        expect(err).toBeDefined();
        expect(err).not.toBeInstanceOf(ForbiddenError);
      }),
      { numRuns: 50 },
    );
  });
});

describe("Route Protection — Property 4: Unauthorized Route Blocking", () => {
  it("any role without the key is forbidden, ADMIN and ADMIN_PARAM included", async () => {
    await fc.assert(
      fc.asyncProperty(
        fc.constantFrom(...NAV_KEYS),
        fc.constantFrom(...ALL_DB_ROLES),
        async (key, role) => {
          signIn(role);
          grant(SCREEN_KEYS.filter((k) => k !== key));
          await expect(requirePermission(key)()).rejects.toBeInstanceOf(ForbiddenError);
        },
      ),
      { numRuns: 100 },
    );
  });

  it("any role holding the key is not blocked", async () => {
    await fc.assert(
      fc.asyncProperty(
        fc.constantFrom(...NAV_KEYS),
        fc.constantFrom(...ALL_DB_ROLES),
        async (key, role) => {
          signIn(role);
          grant([key]);
          await expect(requirePermission(key)()).resolves.toBeUndefined();
        },
      ),
      { numRuns: 100 },
    );
  });

  it("an array of keys passes when the user holds any one of them", async () => {
    await fc.assert(
      fc.asyncProperty(fc.subarray(NAV_KEYS, { minLength: 1 }), async (keys) => {
        signIn("ATM-USER");
        grant([keys[0] as string]);
        await expect(requirePermission(keys)()).resolves.toBeUndefined();
      }),
      { numRuns: 50 },
    );
  });
});
