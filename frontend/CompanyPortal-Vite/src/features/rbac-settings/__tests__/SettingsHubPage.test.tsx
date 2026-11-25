/**
 * Task 10 (admin-user-vendor-management): Settings hub gains "Manajemen
 * Pengguna"/"Manajemen Vendor" cards linking to the admin-users/
 * admin-vendors routes, distinct from the existing read-only "Hierarki
 * Pengguna" card. Task 9 (admin-atm-management) adds "Manajemen ATM";
 * region-management Task 10.2 adds "Manajemen Region".
 *
 * Since the UI revamp, master-data cards live under the "Data master" tab
 * (the hub opens on "Akses & persetujuan"), so those tests switch tab first.
 */

import { usePermissionStore } from "@/lib/auth/permissionStore";
import { SETTINGS_HUB_KEYS } from "@/routes/settings";
import { fireEvent, render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { SettingsHubPage } from "../components/SettingsHubPage";

vi.mock("@tanstack/react-router", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    Link: ({
      to,
      children,
      className,
    }: {
      to: string;
      children?: ReactNode;
      className?: string;
    }) => (
      <a href={to} className={className}>
        {children}
      </a>
    ),
  };
});

beforeEach(() => {
  usePermissionStore.setState({ keys: new Set(SETTINGS_HUB_KEYS), status: "ready" });
});

function renderMasterTab() {
  render(<SettingsHubPage />);
  fireEvent.click(screen.getByRole("tab", { name: "Data master" }));
}

describe("SettingsHubPage — admin cards (Task 10)", () => {
  it.each([
    ["Manajemen Vendor", "/settings/admin/vendors"],
    ["Manajemen ATM", "/settings/admin/atms"],
    ["Manajemen Region", "/settings/admin/regions"],
    ["Type PKS", "/settings/admin/pks-types"],
  ])("links %s to %s on the Data master tab", (name, href) => {
    renderMasterTab();
    expect(screen.getByRole("link", { name: new RegExp(name) })).toHaveAttribute("href", href);
  });

  it("hides master-data cards on the default Akses & persetujuan tab", () => {
    render(<SettingsHubPage />);
    expect(screen.queryByRole("link", { name: /Manajemen Region/ })).not.toBeInTheDocument();
  });

  it.each([
    ["Hierarki Pengguna", "/settings/rbac/users"],
    ["Manajemen Pengguna", "/settings/admin/users"],
  ])("links %s to %s on the default Akses & persetujuan tab", (name, href) => {
    render(<SettingsHubPage />);
    expect(screen.getByRole("tab", { name: "Akses & persetujuan" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(screen.getByRole("link", { name: new RegExp(name) })).toHaveAttribute("href", href);
  });

  it("shows only the three categories in the rail (no 'Semua')", () => {
    render(<SettingsHubPage />);
    expect(screen.getAllByRole("tab")).toHaveLength(3);
    expect(screen.queryByRole("tab", { name: "Semua" })).toBeNull();
  });
});
