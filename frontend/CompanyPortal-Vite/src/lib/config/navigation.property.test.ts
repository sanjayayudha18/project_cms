// Feature: user-login, Property 13: Navigation Filtering by Role
import type { DbRole } from "@/lib/auth/store";
import * as fc from "fast-check";
import { describe, expect, it } from "vitest";
import { NAV_CONFIG, type NavItem, filterNavByRoles } from "./navigation";

// ─── Constants ────────────────────────────────────────────────────────────────

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

const ADMIN_ROLES: DbRole[] = ["ADMIN", "ADMIN_PARAM"];
const NON_ADMIN_ROLES: DbRole[] = ALL_DB_ROLES.filter(
  (r) => !ADMIN_ROLES.includes(r),
);

// ─── Arbitraries ──────────────────────────────────────────────────────────────

/** Generates any single DbRole */
const arbDbRole: fc.Arbitrary<DbRole> = fc.constantFrom(...ALL_DB_ROLES);

/** Generates a non-admin DbRole */
const arbNonAdminRole: fc.Arbitrary<DbRole> = fc.constantFrom(...NON_ADMIN_ROLES);

/** Generates an admin DbRole (ADMIN or ADMIN_PARAM) */
const arbAdminRole: fc.Arbitrary<DbRole> = fc.constantFrom(...ADMIN_ROLES);

/** Generates an arbitrary NavItem with random DB role assignments */
const arbNavItem: fc.Arbitrary<NavItem> = fc
  .record({
    id: fc.string({ minLength: 1, maxLength: 20 }),
    label: fc.string({ minLength: 1, maxLength: 30 }),
    href: fc.string({ minLength: 1, maxLength: 50 }).map((s) => `/${s}`),
    roles: fc.oneof(
      fc.constant(["*"] as (DbRole | "*")[]),
      fc
        .subarray(ALL_DB_ROLES, { minLength: 1, maxLength: ALL_DB_ROLES.length })
        .map((r) => r as (DbRole | "*")[]),
    ),
    group: fc.constantFrom(
      "general" as const,
      "forecasting" as const,
      "invoice" as const,
      "cash-count" as const,
    ),
    disabled: fc.boolean(),
  })
  .map((record) => ({
    ...record,
    icon: (() => null) as unknown as NavItem["icon"],
  }));

/** Generates an arbitrary array of NavItems */
const arbNavItems: fc.Arbitrary<NavItem[]> = fc.array(arbNavItem, {
  minLength: 1,
  maxLength: 25,
});

// ─── Property 13: Navigation Filtering by Role ───────────────────────────────

/**
 * **Validates: Requirements 11.2, 11.4, 11.5, 11.6**
 *
 * For any user role R and navigation configuration, the filterNavByRoles
 * function SHALL return only items where R is in the item's allowed roles list
 * or the item has wildcard "*" access. For ADMIN and ADMIN_PARAM roles, all
 * items SHALL be returned regardless of their allowed roles list.
 */
describe("Property 13: Navigation Filtering by Role", () => {
  it("for any role R, returned items have R in allowed list or wildcard '*'", () => {
    fc.assert(
      fc.property(arbNavItems, arbNonAdminRole, (items, userRole) => {
        const result = filterNavByRoles(items, userRole);

        for (const item of result) {
          const hasWildcard = item.roles.includes("*");
          const hasRole = item.roles.includes(userRole);
          expect(hasWildcard || hasRole).toBe(true);
        }
      }),
      { numRuns: 200 },
    );
  });

  it("ADMIN and ADMIN_PARAM see all items (admin override)", () => {
    fc.assert(
      fc.property(arbNavItems, arbAdminRole, (items, adminRole) => {
        const result = filterNavByRoles(items, adminRole);
        expect(result).toEqual(items);
      }),
      { numRuns: 200 },
    );
  });

  it("no permitted item is ever omitted (completeness)", () => {
    fc.assert(
      fc.property(arbNavItems, arbDbRole, (items, userRole) => {
        const result = filterNavByRoles(items, userRole);
        const isAdmin = ADMIN_ROLES.includes(userRole);

        for (const item of items) {
          const shouldBeIncluded =
            isAdmin ||
            item.roles.includes("*") ||
            item.roles.includes(userRole);

          if (shouldBeIncluded) {
            expect(result).toContain(item);
          }
        }
      }),
      { numRuns: 200 },
    );
  });

  it("no unpermitted item is ever included (soundness)", () => {
    fc.assert(
      fc.property(arbNavItems, arbNonAdminRole, (items, userRole) => {
        const result = filterNavByRoles(items, userRole);

        for (const item of items) {
          const shouldBeExcluded =
            !item.roles.includes("*") && !item.roles.includes(userRole);

          if (shouldBeExcluded) {
            expect(result).not.toContain(item);
          }
        }
      }),
      { numRuns: 200 },
    );
  });

  it("wildcard items are always included for any role", () => {
    fc.assert(
      fc.property(arbDbRole, (userRole) => {
        const wildcardItem: NavItem = {
          id: "wildcard-test",
          label: "Wildcard",
          icon: (() => null) as unknown as NavItem["icon"],
          href: "/wildcard",
          roles: ["*"],
          group: "general",
        };
        const result = filterNavByRoles([wildcardItem], userRole);
        expect(result).toContain(wildcardItem);
      }),
      { numRuns: 100 },
    );
  });

  it("works correctly against the real NAV_CONFIG", () => {
    fc.assert(
      fc.property(arbDbRole, (userRole) => {
        const result = filterNavByRoles(NAV_CONFIG, userRole);
        const isAdmin = ADMIN_ROLES.includes(userRole);

        if (isAdmin) {
          expect(result).toEqual(NAV_CONFIG);
          return;
        }

        // Every returned item is justified
        for (const item of result) {
          const justified =
            item.roles.includes("*") || item.roles.includes(userRole);
          expect(justified).toBe(true);
        }

        // No missing items
        for (const item of NAV_CONFIG) {
          const shouldBeIn =
            item.roles.includes("*") || item.roles.includes(userRole);
          if (shouldBeIn) {
            expect(result).toContain(item);
          }
        }
      }),
      { numRuns: 100 },
    );
  });
});
