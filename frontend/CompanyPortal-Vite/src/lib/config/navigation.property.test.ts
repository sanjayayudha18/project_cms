// Feature: user-login, Property 13: Navigation Filtering by Role
import type { DbRole } from "@/lib/auth/store";
import * as fc from "fast-check";
import { describe, expect, it } from "vitest";
import {
  GROUP_LABELS,
  NAV_CONFIG,
  type NavItem,
  SIDEBAR_GROUP_ORDER,
  filterNavByRoles,
  filterPaletteItems,
  findActiveNavItem,
  getVisibleNav,
} from "./navigation";

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
const NON_ADMIN_ROLES: DbRole[] = ALL_DB_ROLES.filter((r) => !ADMIN_ROLES.includes(r));

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
      "monitoring" as const,
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
            isAdmin || item.roles.includes("*") || item.roles.includes(userRole);

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
          const shouldBeExcluded = !item.roles.includes("*") && !item.roles.includes(userRole);

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
          const justified = item.roles.includes("*") || item.roles.includes(userRole);
          expect(justified).toBe(true);
        }

        // No missing items
        for (const item of NAV_CONFIG) {
          const shouldBeIn = item.roles.includes("*") || item.roles.includes(userRole);
          if (shouldBeIn) {
            expect(result).toContain(item);
          }
        }
      }),
      { numRuns: 100 },
    );
  });
});

// ─── app-shell-revamp ─────────────────────────────────────────────────────────

const SHELL_ROLES: DbRole[] = [
  ...ALL_DB_ROLES,
  "APPACCESS",
  "VENDOR-ATM-USER",
  "VENDOR-VAULT-USER",
  "ACM-USER",
  "ACM-SPV",
];
const arbShellRole: fc.Arbitrary<DbRole | null> = fc.oneof(
  fc.constant(null),
  fc.constantFrom(...SHELL_ROLES),
);

const stubIcon = (() => null) as unknown as NavItem["icon"];
const mkItem = (id: string, href: string, label = id): NavItem => ({
  id,
  label,
  icon: stubIcon,
  href,
  roles: ["*"],
  group: "general",
});

/**
 * Feature: app-shell-revamp, Property 1: visibilitas menu per role tidak melebar
 * **Validates: Requirements 2.1, 2.4, 12.3**
 */
describe("Property 1: getVisibleNav visibility", () => {
  it("all ⊆ filterNavByRoles, tanpa disabled; null role → kosong", () => {
    fc.assert(
      fc.property(arbShellRole, (role) => {
        const nav = getVisibleNav(role);
        const allowed = role ? filterNavByRoles(NAV_CONFIG, role) : [];
        for (const item of nav.all) {
          expect(allowed).toContain(item);
          expect(item.disabled).toBeFalsy();
        }
        for (const item of allowed) {
          if (!item.disabled) expect(nav.all).toContain(item);
        }
        if (role === null) {
          expect(nav.all).toEqual([]);
          expect(nav.sidebar.size).toBe(0);
          expect(nav.settings).toBeUndefined();
        }
      }),
      { numRuns: 200 },
    );
  });

  it("sidebar: hanya grup SIDEBAR_GROUP_ORDER (berurutan), tanpa grup kosong, tanpa item topbar", () => {
    fc.assert(
      fc.property(arbShellRole, (role) => {
        const nav = getVisibleNav(role);
        const keys = [...nav.sidebar.keys()];
        const order = SIDEBAR_GROUP_ORDER.filter((g) => keys.includes(g));
        expect(keys).toEqual(order);
        for (const [group, items] of nav.sidebar) {
          expect(items.length).toBeGreaterThan(0);
          for (const item of items) {
            expect(item.group).toBe(group);
            expect(item.placement).not.toBe("topbar");
            expect(nav.all).toContain(item);
          }
        }
        if (nav.settings) {
          expect(nav.settings.placement).toBe("topbar");
          expect(nav.all).toContain(nav.settings);
        }
      }),
      { numRuns: 200 },
    );
  });
});

// Segmen dari alfabet kecil supaya prefix sering bertabrakan.
const arbSegs = fc.array(fc.constantFrom("a", "b", "c"), { maxLength: 3 });
const toHref = (segs: string[]) => `/${segs.join("/")}`;
const arbItems = fc
  .uniqueArray(arbSegs, { selector: (s) => s.join("/"), maxLength: 6 })
  .map((all) => all.map((segs, i) => mkItem(`i${i}`, toHref(segs))));
const arbSlashes = fc.constantFrom("", "/", "//");

/**
 * Feature: app-shell-revamp, Property 2: active match
 * **Validates: Requirements 3.2, 7.2, 12.4**
 */
describe("Property 2: findActiveNavItem", () => {
  it("pilih prefix segmen terpanjang; '/' hanya exact; trailing slash dinormalisasi", () => {
    fc.assert(
      fc.property(arbItems, arbSegs, arbSlashes, (items, pathSegs, slashes) => {
        const path = toHref(pathSegs) + (pathSegs.length ? slashes : "");
        const hrefSegs = (i: NavItem) => i.href.split("/").filter(Boolean);
        const matches = items.filter((i) => {
          const hs = hrefSegs(i);
          return hs.length === 0
            ? pathSegs.length === 0
            : hs.every((s, idx) => pathSegs[idx] === s);
        });
        const longest = Math.max(-1, ...matches.map((i) => hrefSegs(i).length));
        const result = findActiveNavItem(items, path);
        if (matches.length === 0) {
          expect(result).toBeUndefined();
        } else {
          expect(result && hrefSegs(result).length).toBe(longest);
          expect(matches).toContain(result);
        }
        expect(findActiveNavItem(items, `${path}/`)).toBe(result);
      }),
      { numRuns: 300 },
    );
  });

  it("segmen parsial tidak cocok (/ab bukan anak /a)", () => {
    expect(findActiveNavItem([mkItem("a", "/a")], "/ab")).toBeUndefined();
  });
});

/**
 * Feature: app-shell-revamp, Property 3: filter palette
 * **Validates: Requirements 10.2, 12.9**
 */
describe("Property 3: filterPaletteItems", () => {
  const arbQuery = fc.stringMatching(/^[a-zA-Z ]{0,6}$/);

  it("subset, urutan dipertahankan, query kosong = semua", () => {
    fc.assert(
      fc.property(arbShellRole, arbQuery, (role, query) => {
        const { all } = getVisibleNav(role);
        const result = filterPaletteItems(all, query);
        const idx = result.map((i) => all.indexOf(i));
        expect(idx.every((n) => n >= 0)).toBe(true);
        expect([...idx].sort((x, y) => x - y)).toEqual(idx);
        expect(filterPaletteItems(all, "")).toEqual(all);
        expect(filterPaletteItems(all, "   ")).toEqual(all);
      }),
      { numRuns: 200 },
    );
  });

  it("case-insensitive dan substring label/grup selalu ketemu", () => {
    fc.assert(
      fc.property(
        fc.constantFrom(...NAV_CONFIG),
        fc.boolean(),
        fc.nat(),
        fc.nat(),
        (item, useGroup, a, b) => {
          const text = useGroup ? GROUP_LABELS[item.group] : item.label;
          const start = a % text.length;
          const q = text.slice(start, start + 1 + (b % text.length)).trim();
          if (!q) return;
          const lower = filterPaletteItems(NAV_CONFIG, q.toLocaleLowerCase("id-ID"));
          const upper = filterPaletteItems(NAV_CONFIG, q.toLocaleUpperCase("id-ID"));
          expect(upper).toEqual(lower);
          expect(lower).toContain(item);
        },
      ),
      { numRuns: 200 },
    );
  });
});
