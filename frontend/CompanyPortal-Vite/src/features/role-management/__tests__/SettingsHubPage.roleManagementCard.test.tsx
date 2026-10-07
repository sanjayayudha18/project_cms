/**
 * Design property 8 (Nav renders exactly the mapped menus/features), scoped
 * to the "Manajemen Peran" card: visible to APPACCESS/ADMIN, hidden for
 * every other role (Req 1.4, 4.5). Lives under role-management's own
 * __tests__ (not rbac-settings) since it exercises this feature's specific
 * card, and rbac-settings/__tests__/SettingsHubPage.test.tsx has a
 * pre-existing, unrelated failure (tab-default mismatch from an in-progress
 * edit) this test avoids by not depending on the "Data master" tab at all.
 */

import { SettingsHubPage } from "@/features/rbac-settings";
import { useAuthStore } from "@/lib/auth/store";
import type { DbRole } from "@/lib/auth/store";
import { render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

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

function setUser(role: DbRole) {
  useAuthStore.setState({
    user: {
      id: 1,
      username: "test.user",
      fullName: "Test User",
      email: "test@cimb.local",
      role,
      isKaryawan: role !== "VENDOR-USER",
      vendorId: role === "VENDOR-USER" ? 1 : null,
    },
    accessToken: "test-token",
    isAuthenticated: true,
    isAuthLoading: false,
    error: null,
    rateLimitRetryAfter: null,
  });
}

beforeEach(() => {
  useAuthStore.setState({
    user: null,
    accessToken: null,
    isAuthenticated: false,
    isAuthLoading: false,
    error: null,
    rateLimitRetryAfter: null,
  });
});

describe("SettingsHubPage — Manajemen Peran card visibility", () => {
  it("is visible for APPACCESS", () => {
    setUser("APPACCESS");
    render(<SettingsHubPage />);
    expect(screen.getByRole("link", { name: /Manajemen Peran/ })).toHaveAttribute(
      "href",
      "/settings/roles",
    );
  });

  it("is visible for ADMIN", () => {
    setUser("ADMIN");
    render(<SettingsHubPage />);
    expect(screen.getByRole("link", { name: /Manajemen Peran/ })).toHaveAttribute(
      "href",
      "/settings/roles",
    );
  });

  it("is hidden for ADMIN_PARAM", () => {
    setUser("ADMIN_PARAM");
    render(<SettingsHubPage />);
    expect(screen.queryByRole("link", { name: /Manajemen Peran/ })).not.toBeInTheDocument();
  });

  it("is hidden for a non-admin role", () => {
    setUser("ATM-USER");
    render(<SettingsHubPage />);
    expect(screen.queryByRole("link", { name: /Manajemen Peran/ })).not.toBeInTheDocument();
  });

  it("is hidden when no user is set", () => {
    render(<SettingsHubPage />);
    expect(screen.queryByRole("link", { name: /Manajemen Peran/ })).not.toBeInTheDocument();
  });
});
