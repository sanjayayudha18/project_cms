import { type DbRole, useAuthStore } from "@/lib/auth/store";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("@tanstack/react-router", () => ({
  Link: ({
    to,
    params,
    children,
    ...rest
  }: { to: string; params?: Record<string, string>; children: ReactNode }) => (
    <a href={to.replace("$dataset", params?.dataset ?? "")} {...rest}>
      {children}
    </a>
  ),
}));
vi.mock("./MyExportsTable", () => ({ MyExportsTable: () => <div>tabel-ekspor-saya</div> }));

import { ExportCatalogPage } from "./ExportCatalogPage";

const MASTER: Record<string, string> = {
  vendors: "Vendor",
  "vendor-branches": "Cabang vendor",
  "vendor-vaults": "Vault",
  "vendor-pics": "PIC vendor",
  "dsr-location-maps": "Pemetaan lokasi DSR",
  "package-prices": "Harga paket",
  atms: "ATM",
  "atm-assignments": "Kelolaan ATM",
};
const REPORTS: Record<string, string> = {
  "/dsr-late-report": "Laporan DSR telat",
  "/replenishment": "Laporan replenishment",
};

function renderAs(role: DbRole) {
  useAuthStore.setState({ user: { role } as never });
  return render(<ExportCatalogPage />);
}

const hrefOf = (nameStart: string) =>
  screen
    .getAllByRole("link")
    .find((l) => (l.textContent ?? "").startsWith(nameStart))
    ?.getAttribute("href");

afterEach(() => useAuthStore.setState({ user: null }));

describe("ExportCatalogPage", () => {
  it("ADMIN sees all eight master datasets and both reports", () => {
    renderAs("ADMIN");
    expect(screen.getByRole("heading", { name: "Data master" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Laporan" })).toBeInTheDocument();
    expect(screen.getAllByRole("link")).toHaveLength(10);
  });

  it("ADMIN_PARAM sees only master data (no Laporan group)", () => {
    renderAs("ADMIN_PARAM");
    expect(screen.getByRole("heading", { name: "Data master" })).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "Laporan" })).not.toBeInTheDocument();
    expect(screen.getAllByRole("link")).toHaveLength(8);
  });

  it.each<[DbRole, string[]]>([
    ["ATM-USER", ["/dsr-late-report", "/replenishment"]],
    ["BRANCH-ATM-USER", ["/replenishment"]],
    ["BRANCH-ATM-SPV", ["/dsr-late-report", "/replenishment"]],
  ])("%s sees only reports (no Data master group)", (role, hrefs) => {
    renderAs(role);
    expect(screen.queryByRole("heading", { name: "Data master" })).not.toBeInTheDocument();
    const got = screen.getAllByRole("link").map((l) => l.getAttribute("href"));
    expect(got.sort()).toEqual([...hrefs].sort());
  });

  it("links master datasets to /exports/<slug> and reports to their screens", () => {
    renderAs("ADMIN");
    for (const [slug, label] of Object.entries(MASTER)) {
      expect(hrefOf(label)).toBe(`/exports/${slug}`);
    }
    for (const [href, label] of Object.entries(REPORTS)) expect(hrefOf(label)).toBe(href);
  });

  it("shows the Ekspor Saya section", () => {
    renderAs("BRANCH-ATM-USER");
    expect(screen.getByRole("heading", { name: "Ekspor Saya" })).toBeInTheDocument();
    expect(screen.getByText("tabel-ekspor-saya")).toBeInTheDocument();
  });

  it("gives each link a readable name and reaches every link with Tab", async () => {
    renderAs("ADMIN");
    const user = userEvent.setup();
    const links = screen.getAllByRole("link");
    for (const link of links) {
      expect((link.textContent ?? "").trim().length).toBeGreaterThan(0);
      await user.tab();
      expect(link).toHaveFocus();
    }
    const master = screen.getByRole("region", { name: "Data master" });
    expect(within(master).getAllByRole("listitem")).toHaveLength(8);
  });
});
