import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

const apiMock = vi.hoisted(() => ({ listMyExports: vi.fn() }));
vi.mock("../api", async (orig) => ({ ...(await orig<typeof import("../api")>()), ...apiMock }));

import { MyExportsPage } from "./MyExportsPage";

describe("MyExportsPage", () => {
  it("renders the heading and the user's export table", async () => {
    apiMock.listMyExports.mockResolvedValue([]);
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={qc}>
        <MyExportsPage />
      </QueryClientProvider>,
    );
    expect(screen.getByRole("heading", { name: "Ekspor Saya" })).toBeInTheDocument();
    expect(await screen.findByText(/Belum ada ekspor/)).toBeInTheDocument();
  });

  it("lists report and master-data jobs together", async () => {
    const base = {
      vendor_id: null,
      vendor_code: null,
      branch_id: null,
      branch_code: null,
      status_filter: null,
      price_level: null,
      report_params: null,
      status: "downloaded",
      row_count: 5,
      byte_size: null,
      error_message: null,
      created_at: "2026-10-29T02:00:00Z",
      started_at: null,
      finished_at: null,
      expires_at: null,
      downloaded_at: null,
    };
    apiMock.listMyExports.mockResolvedValue([
      { ...base, id: 1, kind: "dsr_late_summary", entity: "summary", file_name: "rekap.csv" },
      { ...base, id: 2, kind: "cabang", entity: "vaults", file_name: "vault.csv" },
    ]);
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={qc}>
        <MyExportsPage />
      </QueryClientProvider>,
    );
    expect(await screen.findAllByText("rekap.csv")).not.toHaveLength(0);
    expect(screen.getAllByText("vault.csv")).not.toHaveLength(0);
    expect(screen.getByText("Laporan DSR telat (rekap bulanan)")).toBeInTheDocument();
  });
});
