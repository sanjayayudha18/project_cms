import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

// fe-coverage C4: the vendor's view of one day's DSR upload (daily balance + rencana isi).
// The HTTP client is mocked, so the real fetchers (paths) and the real dialog run.
const client = vi.hoisted(() => ({ get: vi.fn() }));
vi.mock("@/lib/api/client", () => ({ api: client }));

import { DsrDetailDialog } from "../DsrDetailDialog";
import { type DsrUploadListItem, listDsrUploads } from "../dsrUploadApi";

const sheet = (fileId: number) => ({ file_id: fileId }) as DsrUploadListItem["daily"];

function renderDialog(item: DsrUploadListItem, onClose = vi.fn()) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={queryClient}>
      <DsrDetailDialog item={item} onClose={onClose} />
    </QueryClientProvider>,
  );
  return onClose;
}

beforeEach(() => {
  client.get.mockReset();
  client.get.mockImplementation(async (path: string) => {
    if (path.includes("/daily/")) {
      return {
        data: {
          daily_rows: [
            { row_no: 1, line_label: "Saldo awal", line_total_idr: "1250000000.50" },
            { row_no: 2, line_label: "Setoran", line_total_idr: null },
          ],
        },
      };
    }
    return {
      data: {
        error_count: 2,
        rencana_isi_rows: [
          {
            row_no: 1,
            atm_terminal_id: "T001",
            atm_location: null,
            fill_100k_idr: "300000000",
            fill_50k_idr: "0",
          },
        ],
      },
    };
  });
});

describe("DsrDetailDialog", () => {
  it("loads both sheets and shows amounts as exact grouped decimals", async () => {
    renderDialog({ report_date: "2026-10-14", daily: sheet(11), rencana_isi: sheet(12) });

    expect(screen.getByRole("dialog", { name: "Data DSR — 2026-10-14" })).toBeInTheDocument();
    expect(screen.getByText("Memuat data...")).toBeInTheDocument();

    expect(await screen.findByText("1.250.000.000,50")).toBeInTheDocument();
    expect(screen.getByText("Saldo awal")).toBeInTheDocument();
    expect(screen.getAllByText("-")).toHaveLength(2); // null total + missing location
    expect(screen.getByText("300.000.000")).toBeInTheDocument();
    expect(screen.getByText("T001")).toBeInTheDocument();
    expect(screen.getByText(/2 baris dilewati/)).toBeInTheDocument();
    expect(client.get.mock.calls.map((c) => c[0]).sort()).toEqual([
      "/api/v1/dsr/uploads/daily/11",
      "/api/v1/dsr/uploads/rencana-isi/12",
    ]);
  });

  it("fetches only the sheet that was uploaded", async () => {
    renderDialog({ report_date: "2026-10-14", daily: sheet(11), rencana_isi: null });

    expect(await screen.findByText("Saldo Harian (Daily)")).toBeInTheDocument();
    expect(screen.queryByText("Rencana Isi")).not.toBeInTheDocument();
    expect(client.get).toHaveBeenCalledTimes(1);
  });

  it("closes with the button, Escape and an overlay click — not a click inside", async () => {
    const onClose = renderDialog({
      report_date: "2026-10-14",
      daily: null,
      rencana_isi: sheet(12),
    });
    await screen.findByText("Rencana Isi");

    await userEvent.setup().click(screen.getByText("Rencana Isi"));
    expect(onClose).not.toHaveBeenCalled();

    await userEvent.setup().click(screen.getByRole("button", { name: "Tutup" }));
    fireEvent.keyDown(document, { key: "Escape" });
    fireEvent.click(screen.getByRole("dialog"));
    expect(onClose).toHaveBeenCalledTimes(3);
  });
});

describe("listDsrUploads", () => {
  it("adds the date filters only when set", async () => {
    client.get.mockResolvedValue({ data: { data: [], total: 0 } });

    await listDsrUploads({});
    await listDsrUploads({ dateFrom: "2026-10-01", dateTo: "2026-10-15" });

    expect(client.get.mock.calls.map((c) => c[0])).toEqual([
      "/api/v1/dsr/uploads",
      "/api/v1/dsr/uploads?date_from=2026-10-01&date_to=2026-10-15",
    ]);
  });
});
