import { LayoutDashboard } from "lucide-react";
import { describe, expect, it } from "vitest";
import type { DbRole } from "@/lib/auth/store";
import { GROUP_LABELS, NAV_CONFIG, type NavItem, filterNavByRoles } from "./navigation";

// ─── Test helpers ─────────────────────────────────────────────────────────────

function makeItem(overrides: Partial<NavItem> = {}): NavItem {
  return {
    id: "test-item",
    label: "Test",
    icon: LayoutDashboard,
    href: "/test",
    roles: ["ADMIN"],
    group: "general",
    ...overrides,
  };
}

// ─── filterNavByRoles ─────────────────────────────────────────────────────────

describe("filterNavByRoles", () => {
  it("returns all wildcard items for any authenticated user", () => {
    const items: NavItem[] = [
      makeItem({ id: "a", roles: ["*"] }),
      makeItem({ id: "b", roles: ["ADMIN"] }),
    ];
    const result = filterNavByRoles(items, "VENDOR-USER");
    expect(result.map((i) => i.id)).toEqual(["a"]);
  });

  it("includes items matching user role", () => {
    const items: NavItem[] = [
      makeItem({ id: "a", roles: ["ATM-USER", "ATM-SPV"] }),
      makeItem({ id: "b", roles: ["BRANCH-USER"] }),
    ];
    const result = filterNavByRoles(items, "ATM-SPV");
    expect(result.map((i) => i.id)).toEqual(["a"]);
  });

  it("ADMIN sees all items regardless of role restrictions", () => {
    const items: NavItem[] = [
      makeItem({ id: "a", roles: ["BRANCH-USER"] }),
      makeItem({ id: "b", roles: ["VENDOR-USER"] }),
      makeItem({ id: "c", roles: ["ATM-SPV"] }),
    ];
    const result = filterNavByRoles(items, "ADMIN");
    expect(result.map((i) => i.id)).toEqual(["a", "b", "c"]);
  });

  it("ADMIN_PARAM sees all items regardless of role restrictions", () => {
    const items: NavItem[] = [
      makeItem({ id: "a", roles: ["BRANCH-USER"] }),
      makeItem({ id: "b", roles: ["VENDOR-USER"] }),
    ];
    const result = filterNavByRoles(items, "ADMIN_PARAM");
    expect(result.map((i) => i.id)).toEqual(["a", "b"]);
  });

  it("returns empty array when no items match user role", () => {
    const items: NavItem[] = [
      makeItem({ id: "a", roles: ["ADMIN"] }),
      makeItem({ id: "b", roles: ["BRANCH-USER"] }),
    ];
    const result = filterNavByRoles(items, "VENDOR-USER");
    expect(result).toEqual([]);
  });

  it("returns empty array when items array is empty", () => {
    expect(filterNavByRoles([], "ADMIN")).toEqual([]);
  });

  it("preserves disabled items in output (filtering is role-based only)", () => {
    const items: NavItem[] = [
      makeItem({ id: "a", roles: ["ATM-USER"], disabled: true }),
      makeItem({ id: "b", roles: ["ATM-USER"], disabled: false }),
    ];
    const result = filterNavByRoles(items, "ATM-USER");
    expect(result).toHaveLength(2);
    expect(result[0]?.disabled).toBe(true);
  });

  it("handles wildcard mixed with specific roles", () => {
    const items: NavItem[] = [
      makeItem({ id: "dashboard", roles: ["*"] }),
      makeItem({ id: "settings", roles: ["ADMIN"] }),
      makeItem({ id: "upload", roles: ["VENDOR-USER", "ATM-USER"] }),
    ];
    const result = filterNavByRoles(items, "ATM-USER");
    expect(result.map((i) => i.id)).toEqual(["dashboard", "upload"]);
  });

  it("VENDOR-USER only sees items with VENDOR-USER or wildcard", () => {
    const items: NavItem[] = [
      makeItem({ id: "dashboard", roles: ["*"] }),
      makeItem({ id: "dsr-upload", roles: ["VENDOR-USER", "ATM-USER", "ATM-SPV"] }),
      makeItem({ id: "settings", roles: ["ADMIN"] }),
      makeItem({ id: "forecast", roles: ["ATM-USER", "ATM-SPV"] }),
    ];
    const result = filterNavByRoles(items, "VENDOR-USER");
    expect(result.map((i) => i.id)).toEqual(["dashboard", "dsr-upload"]);
  });
});

// ─── NAV_CONFIG structure ─────────────────────────────────────────────────────

describe("NAV_CONFIG", () => {
  it("has unique IDs for all items", () => {
    const ids = NAV_CONFIG.map((item) => item.id);
    const uniqueIds = new Set(ids);
    expect(uniqueIds.size).toBe(ids.length);
  });

  it("all items have non-empty labels", () => {
    for (const item of NAV_CONFIG) {
      expect(item.label.length).toBeGreaterThan(0);
    }
  });

  it("all items have valid group assignments", () => {
    const validGroups = Object.keys(GROUP_LABELS);
    for (const item of NAV_CONFIG) {
      expect(validGroups).toContain(item.group);
    }
  });

  it("dashboard is visible to all roles (wildcard)", () => {
    const dashboard = NAV_CONFIG.find((i) => i.id === "dashboard");
    expect(dashboard?.roles).toContain("*");
  });

  it("settings is restricted to ADMIN only", () => {
    const settings = NAV_CONFIG.find((i) => i.id === "settings");
    expect(settings?.roles).toEqual(["ADMIN"]);
  });

  it("all roles in NAV_CONFIG are valid DbRole or wildcard", () => {
    const validRoles: (DbRole | "*")[] = [
      "*",
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
    for (const item of NAV_CONFIG) {
      for (const role of item.roles) {
        expect(validRoles).toContain(role);
      }
    }
  });
});

// ─── GROUP_LABELS ─────────────────────────────────────────────────────────────

describe("GROUP_LABELS", () => {
  it("has labels in Bahasa Indonesia", () => {
    expect(GROUP_LABELS.general).toBe("Umum");
    expect(GROUP_LABELS.forecasting).toBe("Peramalan");
    expect(GROUP_LABELS.invoice).toBe("Tagihan");
    expect(GROUP_LABELS["cash-count"]).toBe("Perhitungan Kas");
  });
});
