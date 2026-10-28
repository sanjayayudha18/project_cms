import { CalendarDays, HeartPulse, LayoutGrid, Settings2 } from "lucide-react";
import { describe, expect, it } from "vitest";
import { NAV_CONFIG, findActiveNavItem } from "./navigation";

const byId = (id: string) => {
  const item = NAV_CONFIG.find((i) => i.id === id);
  if (!item) throw new Error(`nav item ${id} missing`);
  return item;
};

describe("NAV_CONFIG (app-shell-revamp)", () => {
  it.each([
    ["dashboard", LayoutGrid],
    ["eod-monitoring", HeartPulse],
    ["holiday-calendar", CalendarDays],
    ["settings", Settings2],
  ])("ikon %s", (id, icon) => {
    expect(byId(id).icon).toBe(icon);
  });

  it("Pengaturan: group system, placement sidebar-footer", () => {
    expect(byId("settings")).toMatchObject({ group: "system", placement: "sidebar-footer" });
  });

  it.each([
    ["/replenishment/vendor-requests/42", "vendor-requests"],
    ["/settings/admin/holidays", "holiday-calendar"],
    ["/settings/admin/users", "settings"],
  ])("active item untuk %s", (path, id) => {
    expect(findActiveNavItem(NAV_CONFIG, path)?.id).toBe(id);
  });
});
