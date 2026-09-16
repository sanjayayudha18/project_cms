/**
 * Task 10: Settings hub gains "Manajemen Pengguna"/"Manajemen Vendor" cards
 * linking to the admin-users/admin-vendors routes, distinct from the
 * existing read-only "Hierarki Pengguna" card.
 */

import { render, screen } from "@testing-library/react";
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

describe("SettingsHubPage — admin cards (Task 10)", () => {
  it("links Manajemen Pengguna to /settings/admin/users", () => {
    render(<SettingsHubPage />);
    const link = screen.getByRole("link", { name: /Manajemen Pengguna/ });
    expect(link).toHaveAttribute("href", "/settings/admin/users");
  });

  it("links Manajemen Vendor to /settings/admin/vendors", () => {
    render(<SettingsHubPage />);
    const link = screen.getByRole("link", { name: /Manajemen Vendor/ });
    expect(link).toHaveAttribute("href", "/settings/admin/vendors");
  });

  it("keeps the existing Hierarki Pengguna card distinct (read-only hierarchy vs CRUD)", () => {
    render(<SettingsHubPage />);
    expect(screen.getByRole("link", { name: /Hierarki Pengguna/ })).toHaveAttribute(
      "href",
      "/settings/rbac/users",
    );
    expect(screen.getByText(/Tambah, ubah, dan nonaktifkan akun pengguna/)).toBeInTheDocument();
  });
});
