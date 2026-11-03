/**
 * fe-coverage C4: ATM profile page with its real header, tabs and history tables.
 * Data hooks, URL state and the visit-quota card are mocked (they have own tests).
 * Money is checked as exact decimal strings → Rupiah / whole numbers (no float).
 */

import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const m = vi.hoisted(() => ({
  params: { terminalId: "T001" } as { terminalId?: string },
  tab: "replenish" as "replenish" | "cashpos",
  setTab: vi.fn(),
  setHistoryParams: vi.fn(),
  master: {} as Record<string, unknown>,
  replenish: {} as Record<string, unknown>,
  cashpos: {} as Record<string, unknown>,
  refetch: vi.fn(),
}));

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children }: { children: ReactNode }) => <a href="/atm-portal">{children}</a>,
  useParams: () => m.params,
}));
vi.mock("../useAtmProfileUrlState", () => ({
  useAtmProfileUrlState: () => ({
    tab: m.tab,
    historyParams: { page: 1, page_size: 25, date_from: "2026-10-01", date_to: "2026-10-15" },
    setTab: m.setTab,
    setHistoryParams: m.setHistoryParams,
  }),
}));
vi.mock("../useAtmProfileData", () => ({
  useAtmMasterData: () => ({ ...m.master, refetch: m.refetch }),
  useAtmReplenishHistory: () => ({ ...m.replenish, refetch: m.refetch }),
  useAtmCashposHistory: () => ({ ...m.cashpos, refetch: m.refetch }),
}));
vi.mock("../components/VisitQuotaCard", () => ({
  VisitQuotaCard: ({ terminalId }: { terminalId: string }) => <section>kuota {terminalId}</section>,
}));

import { AtmProfileScreen } from "../AtmProfileScreen";

const MASTER = {
  terminal_id: "T001",
  location_name: "KCP Sudirman",
  address: "Jl. Sudirman 1",
  machine_type: "ATM",
  brand: "NCR",
  model: "SelfServ 80",
  deployment_type: "onsite",
  operation_hours: "24 jam",
  capacity_amount: "1500000000.00",
  low_threshold_amount: "250000000.50",
  critical_threshold_amount: null,
  is_active: true,
  replenishment_status: "normal",
};

const REPLENISH_ROW = {
  replenish_date: "2026-10-14",
  replenish_time: "09:30",
  terminal_id: "T001",
  machine_type: "ATM",
  teller_id: "TL1",
  branch_code: "B01",
  escrow: "0",
  refund_denom_10k: "0",
  refund_denom_20k: "0",
  refund_denom_50k: "1250000.00",
  refund_denom_100k: "0",
  refund_total: "1250000.00",
  replenish_denom_10k: "0",
  replenish_denom_20k: "0",
  replenish_denom_50k: "0",
  replenish_denom_100k: "300000000.00",
  replenish_total: "300000000.00",
};

function cashposRow() {
  const row: Record<string, unknown> = {
    id: 1,
    file_id: 2,
    cashpos_date: "2026-10-13",
    terminal_id: "T001",
    machine_type: "ATM",
    teller_id: "TL1",
    branch_code: "B01",
  };
  for (const p of ["starting_cash", "cash_in", "cash_out", "cash_position"]) {
    for (const d of ["10k", "20k", "50k", "100k"]) row[`${p}_${d}`] = "0.00";
  }
  row.cash_position_100k = "987654321.99";
  return row;
}

beforeEach(() => {
  m.params = { terminalId: "T001" };
  m.tab = "replenish";
  m.setTab.mockReset();
  m.setHistoryParams.mockReset();
  m.refetch.mockReset();
  m.master = { isLoading: false, isError: false, data: MASTER };
  m.replenish = { isLoading: false, isError: false, data: { data: [REPLENISH_ROW], total: 1 } };
  m.cashpos = { isLoading: false, isError: false, data: { data: [cashposRow()], total: 1 } };
});

describe("AtmProfileScreen", () => {
  it("shows not-found without a terminal id and for a 404", () => {
    m.params = {};
    const { rerender } = render(<AtmProfileScreen />);
    expect(screen.getByText("ATM tidak ditemukan")).toBeInTheDocument();

    m.params = { terminalId: "NOPE" };
    m.master = { isLoading: false, isError: true, error: { status: 404 }, data: undefined };
    rerender(<AtmProfileScreen />);
    expect(screen.getByText("ATM tidak ditemukan")).toBeInTheDocument();
  });

  it("shows a retryable error for other failures", async () => {
    m.master = { isLoading: false, isError: true, error: { status: 500 }, data: undefined };
    render(<AtmProfileScreen />);

    expect(screen.getByText("Gagal memuat data ATM")).toBeInTheDocument();
    await userEvent.setup().click(screen.getByRole("button", { name: "Coba Lagi" }));
    expect(m.refetch).toHaveBeenCalled();
  });

  it("renders breadcrumb, master data with exact Rupiah amounts, and the quota card", () => {
    render(<AtmProfileScreen />);

    const breadcrumb = screen.getByRole("navigation", { name: "Breadcrumb" });
    expect(within(breadcrumb).getByText("T001")).toHaveAttribute("aria-current", "page");
    const header = screen.getAllByLabelText("Data ATM")[0] as HTMLElement;
    expect(header).toHaveTextContent("KCP Sudirman");
    expect(header).toHaveTextContent("Rp 1.500.000.000");
    expect(header).toHaveTextContent("Rp 250.000.000,50");
    expect(header).toHaveTextContent("Aktif");
    expect(screen.getByText("kuota T001")).toBeInTheDocument();
  });

  it("replenish tab: rows with Indonesian date and Rupiah denominations", () => {
    render(<AtmProfileScreen />);

    const table = screen.getByRole("table", { name: "Riwayat Replenish" });
    expect(within(table).getByText("14 Okt 2026")).toBeInTheDocument();
    expect(within(table).getAllByText("Rp 1.250.000")).toHaveLength(2);
    expect(within(table).getAllByText("Rp 300.000.000")).toHaveLength(2);
    expect(screen.getByText("Menampilkan 1 baris replenish")).toBeInTheDocument();
  });

  it("changing the date range resets to page 1", () => {
    render(<AtmProfileScreen />);

    fireEvent.change(screen.getByLabelText("Dari Tanggal"), { target: { value: "2026-09-01" } });
    fireEvent.change(screen.getByLabelText("Sampai Tanggal"), { target: { value: "2026-09-30" } });

    expect(m.setHistoryParams.mock.calls).toEqual([
      [{ date_from: "2026-09-01", page: 1 }],
      [{ date_to: "2026-09-30", page: 1 }],
    ]);
  });

  it("replenish empty and error states", async () => {
    m.replenish = { isLoading: false, isError: false, data: { data: [], total: 0 } };
    const { rerender } = render(<AtmProfileScreen />);
    expect(screen.getByText("Tidak ada data replenish yang sesuai filter")).toBeInTheDocument();

    m.replenish = { isLoading: false, isError: true, data: undefined };
    rerender(<AtmProfileScreen />);
    // visible message + the screen-reader live region
    expect(screen.getAllByText("Gagal memuat data replenish")).toHaveLength(2);
    await userEvent.setup().click(screen.getByRole("button", { name: "Coba Lagi" }));
    expect(m.refetch).toHaveBeenCalled();
  });

  it("tabs switch by click and by arrow keys", async () => {
    render(<AtmProfileScreen />);

    const tabs = within(screen.getByRole("tablist", { name: "Riwayat ATM" }));
    expect(tabs.getByRole("tab", { name: "Replenish" })).toHaveAttribute("aria-selected", "true");
    await userEvent.setup().click(tabs.getByRole("tab", { name: "Cashpos" }));
    expect(m.setTab).toHaveBeenLastCalledWith("cashpos");

    fireEvent.keyDown(tabs.getByRole("tab", { name: "Replenish" }), { key: "ArrowRight" });
    expect(m.setTab).toHaveBeenLastCalledWith("cashpos");
    fireEvent.keyDown(tabs.getByRole("tab", { name: "Replenish" }), { key: "End" });
    expect(m.setTab).toHaveBeenLastCalledWith("cashpos");
  });

  it("cashpos tab: whole-number denominations, never rounded up", () => {
    m.tab = "cashpos";
    render(<AtmProfileScreen />);

    const table = screen.getByRole("table", { name: "Riwayat Cash Position" });
    expect(within(table).getByText("987.654.321")).toBeInTheDocument();
    expect(within(table).getByText("13 Okt 2026")).toBeInTheDocument();
    expect(screen.getByText("Menampilkan 1 baris cash position")).toBeInTheDocument();
  });

  it("cashpos rejects an end date before the start date and keeps the URL as is", () => {
    m.tab = "cashpos";
    render(<AtmProfileScreen />);

    fireEvent.change(screen.getByLabelText("Sampai Tanggal"), { target: { value: "2026-09-01" } });

    expect(screen.getByText("Tanggal akhir tidak boleh sebelum tanggal awal")).toBeInTheDocument();
    expect(m.setHistoryParams).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText("Dari Tanggal"), { target: { value: "2026-08-01" } });
    expect(m.setHistoryParams).toHaveBeenCalledWith({
      date_from: "2026-08-01",
      date_to: "2026-09-01",
      page: 1,
    });
  });

  it("cashpos error state is retryable", async () => {
    m.tab = "cashpos";
    m.cashpos = { isLoading: false, isError: true, data: undefined };
    render(<AtmProfileScreen />);

    expect(screen.getByText("Gagal memuat data cashpos")).toBeInTheDocument();
    expect(screen.getByText("Gagal memuat data cash position")).toBeInTheDocument();
    await userEvent.setup().click(screen.getByRole("button", { name: "Coba Lagi" }));
    expect(m.refetch).toHaveBeenCalled();
  });
});
