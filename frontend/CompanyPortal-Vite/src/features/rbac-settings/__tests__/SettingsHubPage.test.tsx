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

import { fireEvent, render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
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

function renderMasterTab() {
  render(<SettingsHubPage />);
  fireEvent.click(screen.getByRole("tab", { name: "Data master" }));
}

describe("SettingsHubPage — admin cards (Task 10)", () => {
  it.each([
    ["Manajemen Pengguna", "/settings/admin/users"],
    ["Manajemen Vendor", "/settings/admin/vendors"],
    ["Manajemen ATM", "/settings/admin/atms"],
    ["Manajemen Region", "/settings/admin/regions"],
  ])("links %s to %s on the Data master tab", (name, href) => {
    renderMasterTab();
    expect(screen.getByRole("link", { name: new RegExp(name) })).toHaveAttribute("href", href);
  });

  it("hides master-data cards on the default Akses & persetujuan tab", () => {
    render(<SettingsHubPage />);
    expect(screen.queryByRole("link", { name: /Manajemen Region/ })).not.toBeInTheDocument();
  });

  it("keeps the existing Hierarki Pengguna card distinct (read-only hierarchy vs CRUD)", () => {
    render(<SettingsHubPage />);
    expect(screen.getByText("Hierarki Pengguna")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Kelola hierarki/ })).toHaveAttribute(
      "href",
      "/settings/rbac/users",
    );
  });
});
