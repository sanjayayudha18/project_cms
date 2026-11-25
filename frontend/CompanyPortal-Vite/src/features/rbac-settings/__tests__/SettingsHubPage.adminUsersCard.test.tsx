/**
 * permission-catalog-coverage task 6.2: hub cards are shown per catalog key
 * (permissionStore), not per role. Hiding is UX only; routes/API still guard.
 */

import { SettingsHubPage } from "@/features/rbac-settings";
import { usePermissionStore } from "@/lib/auth/permissionStore";
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

function grant(...keys: string[]) {
  usePermissionStore.setState({ keys: new Set(keys), status: "ready" });
}

async function openMasterTab() {
  await userEvent.setup().click(screen.getByRole("tab", { name: "Data master" }));
}

beforeEach(() => {
  usePermissionStore.setState({ keys: new Set(), status: "idle" });
});

describe("SettingsHubPage — cards per permission key", () => {
  it("shows Manajemen Pengguna and Tambah Pengguna with settings.users", () => {
    grant("settings.users");
    render(<SettingsHubPage />);

    expect(screen.getByRole("link", { name: /Manajemen Pengguna/ })).toHaveAttribute(
      "href",
      "/settings/admin/users",
    );
    expect(screen.getByRole("link", { name: /Tambah Pengguna/ })).toHaveAttribute(
      "href",
      "/settings/admin/users/new",
    );
  });

  it("hides user cards without settings.users, while other granted cards stay", async () => {
    grant("settings.vendors");
    render(<SettingsHubPage />);

    expect(screen.queryByRole("link", { name: /Manajemen Pengguna/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /Tambah Pengguna/ })).not.toBeInTheDocument();
    await openMasterTab();
    expect(screen.getByRole("link", { name: /Manajemen Vendor/ })).toBeInTheDocument();
  });

  it("read-only PKS access shows only Daftar PKS and Manajemen Vendor", () => {
    grant("settings.pks", "settings.vendors");
    render(<SettingsHubPage />);

    // Only the Data master category is visible, so the hub falls back to it.
    expect(screen.getAllByRole("tab")).toHaveLength(1);
    expect(screen.getByRole("link", { name: /Daftar PKS/ })).toHaveAttribute(
      "href",
      "/settings/admin/pks",
    );
    expect(screen.getByRole("link", { name: /Manajemen Vendor/ })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /Manajemen ATM/ })).not.toBeInTheDocument();
  });

  it("shows Area ACM and Manajemen Peran only with their own keys", async () => {
    grant("settings.acm-areas", "settings.roles");
    render(<SettingsHubPage />);
    expect(screen.getByRole("link", { name: /Manajemen Peran/ })).toBeInTheDocument();
    await openMasterTab();
    expect(screen.getByRole("link", { name: /Area ACM/ })).toBeInTheDocument();
  });

  it.each(["idle", "loading", "error"] as const)("shows no cards while status is %s", (status) => {
    usePermissionStore.setState({ keys: new Set(["settings.users"]), status });
    render(<SettingsHubPage />);
    expect(screen.queryAllByRole("link")).toHaveLength(0);
    expect(screen.queryAllByRole("tab")).toHaveLength(0);
  });
});
