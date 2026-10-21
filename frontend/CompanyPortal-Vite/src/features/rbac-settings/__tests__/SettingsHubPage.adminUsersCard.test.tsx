/**
 * vendor-party-scope Req 2.13 / D8: /api/v1/admin/users is APPACCESS + ADMIN only, so the
 * "Manajemen Pengguna" card (Data master tab) is hidden for ADMIN_PARAM. The frontend
 * requireRoles() still lets ADMIN_PARAM through to the route; the backend 403 is the real guard.
 */

import { SettingsHubPage } from "@/features/rbac-settings";
import { useAuthStore } from "@/lib/auth/store";
import type { DbRole } from "@/lib/auth/store";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
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
      isKaryawan: true,
      vendorId: null,
    },
    accessToken: "test-token",
    isAuthenticated: true,
    isAuthLoading: false,
    error: null,
    rateLimitRetryAfter: null,
  });
}

async function openMasterTab() {
  await userEvent.setup().click(screen.getByRole("tab", { name: "Data master" }));
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

describe("SettingsHubPage — Manajemen Pengguna card visibility", () => {
  it.each(["APPACCESS", "ADMIN"] as const)("is visible for %s", async (role) => {
    setUser(role);
    render(<SettingsHubPage />);
    await openMasterTab();

    expect(screen.getByRole("link", { name: /Manajemen Pengguna/ })).toHaveAttribute(
      "href",
      "/settings/admin/users",
    );
  });

  it("is hidden for ADMIN_PARAM, while other master-data cards stay", async () => {
    setUser("ADMIN_PARAM");
    render(<SettingsHubPage />);
    await openMasterTab();

    expect(screen.queryByRole("link", { name: /Manajemen Pengguna/ })).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Manajemen Vendor/ })).toBeInTheDocument();
  });
});
