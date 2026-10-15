import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ReplenishmentScreen } from "../ReplenishmentScreen";
import type { ClassificationParams, ClassificationView } from "../api";
import { rangeDays, rangeError } from "../api";

const { listParams, download, toast, listResult } = vi.hoisted(() => ({
  listParams: vi.fn(),
  download: vi.fn(),
  toast: vi.fn(),
  listResult: { current: null as unknown },
}));

vi.mock("../api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../api")>();
  return {
    ...actual,
    useClassifications: (p: ClassificationParams) => {
      listParams(p);
      return listResult.current;
    },
    downloadReport: (...args: unknown[]) => download(...args),
  };
});

vi.mock("@/features/vendor-request/hooks", () => ({
  useVendorOptions: () => ({ data: { vendors: [{ id: 2, name: "Abacus" }], regions: ["MDN"] } }),
}));

vi.mock("@/lib/hooks/useToast", () => ({ useToast: () => ({ toast }) }));

vi.mock("@tanstack/react-router", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    Link: ({ children }: { children?: ReactNode }) => <a href="/r">{children}</a>,
  };
});

const ROW: ClassificationView = {
  id: 1,
  ticket_number: "ATM1_100K_20261014_001",
  request_id: 7545,
  request_number: "REP-ABA-MDN-20261014-001",
  terminal_id: "ATM1",
  vendor_id: 2,
  vendor_name: "Abacus",
  branch_code: "ABA-MDN",
  branch_name: "Abacus Medan",
  region_code: "MDN",
  scheduled_date: "2026-10-14",
  realized_date: "2026-10-15",
  realized_time: "09:30:00",
  workday_offset: 1,
  classification: "late",
  is_final: true,
  has_amount_diff: true,
  vendor_mismatch: true,
  vendor_report_late: false,
  amounts: [
    {
      denom: 100000,
      order_amount: 200000000,
      itm_amount: 150000000,
      vendor_amount: null,
      diff: -50000000,
    },
  ],
  currency: "IDR",
};

function data(items: ClassificationView[], total = items.length) {
  return {
    isLoading: false,
    isError: false,
    data: { items, total, page: 1, page_size: 20 },
    refetch: vi.fn(),
  };
}

describe("ReplenishmentScreen", () => {
  beforeEach(() => {
    listParams.mockClear();
    download.mockReset();
    toast.mockClear();
    listResult.current = data([ROW]);
  });

  it("shows the classification with its label, offset, amounts and flags", () => {
    render(<ReplenishmentScreen />);
    expect(screen.getByText("Mundur (1 hari kerja terlambat)")).toBeInTheDocument();
    expect(screen.getByText("REP-ABA-MDN-20261014-001")).toBeInTheDocument();
    expect(screen.getByText("IDR 200.000.000")).toBeInTheDocument();
    expect(screen.getByText("IDR 150.000.000")).toBeInTheDocument();
    expect(screen.getByText(/-50\.000\.000/)).toBeInTheDocument();
    expect(screen.getByText("Beda laporan vendor")).toBeInTheDocument();
    expect(screen.queryByText("Laporan vendor telat")).not.toBeInTheDocument();
    expect(screen.getByText("09:30 WIB")).toBeInTheDocument();
  });

  it("defaults to the last 7 days and resets to page 1 when a filter changes", () => {
    render(<ReplenishmentScreen />);
    const first = listParams.mock.calls[0]?.[0] as ClassificationParams;
    expect(rangeDays(first.dateFrom, first.dateTo)).toBe(7);
    fireEvent.click(screen.getByRole("checkbox", { name: /selisih/i }));
    const last = listParams.mock.calls.at(-1)?.[0] as ClassificationParams;
    expect(last.onlyDiff).toBe(true);
    expect(last.page).toBe(1);
  });

  it("blocks a range over 31 days and the CSV buttons with it", () => {
    render(<ReplenishmentScreen />);
    fireEvent.change(screen.getByLabelText("Dari tanggal"), { target: { value: "2026-01-01" } });
    fireEvent.change(screen.getByLabelText("Sampai tanggal"), { target: { value: "2026-03-01" } });
    expect(screen.getByRole("alert")).toHaveTextContent("Rentang maksimal 31 hari");
    expect(screen.getByRole("button", { name: /Selisih nominal/ })).toBeDisabled();
  });

  it("downloads a report with the current filter and reports a failure", async () => {
    download.mockRejectedValueOnce(new Error("Anda tidak berhak"));
    render(<ReplenishmentScreen />);
    fireEvent.click(screen.getByRole("button", { name: /Trip per ATM/ }));
    await waitFor(() =>
      expect(toast).toHaveBeenCalledWith({ type: "error", message: "Anda tidak berhak" }),
    );
    expect(download.mock.calls[0]?.[0]).toBe("trips");
  });

  it("shows an empty state", () => {
    listResult.current = data([]);
    render(<ReplenishmentScreen />);
    expect(screen.getByText("Belum ada data realisasi untuk filter ini.")).toBeInTheDocument();
  });
});

describe("rangeError", () => {
  it("accepts up to 31 days and rejects reversed or longer ranges", () => {
    expect(rangeError("2026-10-01", "2026-10-31")).toBeNull();
    expect(rangeError("2026-10-01", "2026-11-01")).toMatch(/31 hari/);
    expect(rangeError("2026-10-05", "2026-10-01")).toMatch(/sebelum/);
    expect(rangeError("", "2026-10-01")).not.toBeNull();
  });
});
