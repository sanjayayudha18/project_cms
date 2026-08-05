import type { Role } from "@/lib/auth/store";
import * as fc from "fast-check";
import { describe, expect, it } from "vitest";
import { NAV_CONFIG, type NavItem, filterNavByRoles } from "./navigation";

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

/** Generates a non-empty subset of roles */
const arbRoleSubset: fc.Arbitrary<Role[]> = fc
  .subarray(ALL_ROLES, { minLength: 1, maxLength: ALL_ROLES.length })
  .map((roles) => [...roles]);

/** Generates an arbitrary NavItem with random role assignments */
const arbNavItem: fc.Arbitrary<NavItem> = fc
  .record({
    id: fc.string({ minLength: 1, maxLength: 20 }),
    label: fc.string({ minLength: 1, maxLength: 30 }),
    href: fc.string({ minLength: 1, maxLength: 50 }).map((s) => `/${s}`),
    roles: fc.oneof(
      // Either wildcard or a non-empty subset of roles
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
    // Use a stub icon — not needed for filtering logic
    icon: (() => null) as unknown as NavItem["icon"],
  }));

/** Generates an arbitrary array of NavItems */
const arbNavItems: fc.Arbitrary<NavItem[]> = fc.array(arbNavItem, {
  minLength: 1,
  maxLength: 25,
});

// ─── Property 1: RBAC Navigation Filtering ───────────────────────────────────

/**
 * Property 1: RBAC Navigation Filtering
 * Validates: Requirements 3.1, 3.2, 3.3, 3.4, 3.5, 3.7
 *
 * For any user with a given set of roles, the navigation filter function SHALL
 * return only those nav items whose `roles` array intersects with the user's
 * roles (or contains the wildcard '*'). No item outside the user's role
 * permissions should ever appear, and no permitted item should ever be omitted.
 */
describe("filterNavByRoles — Property 1: RBAC Navigation Filtering", () => {
  it("returns only items whose roles intersect with user roles or contain '*' (Admin sees all)", () => {
    fc.assert(
      fc.property(arbNavItems, arbRoleSubset, (items, userRoles) => {
        const result = filterNavByRoles(items, userRoles);

        // Admin is a special case: it sees every item unconditionally.
        if (userRoles.includes("Admin")) {
          expect(result).toEqual(items);
          return;
        }

        for (const item of result) {
          const hasWildcard = item.roles.includes("*");
          const hasIntersection = item.roles.some((role) => userRoles.includes(role as Role));
          expect(hasWildcard || hasIntersection).toBe(true);
        }
      }),
    );
  });

  it("never omits an item that should be included (completeness)", () => {
    fc.assert(
      fc.property(arbNavItems, arbRoleSubset, (items, userRoles) => {
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
    );
  });

  it("never includes an item that should be excluded (soundness)", () => {
    fc.assert(
      fc.property(arbNavItems, arbRoleSubset, (items, userRoles) => {
        const result = filterNavByRoles(items, userRoles);
        const isAdmin = userRoles.includes("Admin");

        for (const item of items) {
          // Admin sees everything, so nothing is excluded for an Admin user.
          const shouldBeExcluded =
            !isAdmin &&
            !item.roles.includes("*") &&
            !item.roles.some((role) => userRoles.includes(role as Role));

          if (shouldBeExcluded) {
            expect(result).not.toContain(item);
          }
        }
      }),
    );
  });

  it("returns empty array when user has no roles", () => {
    fc.assert(
      fc.property(arbNavItems, (items) => {
        const result = filterNavByRoles(items, []);
        expect(result).toHaveLength(0);
      }),
    );
  });

  it("wildcard items are always included for any authenticated user", () => {
    fc.assert(
      fc.property(arbRoleSubset, (userRoles) => {
        const wildcardItem: NavItem = {
          id: "wildcard-test",
          label: "Wildcard",
          icon: (() => null) as unknown as NavItem["icon"],
          href: "/wildcard",
          roles: ["*"],
          group: "general",
        };
        const items = [wildcardItem];
        const result = filterNavByRoles(items, userRoles);

        expect(result).toContain(wildcardItem);
      }),
    );
  });

  it("works correctly against the real NAV_CONFIG", () => {
    fc.assert(
      fc.property(arbRoleSubset, (userRoles) => {
        const result = filterNavByRoles(NAV_CONFIG, userRoles);

        // Admin sees the entire config unconditionally.
        if (userRoles.includes("Admin")) {
          expect(result).toEqual(NAV_CONFIG);
          return;
        }

        // Every result item is justified
        for (const item of result) {
          const justified =
            item.roles.includes("*") || item.roles.some((role) => userRoles.includes(role as Role));
          expect(justified).toBe(true);
        }

        // No missing items
        for (const item of NAV_CONFIG) {
          const shouldBeIn =
            item.roles.includes("*") || item.roles.some((role) => userRoles.includes(role as Role));
          if (shouldBeIn) {
            expect(result).toContain(item);
          }
        }
      }),
    );
  });
});

// ─── Property 2: Active Route Highlighting ───────────────────────────────────

/**
 * Property 2: Active Route Highlighting
 * Validates: Requirements 1.5
 *
 * For any valid route path from the navigation configuration, the
 * active-item-matching function SHALL return exactly one nav item whose `href`
 * matches the current route. No route should match zero items (when the route
 * is a known nav route) and no route should match more than one item.
 */
describe("Active route matching — Property 2: Active Route Highlighting", () => {
  /** The active-item-matching function as used in Sidebar */
  function findActiveItem(items: NavItem[], currentPath: string): NavItem | undefined {
    return items.find((item) => currentPath === item.href);
  }

  it("NAV_CONFIG has unique href values (precondition)", () => {
    const hrefs = NAV_CONFIG.map((item) => item.href);
    const uniqueHrefs = new Set(hrefs);
    expect(uniqueHrefs.size).toBe(hrefs.length);
  });

  it("for any NAV_CONFIG href, exactly one item matches", () => {
    const arbNavIndex = fc.integer({ min: 0, max: NAV_CONFIG.length - 1 });

    fc.assert(
      fc.property(arbNavIndex, (index) => {
        const targetItem = NAV_CONFIG[index];
        if (!targetItem) return;
        const currentPath = targetItem.href;

        // Count matches
        const matches = NAV_CONFIG.filter((item) => currentPath === item.href);
        expect(matches).toHaveLength(1);
        expect(matches[0]).toBe(targetItem);
      }),
    );
  });

  it("findActiveItem returns the correct item for any known route", () => {
    const arbNavIndex = fc.integer({ min: 0, max: NAV_CONFIG.length - 1 });

    fc.assert(
      fc.property(arbNavIndex, (index) => {
        const targetItem = NAV_CONFIG[index];
        if (!targetItem) return;
        const result = findActiveItem(NAV_CONFIG, targetItem.href);

        expect(result).toBeDefined();
        expect(result?.id).toBe(targetItem.id);
        expect(result?.href).toBe(targetItem.href);
      }),
    );
  });

  it("findActiveItem returns undefined for unknown routes", () => {
    const knownHrefs = new Set(NAV_CONFIG.map((item) => item.href));
    const arbUnknownPath = fc
      .string({ minLength: 2, maxLength: 30 })
      .map((s) => `/unknown/${s}`)
      .filter((path) => !knownHrefs.has(path));

    fc.assert(
      fc.property(arbUnknownPath, (unknownPath) => {
        const result = findActiveItem(NAV_CONFIG, unknownPath);
        expect(result).toBeUndefined();
      }),
    );
  });

  it("no route matches more than one item (uniqueness across arbitrary items with unique hrefs)", () => {
    const arbUniqueNavItems = fc
      .array(
        fc.record({
          id: fc.string({ minLength: 1, maxLength: 10 }),
          label: fc.string({ minLength: 1, maxLength: 20 }),
          href: fc.string({ minLength: 1, maxLength: 30 }).map((s) => `/${s}`),
          roles: fc.constant(["*"] as (Role | "*")[]),
          group: fc.constant("general" as const),
        }),
        { minLength: 2, maxLength: 15 },
      )
      .map((items) => {
        // Deduplicate hrefs
        const seen = new Set<string>();
        return items
          .filter((item) => {
            if (seen.has(item.href)) return false;
            seen.add(item.href);
            return true;
          })
          .map((item) => ({
            ...item,
            icon: (() => null) as unknown as NavItem["icon"],
          }));
      })
      .filter((items) => items.length >= 2);

    fc.assert(
      fc.property(arbUniqueNavItems, (items) => {
        for (const item of items) {
          const matches = items.filter((i) => i.href === item.href);
          expect(matches).toHaveLength(1);
        }
      }),
    );
  });
});
