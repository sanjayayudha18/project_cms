import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { UsersTable } from "../components/UsersTable";
import type { AdminUser } from "../types";

function user(overrides: Partial<AdminUser> = {}): AdminUser {
  return {
    id: 1,
    username: "budi.santoso",
    full_name: "Budi Santoso",
    email: "budi@example.com",
    role: "APPACCESS",
    is_karyawan: true,
    auth_source: "ldap",
    vendor_id: null,
    is_active: true,
    deleted_at: null,
    supervisor_id: null,
    approval_level: null,
    last_login_at: null,
    ...overrides,
  };
}

describe("UsersTable status badge (Task 8: icon+label)", () => {
  it("shows an Aktif badge with a checkmark icon for an active user", () => {
    render(
      <UsersTable
        users={[user({ is_active: true })]}
        vendorsById={new Map()}
        onEdit={vi.fn()}
        onDisable={vi.fn()}
        onEnable={vi.fn()}
      />,
    );

    const badge = screen.getByText("Aktif");
    expect(badge).toBeInTheDocument();
    expect(badge.parentElement?.querySelector("svg")).toBeInTheDocument();
  });

  it("shows a Nonaktif badge with an icon for a disabled user", () => {
    render(
      <UsersTable
        users={[user({ is_active: false })]}
        vendorsById={new Map()}
        onEdit={vi.fn()}
        onDisable={vi.fn()}
        onEnable={vi.fn()}
      />,
    );

    const badge = screen.getByText("Nonaktif");
    expect(badge).toBeInTheDocument();
    expect(badge.parentElement?.querySelector("svg")).toBeInTheDocument();
  });
});
