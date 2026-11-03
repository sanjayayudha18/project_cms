import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const apiMock = vi.hoisted(() => ({
  requestExport: vi.fn(),
  listMyExports: vi.fn(),
  downloadExportFile: vi.fn(),
}));
vi.mock("../api", async (orig) => ({ ...(await orig<typeof import("../api")>()), ...apiMock }));

import type { ExportJob, ExportStatus } from "../types";
import { ExportStatusBadge } from "./ExportStatusBadge";
import { MyExportsTable, remainingText } from "./MyExportsTable";

const NOW = new Date("2026-10-29T03:00:00Z");

const mkJob = (id: number, status: ExportStatus, extra: Partial<ExportJob> = {}): ExportJob => ({
  id,
  kind: "cabang",
  entity: "vaults",
  vendor_id: 7,
  vendor_code: "V7",
  branch_id: 3,
  branch_code: "B3",
  status_filter: "active",
  price_level: null,
  report_params: null,
  file_name: `file-${id}.csv`,
  status,
  row_count: 1234,
  byte_size: null,
  error_message: null,
  created_at: "2026-10-29T02:00:00Z",
  started_at: null,
  finished_at: null,
  expires_at: null,
  downloaded_at: null,
  ...extra,
});

function renderTable(jobs: ExportJob[]) {
  apiMock.listMyExports.mockResolvedValue(jobs);
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MyExportsTable />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  for (const m of Object.values(apiMock)) m.mockReset();
});
afterEach(() => vi.useRealTimers());

describe("ExportStatusBadge", () => {
  const labels: [ExportStatus, string][] = [
    ["queued", "Antri"],
    ["running", "Diproses"],
    ["succeeded", "Siap diunduh"],
    ["downloading", "Sedang diunduh"],
    ["downloaded", "Sudah diunduh"],
    ["failed", "Gagal"],
    ["expired", "Kedaluwarsa"],
  ];
  it.each(labels)("renders text for %s", (status, text) => {
    render(<ExportStatusBadge status={status} />);
    expect(screen.getByText(text)).toBeInTheDocument();
  });
});

describe("remainingText", () => {
  it("formats minutes, sub-minute, and past", () => {
    const now = NOW.getTime();
    expect(remainingText(new Date(now + 9 * 60_000).toISOString(), now)).toBe("9 menit lagi");
    expect(remainingText(new Date(now + 20_000).toISOString(), now)).toBe("kurang dari 1 menit");
    expect(remainingText(new Date(now - 1).toISOString(), now)).toBe("sudah lewat");
    expect(remainingText(null, now)).toBe("—");
  });
});

describe("MyExportsTable", () => {
  it("shows loading then the empty state", async () => {
    renderTable([]);
    expect(screen.getByRole("status")).toHaveTextContent("Memuat");
    expect(await screen.findByText(/Belum ada ekspor/)).toBeInTheDocument();
  });

  it("shows an error state with retry", async () => {
    apiMock.listMyExports.mockRejectedValue(new Error("boom"));
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={qc}>
        <MyExportsTable />
      </QueryClientProvider>,
    );
    expect(await screen.findByRole("alert")).toHaveTextContent("Gagal memuat daftar ekspor.");
    expect(screen.getByRole("button", { name: "Coba lagi" })).toBeInTheDocument();
  });

  it("renders columns, WIB times, right-aligned tabular row count and error text", async () => {
    renderTable([
      mkJob(1, "failed", { error_message: "Disk penuh" }),
      mkJob(2, "downloaded", { downloaded_at: "2026-10-29T02:30:00Z" }),
    ]);
    await screen.findAllByText("file-1.csv");
    for (const h of [
      "Nama file",
      "Jenis",
      "Diminta",
      "Status",
      "Baris",
      "Batas unduh",
      "Diunduh",
      "Keterangan",
    ])
      expect(screen.getByRole("columnheader", { name: h })).toBeInTheDocument();
    const row = screen.getAllByText("file-1.csv")[0].closest("tr") as HTMLElement;
    expect(within(row).getByText("Cabang · vaults")).toBeInTheDocument();
    expect(within(row).getByText(/09:00 WIB/)).toBeInTheDocument(); // 02:00Z = 09:00 WIB
    expect(within(row).getByText("Disk penuh")).toBeInTheDocument();
    const rows = within(row).getByText("1.234");
    expect(rows).toHaveClass("tabular-nums");
    expect(rows.closest("td")).toHaveClass("text-right");
    const row2 = screen.getAllByText("file-2.csv")[0].closest("tr") as HTMLElement;
    expect(within(row2).getByText(/09:30 WIB/)).toBeInTheDocument();
  });

  it("offers Unduh only for succeeded, downloads, and refetches", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true, now: NOW });
    const exp = new Date(NOW.getTime() + 9 * 60_000).toISOString();
    renderTable([
      mkJob(1, "succeeded", { expires_at: exp }),
      mkJob(2, "downloaded"),
      mkJob(3, "queued"),
    ]);
    await screen.findAllByText("file-1.csv");
    expect(screen.getByText("9 menit lagi")).toBeInTheDocument();
    expect(
      screen.getByText("Hanya bisa diunduh satu kali, lalu dihapus dari server."),
    ).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: /^Unduh/ })).toHaveLength(1);

    apiMock.downloadExportFile.mockResolvedValue(undefined);
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    await user.click(screen.getByRole("button", { name: "Unduh file-1.csv" }));
    expect(apiMock.downloadExportFile).toHaveBeenCalledWith(1, "file-1.csv");
    await waitFor(() => expect(apiMock.listMyExports).toHaveBeenCalledTimes(2));
  });

  it("counts remaining time down as the clock advances", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true, now: NOW });
    renderTable([
      mkJob(1, "succeeded", { expires_at: new Date(NOW.getTime() + 9 * 60_000).toISOString() }),
    ]);
    await screen.findByText("9 menit lagi");
    act(() => {
      vi.advanceTimersByTime(3 * 60_000);
    });
    expect(await screen.findByText("6 menit lagi")).toBeInTheDocument();
  });

  it("offers Minta ekspor lagi only for downloaded/expired, with the same params", async () => {
    renderTable([
      mkJob(1, "downloaded"),
      mkJob(2, "expired", {
        kind: "harga_paket",
        entity: "package-prices",
        branch_id: 3,
        price_level: "branch",
        status_filter: "all",
      }),
      mkJob(3, "succeeded"),
      mkJob(4, "failed"),
    ]);
    await screen.findAllByText("file-1.csv");
    expect(screen.getAllByRole("button", { name: /^Minta ekspor lagi/ })).toHaveLength(2);

    apiMock.requestExport.mockResolvedValue({ job: mkJob(9, "queued"), created: true });
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Minta ekspor lagi file-1.csv" }));
    expect(apiMock.requestExport.mock.lastCall?.[0]).toEqual({
      kind: "cabang",
      vendorId: 7,
      branchId: 3,
      entity: "vaults",
      status: "active",
    });
    await user.click(screen.getByRole("button", { name: "Minta ekspor lagi file-2.csv" }));
    expect(apiMock.requestExport.mock.lastCall?.[0]).toEqual({
      kind: "harga_paket",
      vendorId: 7,
      branchId: 3,
      priceLevel: "branch",
      status: "all",
    });
  });

  it("labels report jobs and re-requests them with report_params", async () => {
    const reportParams = { month: "2026-10", vendor_id: 4 };
    renderTable([
      mkJob(1, "expired", {
        kind: "dsr_late_summary",
        entity: "summary",
        vendor_id: null,
        branch_id: null,
        status_filter: null,
        report_params: reportParams,
      }),
      mkJob(2, "downloaded", {
        kind: "replenishment_trips",
        entity: "trips",
        vendor_id: null,
        branch_id: null,
        status_filter: null,
        report_params: null,
      }),
    ]);
    await screen.findAllByText("file-1.csv");
    expect(screen.getByText("Laporan DSR telat (rekap bulanan)")).toBeInTheDocument();
    expect(screen.getByText("Replenish: trip per ATM")).toBeInTheDocument();
    // Job 2 has no report_params -> no re-request button.
    expect(screen.getAllByRole("button", { name: /^Minta ekspor lagi/ })).toHaveLength(1);

    apiMock.requestExport.mockResolvedValue({ job: mkJob(9, "queued"), created: true });
    await userEvent.setup().click(screen.getByRole("button", { name: /^Minta ekspor lagi/ }));
    expect(apiMock.requestExport.mock.lastCall?.[0]).toEqual({
      kind: "report",
      report: "dsr-late-summary",
      params: reportParams,
    });
  });

  it("shows a 429 inline on the row, with icon-bearing alert text", async () => {
    renderTable([mkJob(1, "expired"), mkJob(2, "expired")]);
    await screen.findAllByText("file-1.csv");
    apiMock.requestExport.mockRejectedValue({
      status: 429,
      code: "too_many_exports",
      message: "Terlalu banyak ekspor aktif.",
    });
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Minta ekspor lagi file-1.csv" }));
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Terlalu banyak ekspor aktif.");
    expect(alert.closest("tr")).toHaveTextContent("file-1.csv");
    expect(screen.getAllByRole("alert")).toHaveLength(1);
  });
});
