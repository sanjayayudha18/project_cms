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
    vendor_branch_id: null,
    vendor_branch_name: null,
    is_active: true,
    deleted_at: null,
    supervisor_id: null,
    approval_level: null,
    last_login_at: null,
    ...overrides,
  };
}

describe("UsersTable Cabang column (vendor-party-scope Req 2.12)", () => {
  function renderTable(users: AdminUser[]) {
    render(
      <UsersTable
        users={users}
        vendorsById={new Map([[1, { name: "PT Abacus" }]])}
        onEdit={vi.fn()}
        onDisable={vi.fn()}
        onEnable={vi.fn()}
      />,
    );
  }

  it("shows the branch name for a branch-scoped vendor account", () => {
    renderTable([
      user({
        role: "VENDOR-ATM-USER",
        vendor_id: 1,
        vendor_branch_id: 10,
        vendor_branch_name: "Medan",
      }),
    ]);

    expect(screen.getByRole("columnheader", { name: "Cabang" })).toBeInTheDocument();
    expect(screen.getByText("PT Abacus")).toBeInTheDocument();
    expect(screen.getByText("Medan")).toBeInTheDocument();
  });

  it("shows 'Semua cabang' for a vendor-wide account and '-' for a non-vendor user", () => {
    renderTable([
      user({ id: 1, role: "VENDOR-USER", vendor_id: 1 }),
      user({ id: 2, username: "internal", vendor_id: null }),
    ]);

    expect(screen.getByText("Semua cabang")).toBeInTheDocument();
    expect(screen.getAllByText("-").length).toBeGreaterThan(0);
  });
});

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
