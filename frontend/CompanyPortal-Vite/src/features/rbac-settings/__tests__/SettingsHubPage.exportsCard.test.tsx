import { SettingsHubPage } from "@/features/rbac-settings";
import { usePermissionStore } from "@/lib/auth/permissionStore";
import { SETTINGS_HUB_KEYS } from "@/routes/settings";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { expect, it, vi } from "vitest";

vi.mock("@tanstack/react-router", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@tanstack/react-router")>()),
  Link: ({ to, children }: { to: string; children?: ReactNode }) => <a href={to}>{children}</a>,
}));

function loginAsAdmin(): void {
  usePermissionStore.setState({ keys: new Set(SETTINGS_HUB_KEYS), status: "ready" });
}

async function openTab(name: string): Promise<void> {
  loginAsAdmin();
  render(<SettingsHubPage />);
  await userEvent.setup().click(screen.getByRole("tab", { name }));
}

it("shows the Export Data card on the Export & Import tab", async () => {
  await openTab("Export & Import");
  expect(screen.getByRole("link", { name: /Export Data/ })).toHaveAttribute("href", "/exports");
});

it("shows the Impor CSV Data Master card with the new description", async () => {
  await openTab("Export & Import");
  const card = screen.getByRole("link", { name: /Impor CSV Data Master/ });
  expect(card).toHaveAttribute("href", "/settings/admin/master-data-io");
  expect(card).toHaveTextContent("lalu pantau persetujuannya");
  expect(screen.queryByText("Ekspor & Impor CSV")).toBeNull();
});

it("shows only the import and export cards on the Export & Import tab", async () => {
  await openTab("Export & Import");
  const io = screen.getByRole("region", { name: "Export & Import" });
  for (const name of [/Impor CSV Data Master/, /Export Data/]) {
    expect(within(io).getByRole("link", { name })).toBeInTheDocument();
  }
  expect(within(io).getAllByRole("link")).toHaveLength(2);
  expect(screen.queryByRole("region", { name: "Data master" })).toBeNull();
});

it("no longer shows the import and export cards on the Data master tab", async () => {
  await openTab("Data master");
  expect(screen.queryByRole("region", { name: "Export & Import" })).toBeNull();
  expect(screen.queryByRole("link", { name: /Impor CSV Data Master/ })).toBeNull();
  expect(screen.queryByRole("link", { name: /Export Data/ })).toBeNull();
  expect(screen.getByRole("region", { name: "Data master" })).toBeInTheDocument();
});

it("hides the Export & Import section when the search matches neither card", async () => {
  await openTab("Export & Import");
  await userEvent
    .setup()
    .type(screen.getByRole("searchbox", { name: "Cari pengaturan" }), "region");
  expect(screen.queryByRole("region", { name: "Export & Import" })).toBeNull();
  expect(screen.getByText("Tidak ada pengaturan yang cocok.")).toBeInTheDocument();
});
