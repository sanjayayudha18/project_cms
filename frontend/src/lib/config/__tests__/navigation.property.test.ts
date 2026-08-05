// Feature: frontend-consolidation, Property 9: RBAC Navigation Filtering Correctness
import type { Role } from "@/lib/auth/store";
import * as fc from "fast-check";
import { describe, expect, it } from "vitest";
import { NAV_CONFIG, type NavItem, filterNavByRoles } from "../navigation";

// ─── Constants ────────────────────────────────────────────────────────────────

const ALL_ROLES: Role[] = [
  "Admin",
  "ATM_Support",
  "Cash_Management",
  "Vendor",
  "WMO",
  "Finance",
  "Cash_Count_PIC",
  "Cash_Count_Lead",
  "Branch",
  "Approver",
];

// ─── Arbitraries ──────────────────────────────────────────────────────────────

/** Generates an arbitrary non-empty subset of roles (excludes Admin to test non-admin paths) */
const arbNonAdminRoles: fc.Arbitrary<Role[]> = fc
  .subarray(ALL_ROLES.filter((r) => r !== "Admin"), {
    minLength: 1,
    maxLength: ALL_ROLES.length - 1,
  })
  .map((roles) => [...roles]);

/** Generates an arbitrary subset of roles that always includes Admin */
const arbAdminRoles: fc.Arbitrary<Role[]> = fc
  .subarray(ALL_ROLES.filter((r) => r !== "Admin"), {
    minLength: 0,
    maxLength: ALL_ROLES.length - 1,
  })
  .map((roles) => ["Admin" as Role, ...roles]);

/** Generates an arbitrary non-empty subset of all roles (may or may not include Admin) */
const arbAnyRoles: fc.Arbitrary<Role[]> = fc
  .subarray(ALL_ROLES, { minLength: 1, maxLength: ALL_ROLES.length })
  .map((roles) => [...roles]);

/** Generates an arbitrary NavItem for testing */
const arbNavItem: fc.Arbitrary<NavItem> = fc
  .record({
    id: fc.string({ minLength: 1, maxLength: 20 }),
    label: fc.string({ minLength: 1, maxLength: 30 }),
    href: fc.string({ minLength: 1, maxLength: 50 }).map((s) => `/${s}`),
    roles: fc.oneof(
      fc.constant(["*"] as (Role | "*")[]),
      fc
        .subarray(ALL_ROLES, { minLength: 1, maxLength: ALL_ROLES.length })
        .map((r) => r as (Role | "*")[]),
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

// ─── Property 9: RBAC Navigation Filtering Correctness ───────────────────────

/**
 * **Validates: Requirements 10.3**
 *
 * For any user role combination and the complete NAV_CONFIG array,
 * `filterNavByRoles` SHALL return only entries where at least one of the user's
 * roles appears in the entry's `roles` array (or the entry has `roles: ["*"]`),
 * and Admin users SHALL see all entries regardless of role restrictions.
 */
describe("Property 9: RBAC Navigation Filtering Correctness", () => {
  it("returned entries match user roles — every returned item has at least one role in common with user", () => {
    fc.assert(
      fc.property(arbNavItems, arbNonAdminRoles, (items, userRoles) => {
        const result = filterNavByRoles(items, userRoles);

        for (const item of result) {
          const hasWildcard = item.roles.includes("*");
          const hasMatchingRole = item.roles.some((role) =>
            userRoles.includes(role as Role),
          );
          expect(hasWildcard || hasMatchingRole).toBe(true);
        }
      }),
      { numRuns: 200 },
    );
  });

  it("Admin sees all entries regardless of role restrictions", () => {
    fc.assert(
      fc.property(arbNavItems, arbAdminRoles, (items, userRoles) => {
        const result = filterNavByRoles(items, userRoles);
        expect(result).toEqual(items);
      }),
      { numRuns: 200 },
    );
  });

  it("empty roles returns empty array", () => {
    fc.assert(
      fc.property(arbNavItems, (items) => {
        const result = filterNavByRoles(items, []);
        expect(result).toHaveLength(0);
      }),
      { numRuns: 100 },
    );
  });

  it("no permitted item is ever omitted (completeness)", () => {
    fc.assert(
      fc.property(arbNavItems, arbAnyRoles, (items, userRoles) => {
        const result = filterNavByRoles(items, userRoles);
        const isAdmin = userRoles.includes("Admin");

        for (const item of items) {
          const shouldBeIncluded =
            isAdmin ||
            item.roles.includes("*") ||
            item.roles.some((role) => userRoles.includes(role as Role));

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
      fc.property(arbNavItems, arbNonAdminRoles, (items, userRoles) => {
        const result = filterNavByRoles(items, userRoles);

        for (const item of items) {
          const shouldBeExcluded =
            !item.roles.includes("*") &&
            !item.roles.some((role) => userRoles.includes(role as Role));

          if (shouldBeExcluded) {
            expect(result).not.toContain(item);
          }
        }
      }),
      { numRuns: 200 },
    );
  });

  it("works correctly against the real NAV_CONFIG with arbitrary role combinations", () => {
    fc.assert(
      fc.property(arbAnyRoles, (userRoles) => {
        const result = filterNavByRoles(NAV_CONFIG, userRoles);

        if (userRoles.includes("Admin")) {
          expect(result).toEqual(NAV_CONFIG);
          return;
        }

        // Every returned item is justified
        for (const item of result) {
          const justified =
            item.roles.includes("*") ||
            item.roles.some((role) => userRoles.includes(role as Role));
          expect(justified).toBe(true);
        }

        // No missing items
        for (const item of NAV_CONFIG) {
          const shouldBeIn =
            item.roles.includes("*") ||
            item.roles.some((role) => userRoles.includes(role as Role));
          if (shouldBeIn) {
            expect(result).toContain(item);
          }
        }
      }),
      { numRuns: 200 },
    );
  });
});
