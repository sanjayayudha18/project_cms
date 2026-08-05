import { LayoutDashboard } from "lucide-react";
import { describe, expect, it } from "vitest";
import { GROUP_LABELS, NAV_CONFIG, type NavItem, filterNavByRoles } from "./navigation";

// ─── Test helpers ─────────────────────────────────────────────────────────────

function makeItem(overrides: Partial<NavItem> = {}): NavItem {
  return {
    id: "test-item",
    label: "Test",
    icon: LayoutDashboard,
    href: "/test",
    roles: ["Admin"],
    group: "general",
    ...overrides,
  };
}

// ─── filterNavByRoles ─────────────────────────────────────────────────────────

describe("filterNavByRoles", () => {
  it("returns empty array when user has no roles", () => {
    const items: NavItem[] = [makeItem({ roles: ["*"] })];
    expect(filterNavByRoles(items, [])).toEqual([]);
  });

  it("returns all wildcard items for any authenticated user", () => {
    const items: NavItem[] = [
      makeItem({ id: "a", roles: ["*"] }),
      makeItem({ id: "b", roles: ["Admin"] }),
    ];
    const result = filterNavByRoles(items, ["Vendor"]);
    expect(result.map((i) => i.id)).toEqual(["a"]);
  });

  it("includes items matching any of user roles", () => {
    const items: NavItem[] = [
      makeItem({ id: "a", roles: ["Admin", "WMO"] }),
      makeItem({ id: "b", roles: ["Finance"] }),
    ];
    const result = filterNavByRoles(items, ["WMO"]);
    expect(result.map((i) => i.id)).toEqual(["a"]);
  });

  it("includes items when user has multiple non-admin roles", () => {
    const items: NavItem[] = [
      makeItem({ id: "a", roles: ["WMO"] }),
      makeItem({ id: "b", roles: ["Finance"] }),
      makeItem({ id: "c", roles: ["Vendor"] }),
    ];
    const result = filterNavByRoles(items, ["WMO", "Finance"]);
    expect(result.map((i) => i.id)).toEqual(["a", "b"]);
  });

  it("returns all items when user is Admin (Admin sees everything)", () => {
    const items: NavItem[] = [
      makeItem({ id: "a", roles: ["Admin"] }),
      makeItem({ id: "b", roles: ["Finance"] }),
      makeItem({ id: "c", roles: ["Vendor"] }),
    ];
    const result = filterNavByRoles(items, ["Admin", "Finance"]);
    expect(result.map((i) => i.id)).toEqual(["a", "b", "c"]);
  });

  it("returns empty array when no items match", () => {
    const items: NavItem[] = [
      makeItem({ id: "a", roles: ["Admin"] }),
      makeItem({ id: "b", roles: ["Finance"] }),
    ];
    const result = filterNavByRoles(items, ["Vendor"]);
    expect(result).toEqual([]);
  });

  it("returns empty array when items array is empty", () => {
    expect(filterNavByRoles([], ["Admin"])).toEqual([]);
  });

  it("preserves disabled items in output (filtering is role-based only)", () => {
    const items: NavItem[] = [
      makeItem({ id: "a", roles: ["Admin"], disabled: true }),
      makeItem({ id: "b", roles: ["Admin"], disabled: false }),
    ];
    const result = filterNavByRoles(items, ["Admin"]);
    expect(result).toHaveLength(2);
    expect(result[0]?.disabled).toBe(true);
  });

  it("handles wildcard mixed with specific roles", () => {
    const items: NavItem[] = [
      makeItem({ id: "dashboard", roles: ["*"] }),
      makeItem({ id: "settings", roles: ["Admin"] }),
      makeItem({ id: "upload", roles: ["Vendor", "ATM_Support"] }),
    ];
    const result = filterNavByRoles(items, ["ATM_Support"]);
    expect(result.map((i) => i.id)).toEqual(["dashboard", "upload"]);
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

  it("settings is restricted to Admin only", () => {
    const settings = NAV_CONFIG.find((i) => i.id === "settings");
    expect(settings?.roles).toEqual(["Admin"]);
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
