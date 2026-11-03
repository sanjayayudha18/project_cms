import { SettingsHubPage } from "@/features/rbac-settings";
import { useAuthStore } from "@/lib/auth/store";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { expect, it, vi } from "vitest";

vi.mock("@tanstack/react-router", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@tanstack/react-router")>()),
  Link: ({ to, children }: { to: string; children?: ReactNode }) => <a href={to}>{children}</a>,
}));

function loginAsAdmin(): void {
  useAuthStore.setState({
    user: {
      id: 1,
      username: "a",
      fullName: "A",
      email: "a@b.c",
      role: "ADMIN",
      isKaryawan: true,
      vendorId: null,
    },
    accessToken: "t",
    isAuthenticated: true,
    isAuthLoading: false,
    error: null,
    rateLimitRetryAfter: null,
  });
}

async function openDataMasterTab(): Promise<void> {
  loginAsAdmin();
  render(<SettingsHubPage />);
  await userEvent.setup().click(screen.getByRole("tab", { name: "Data master" }));
}

it("shows the Export Data card on the Data master tab", async () => {
  await openDataMasterTab();
  expect(screen.getByRole("link", { name: /Export Data/ })).toHaveAttribute("href", "/exports");
});

it("shows the Impor CSV Data Master card with the new description", async () => {
  await openDataMasterTab();
  const card = screen.getByRole("link", { name: /Impor CSV Data Master/ });
  expect(card).toHaveAttribute("href", "/settings/admin/master-data-io");
  expect(card).toHaveTextContent("lalu pantau persetujuannya");
  expect(screen.queryByText("Ekspor & Impor CSV")).toBeNull();
});
