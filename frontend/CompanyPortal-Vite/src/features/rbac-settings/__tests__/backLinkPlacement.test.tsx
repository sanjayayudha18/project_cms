/**
 * Property 3 (settings-menu-review, task 2.3): every Settings "tambah" page shows
 * exactly one "Kembali ke Pengaturan" link to /settings; the hub shows none.
 * Req 1.1, 1.2, 1.3, 1.6, 6.3.
 */

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import type { ComponentType, ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

// Link jadi <a href> biasa; hook router lain dibuat stub supaya tanpa RouterProvider.
vi.mock("@tanstack/react-router", () => ({
  Link: ({ to, children }: { to: string; children: ReactNode }) => <a href={to}>{children}</a>,
  useNavigate: () => vi.fn(),
  useSearch: () => ({}),
  useParams: () => ({}),
  useRouter: () => ({ navigate: vi.fn() }),
  useRouterState: () => ({ location: { pathname: "/" } }),
}));

import { AdminAcmAreasPage } from "@/features/admin-acm-areas/AdminAcmAreasPage";
import { AdminATMsPage } from "@/features/admin-atms/components/AdminATMsPage";
import { AdminHolidaysPage } from "@/features/admin-holidays/AdminHolidaysPage";
import { AdminRegionsPage } from "@/features/admin-regions/components/AdminRegionsPage";
import { AdminUsersPage } from "@/features/admin-users/components/AdminUsersPage";
import { AdminVendorsPage } from "@/features/admin-vendors/components/AdminVendorsPage";
import { MasterDataIOPage } from "@/features/master-data-io/components/MasterDataIOPage";
import { RoleManagementPage } from "@/features/role-management/components/RoleManagementPage";
import { RbacDelegationsPage } from "../components/RbacDelegationsPage";
import { RbacLeavesPage } from "../components/RbacLeavesPage";
import { RbacPoliciesPage } from "../components/RbacPoliciesPage";
import { RbacUsersPage } from "../components/RbacUsersPage";
import { SettingsHubPage } from "../components/SettingsHubPage";

const PAGES: [string, ComponentType][] = [
  ["AdminUsersPage", AdminUsersPage],
  ["AdminVendorsPage", AdminVendorsPage],
  ["AdminATMsPage", AdminATMsPage],
  ["AdminAcmAreasPage", AdminAcmAreasPage],
  ["AdminHolidaysPage", AdminHolidaysPage],
  ["AdminRegionsPage", AdminRegionsPage],
  ["MasterDataIOPage", MasterDataIOPage],
  ["RoleManagementPage", RoleManagementPage],
  ["RbacUsersPage", RbacUsersPage],
  ["RbacDelegationsPage", RbacDelegationsPage],
  ["RbacLeavesPage", RbacLeavesPage],
  ["RbacPoliciesPage", RbacPoliciesPage],
];

function renderPage(Page: ComponentType) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <Page />
    </QueryClientProvider>,
  );
}

describe("Property 3: BackLink di halaman Pengaturan", () => {
  beforeEach(() => {
    // Request tidak pernah selesai: halaman tetap di state loading, tautan tetap dirender.
    vi.stubGlobal(
      "fetch",
      vi.fn(() => new Promise(() => {})),
    );
  });

  it.each(PAGES)("%s punya tepat satu tautan ke /settings", (_name, Page) => {
    renderPage(Page);
    const links = screen.getAllByRole("link", { name: "Kembali ke Pengaturan" });
    expect(links).toHaveLength(1);
    expect(links[0]).toHaveAttribute("href", "/settings");
  });

  it("SettingsHubPage tidak punya tautan kembali", () => {
    renderPage(SettingsHubPage);
    expect(screen.queryByRole("link", { name: "Kembali ke Pengaturan" })).toBeNull();
  });
});
