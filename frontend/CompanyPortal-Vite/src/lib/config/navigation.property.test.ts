// Feature: user-login, Property 13: Navigation Filtering by Role
import { ALWAYS_ON_KEYS, SCREEN_KEYS } from "@/lib/auth/permissionKeys";
import { usePermissionStore } from "@/lib/auth/permissionStore";
import type { DbRole } from "@/lib/auth/store";
import {
  getVisibleNavForRole as getVisibleNav,
  permissionsForRole,
} from "@/test/visibleNavForRole";
import * as fc from "fast-check";
import { describe, expect, it } from "vitest";
import {
  GROUP_LABELS,
  NAV_CONFIG,
  type NavItem,
  SIDEBAR_GROUP_ORDER,
  filterNavByPermissions,
  filterPaletteItems,
  findActiveNavItem,
  getVisibleNav as getVisibleNavByKeys,
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

// ─── Arbitraries ──────────────────────────────────────────────────────────────

const arbKeySet: fc.Arbitrary<Set<string>> = fc
  .subarray([...SCREEN_KEYS])
  .map((ks) => new Set<string>(ks));

const itemKeys = (i: NavItem): readonly string[] =>
  typeof i.permission === "string" ? [i.permission] : i.permission;

// ─── Property 8: nav visibility follows the permission set ───────────────────
// Feature: permission-catalog-coverage, Property 8
// **Validates: Requirements 5.1, 5.2, 5.3, 5.4, 5.5**

const ALWAYS_ON = new Set<string>(ALWAYS_ON_KEYS);
const enabledWith = (has: (k: string) => boolean) =>
  NAV_CONFIG.filter((i) => !i.disabled && itemKeys(i).some(has));

function setStore(status: "ready" | "loading" | "error" | "idle", keys: Set<string>) {
  usePermissionStore.setState({ status, keys });
}

describe("Property 8: filterNavByPermissions / getVisibleNav", () => {
  it("item visible ⇔ one of its keys is granted (no role bypass)", () => {
    fc.assert(
      fc.property(arbKeySet, (granted) => {
        const has = (k: string) => granted.has(k);
        const result = filterNavByPermissions(NAV_CONFIG, has);
        for (const item of NAV_CONFIG) {
          expect(result.includes(item)).toBe(itemKeys(item).some(has));
        }
      }),
      { numRuns: 200 },
    );
  });

  it("getVisibleNav = exactly the granted, enabled items; sidebar/footer partition them", () => {
    fc.assert(
      fc.property(arbKeySet, (granted) => {
        const nav = getVisibleNavByKeys((k) => granted.has(k));
        const expected = enabledWith((k) => granted.has(k));
        expect(nav.all).toEqual(expected);
        const footer = expected.filter((i) => i.placement === "sidebar-footer");
        expect(nav.settings).toBe(footer[0]);
        const sidebarIds = [...nav.sidebar.values()].flat().map((i) => i.id);
        const expectedSidebar = expected
          .filter((i) => i.placement !== "sidebar-footer" && SIDEBAR_GROUP_ORDER.includes(i.group))
          .map((i) => i.id);
        expect([...sidebarIds].sort()).toEqual([...expectedSidebar].sort());
      }),
      { numRuns: 200 },
    );
  });

  it("empty set → nothing visible; full set → every enabled item", () => {
    expect(getVisibleNavByKeys(() => false).all).toEqual([]);
    expect(getVisibleNavByKeys(() => true).all).toEqual(NAV_CONFIG.filter((i) => !i.disabled));
  });

  it("store ready: the store's keys decide", () => {
    fc.assert(
      fc.property(arbKeySet, (granted) => {
        setStore("ready", granted);
        const nav = getVisibleNavByKeys((k) => usePermissionStore.getState().has(k));
        expect(nav.all).toEqual(enabledWith((k) => granted.has(k)));
      }),
      { numRuns: 100 },
    );
    usePermissionStore.getState().reset();
  });

  it("store loading/error/idle: only Always_On items, whatever keys were held", () => {
    fc.assert(
      fc.property(
        arbKeySet,
        fc.constantFrom<"loading" | "error" | "idle">("loading", "error", "idle"),
        (granted, status) => {
          setStore(status, granted);
          const nav = getVisibleNavByKeys((k) => usePermissionStore.getState().has(k));
          expect(nav.all).toEqual(enabledWith((k) => ALWAYS_ON.has(k)));
          expect(nav.all.length).toBeGreaterThan(0);
        },
      ),
      { numRuns: 100 },
    );
    usePermissionStore.getState().reset();
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
  permission: "dashboard",
  group: "general",
});

/**
 * Feature: app-shell-revamp, Property 1: visibilitas menu per role tidak melebar
 * **Validates: Requirements 2.1, 2.4, 12.3**
 */
describe("Property 1: getVisibleNav visibility", () => {
  it("all ⊆ filterNavByPermissions, tanpa disabled; null role → kosong", () => {
    fc.assert(
      fc.property(arbShellRole, (role) => {
        const nav = getVisibleNav(role);
        const keys = permissionsForRole(role);
        const allowed = filterNavByPermissions(NAV_CONFIG, (k) => keys.has(k));
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

  it("sidebar: hanya grup SIDEBAR_GROUP_ORDER (berurutan), tanpa grup kosong, tanpa item sidebar-footer", () => {
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
            expect(item.placement).not.toBe("sidebar-footer");
            expect(nav.all).toContain(item);
          }
        }
        if (nav.settings) {
          expect(nav.settings.placement).toBe("sidebar-footer");
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
