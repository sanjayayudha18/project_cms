import { NAV_CONFIG } from "@/lib/config/navigation";
import { describe, expect, it } from "vitest";
import catalogKeys from "./__fixtures__/catalog-keys.json";
import { ALWAYS_ON_KEYS, LOCKED_KEYS, SCREEN_KEYS } from "./permissionKeys";

// Feature: permission-catalog-coverage, Property 1 (catalog coverage)
// **Validates: Requirements 1.1, 1.4, 5.1, 9.1**
// catalog-keys.json is the dev `menu_features` key list (written by task 1.4); SCREEN_KEYS must match it.

const routeSources = import.meta.glob<string>(
  ["/src/routes/**/*.tsx", "!/src/routes/**/*.test.tsx"],
  { query: "?raw", import: "default", eager: true },
);

/** Route files that need no key: dashboard is Always_On, the rest are not guarded screens. */
const UNGUARDED = new Set([
  "/src/routes/index.tsx", // "/" dashboard — Always_On
  "/src/routes/login.tsx",
  "/src/routes/_auth.tsx",
  "/src/routes/_auth/login.tsx",
  "/src/routes/__root.tsx",
  "/src/routes/_protected.tsx",
  "/src/routes/settings/admin/exports.tsx", // redirect to /exports
]);

const GUARD_RE = /requirePermission\(\s*"([^"]+)"/g;

const guardKeys = (src: string): string[] => [...src.matchAll(GUARD_RE)].map((m) => m[1] as string);

describe("permission catalog coverage", () => {
  it("SCREEN_KEYS equals the dev catalog fixture", () => {
    expect([...SCREEN_KEYS].sort()).toEqual([...catalogKeys].sort());
  });

  it("Always_On and Locked keys are catalog keys", () => {
    for (const k of [...ALWAYS_ON_KEYS, ...LOCKED_KEYS]) expect(SCREEN_KEYS).toContain(k);
  });

  it("every NAV_CONFIG item (active or not) carries only catalog keys", () => {
    const known = new Set<string>(SCREEN_KEYS);
    for (const item of NAV_CONFIG) {
      const keys = typeof item.permission === "string" ? [item.permission] : item.permission;
      expect(keys.length, item.id).toBeGreaterThan(0);
      for (const k of keys) expect(known.has(k), `${item.id} → ${k}`).toBe(true);
    }
  });

  it("every route file is guarded by requirePermission (or explicitly exempt)", () => {
    const unguarded = Object.entries(routeSources)
      .filter(([path]) => !UNGUARDED.has(path))
      .filter(([, src]) => /createRoute\(/.test(src) && !/requirePermission\(/.test(src))
      .map(([path]) => path);
    expect(unguarded).toEqual([]);
  });

  it("every literal key used by a route guard is a catalog key", () => {
    const known = new Set<string>(SCREEN_KEYS);
    const used = Object.entries(routeSources).flatMap(([path, src]) =>
      guardKeys(src).map((k) => ({ path, k })),
    );
    expect(used.length).toBeGreaterThan(20);
    for (const { path, k } of used) expect(known.has(k), `${path} → ${k}`).toBe(true);
  });

  it("every active NAV_CONFIG item's key is enforced by some route guard", () => {
    const enforced = new Set(Object.values(routeSources).flatMap(guardKeys));
    for (const item of NAV_CONFIG) {
      if (item.disabled || typeof item.permission !== "string") continue;
      if (item.permission === "dashboard") continue; // Always_On, route unguarded
      expect(enforced.has(item.permission), `${item.id} → ${item.permission}`).toBe(true);
    }
  });
});
