import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { EnrichedDsrRecord } from "../types";

// ─── Mock useDsrData hook ────────────────────────────────────────────────────

const mockUseDsrData = vi.fn();

vi.mock("../useDsrData", () => ({
  useDsrData: () => mockUseDsrData(),
}));

// ─── Import components after mocks ───────────────────────────────────────────

import { DsrDashboard } from "../DsrDashboard";
import { DsrSummary } from "../DsrSummary";
import { DsrTable } from "../DsrTable";

// ─── Test Data ───────────────────────────────────────────────────────────────

const mockRecords: EnrichedDsrRecord[] = [
  {
    id: "DSR-001",
    atmId: "ATM-JKT-001",
    date: "2024-01-15",
    beginningBalance: 320000000,
    cashIn: 150000000,
    cashOut: 280000000,
    endingBalance: 190000000,
    status: "Normal",
    location: "Jakarta Selatan",
    vendorName: "PT Gardanet",
  },
  {
    id: "DSR-002",
    atmId: "ATM-JKT-002",
    date: "2024-01-15",
    beginningBalance: 180000000,
    cashIn: 50000000,
    cashOut: 160000000,
    endingBalance: 70000000,
    status: "Low",
    location: "Jakarta Barat",
    vendorName: "PT SSI",
  },
  {
    id: "DSR-003",
    atmId: "ATM-BDG-001",
    date: "2024-01-15",
    beginningBalance: 250000000,
    cashIn: 100000000,
    cashOut: 310000000,
    endingBalance: 40000000,
    status: "Critical",
    location: "Bandung",
    vendorName: "PT G4S",
  },
];

// ─── DsrSummary Tests ────────────────────────────────────────────────────────

describe("DsrSummary", () => {
  it("renders correct aggregated totals", () => {
    render(<DsrSummary data={mockRecords} />);

    // Total Beginning Balance = 320M + 180M + 250M = 750M
    // Total Cash Out = 280M + 160M + 310M = 750M
    // Both are 750.000.000 — should appear twice
    const totals750 = screen.getAllByText("750.000.000");
    expect(totals750).toHaveLength(2);

    // Total Cash In = 150M + 50M + 100M = 300M
    // Total Ending Balance = 190M + 70M + 40M = 300M
    // Both are 300.000.000 — should appear twice
    const totals300 = screen.getAllByText("300.000.000");
    expect(totals300).toHaveLength(2);

    // Verify the labels are rendered alongside the values
    // biome-ignore lint/style/noNonNullAssertion: closest("div") always resolves in DsrSummary's known markup.
    const beginningCard = screen.getByText("Total Saldo Awal").closest("div")!;
    expect(beginningCard).toHaveTextContent("750.000.000");

    // biome-ignore lint/style/noNonNullAssertion: closest("div") always resolves here — same card markup as above.
    const cashInCard = screen.getByText("Total Kas Masuk").closest("div")!;
    expect(cashInCard).toHaveTextContent("300.000.000");

    // biome-ignore lint/style/noNonNullAssertion: closest("div") always resolves here — same card markup as above.
    const cashOutCard = screen.getByText("Total Kas Keluar").closest("div")!;
    expect(cashOutCard).toHaveTextContent("750.000.000");

    // biome-ignore lint/style/noNonNullAssertion: closest("div") always resolves here — same card markup as above.
    const endingCard = screen.getByText("Total Saldo Akhir").closest("div")!;
    expect(endingCard).toHaveTextContent("300.000.000");
  });

  it("renders all four summary card labels", () => {
    render(<DsrSummary data={mockRecords} />);

    expect(screen.getByText("Total Saldo Awal")).toBeInTheDocument();
    expect(screen.getByText("Total Kas Masuk")).toBeInTheDocument();
    expect(screen.getByText("Total Kas Keluar")).toBeInTheDocument();
    expect(screen.getByText("Total Saldo Akhir")).toBeInTheDocument();
  });

  it("renders four cards in a grid", () => {
    const { container } = render(<DsrSummary data={mockRecords} />);

    const grid = container.querySelector(".grid");
    expect(grid).toBeInTheDocument();
    expect(grid?.children.length).toBe(4);
  });

  it("renders zero totals when data is empty", () => {
    render(<DsrSummary data={[]} />);

    // formatIDR(0) = "0"
    const zeroValues = screen.getAllByText("0");
    expect(zeroValues).toHaveLength(4);
  });
});

// ─── DsrTable Tests ──────────────────────────────────────────────────────────

describe("DsrTable", () => {
  it("renders all column headers correctly", () => {
    render(<DsrTable data={mockRecords} />);

    expect(screen.getByText("ATM ID")).toBeInTheDocument();
    expect(screen.getByText("Lokasi")).toBeInTheDocument();
    expect(screen.getByText("Vendor")).toBeInTheDocument();
    expect(screen.getByText("Saldo Awal")).toBeInTheDocument();
    expect(screen.getByText("Kas Masuk")).toBeInTheDocument();
    expect(screen.getByText("Kas Keluar")).toBeInTheDocument();
    expect(screen.getByText("Saldo Akhir")).toBeInTheDocument();
    expect(screen.getByText("Status")).toBeInTheDocument();
  });

  it("renders record data in table rows", () => {
    render(<DsrTable data={mockRecords} />);

    // ATM IDs
    expect(screen.getByText("ATM-JKT-001")).toBeInTheDocument();
    expect(screen.getByText("ATM-JKT-002")).toBeInTheDocument();
    expect(screen.getByText("ATM-BDG-001")).toBeInTheDocument();

    // Locations
    expect(screen.getByText("Jakarta Selatan")).toBeInTheDocument();
    expect(screen.getByText("Jakarta Barat")).toBeInTheDocument();
    expect(screen.getByText("Bandung")).toBeInTheDocument();

    // Vendor names
    expect(screen.getByText("PT Gardanet")).toBeInTheDocument();
    expect(screen.getByText("PT SSI")).toBeInTheDocument();
    expect(screen.getByText("PT G4S")).toBeInTheDocument();
  });

  it("renders monetary values as IDR-formatted", () => {
    render(<DsrTable data={mockRecords} />);

    // formatIDR(320000000) = "320.000.000"
    expect(screen.getByText("320.000.000")).toBeInTheDocument();
    // formatIDR(150000000) = "150.000.000"
    expect(screen.getByText("150.000.000")).toBeInTheDocument();
    // formatIDR(280000000) = "280.000.000"
    expect(screen.getByText("280.000.000")).toBeInTheDocument();
  });

  it("renders status badges with correct labels", () => {
    render(<DsrTable data={mockRecords} />);

    expect(screen.getByText("Normal")).toBeInTheDocument();
    expect(screen.getByText("Low")).toBeInTheDocument();
    expect(screen.getByText("Critical")).toBeInTheDocument();
  });

  it("renders empty message when data is empty", () => {
    render(<DsrTable data={[]} />);

    expect(screen.getByText("Tidak ada data DSR tersedia untuk tanggal ini.")).toBeInTheDocument();
  });
});

// ─── DsrDashboard Integration Tests ─────────────────────────────────────────

describe("DsrDashboard", () => {
  beforeEach(() => {
    mockUseDsrData.mockReset();
  });

  it("renders summary and table when data is available", () => {
    mockUseDsrData.mockReturnValue({ data: mockRecords, isLoading: false, isError: false });

    render(<DsrDashboard />);

    // Page title
    expect(screen.getByText("DSR Dashboard")).toBeInTheDocument();

    // Summary cards rendered
    expect(screen.getByText("Total Saldo Awal")).toBeInTheDocument();
    expect(screen.getByText("Total Kas Masuk")).toBeInTheDocument();

    // Table columns rendered
    expect(screen.getByText("ATM ID")).toBeInTheDocument();

    // Data rendered
    expect(screen.getByText("ATM-JKT-001")).toBeInTheDocument();
  });

  it("displays empty state when no records available", () => {
    mockUseDsrData.mockReturnValue({ data: [], isLoading: false, isError: false });

    render(<DsrDashboard />);

    expect(screen.getByText("Tidak ada data DSR tersedia untuk tanggal ini.")).toBeInTheDocument();
  });

  it("displays empty state via EmptyState component when data is empty array", () => {
    mockUseDsrData.mockReturnValue({ data: [], isLoading: false, isError: false });

    render(<DsrDashboard />);

    // EmptyState renders a paragraph with the message
    const emptyMsg = screen.getByText("Tidak ada data DSR tersedia untuk tanggal ini.");
    expect(emptyMsg).toBeInTheDocument();

    // Summary and table should NOT be rendered in empty state
    expect(screen.queryByText("Total Saldo Awal")).not.toBeInTheDocument();
    expect(screen.queryByText("ATM ID")).not.toBeInTheDocument();
  });

  it("displays error state when data fetch fails", () => {
    mockUseDsrData.mockReturnValue({ data: [], isLoading: false, isError: true });

    render(<DsrDashboard />);

    expect(screen.getByText("Gagal memuat data. Silakan periksa file data.")).toBeInTheDocument();
  });

  it("displays loading state while data is being fetched", () => {
    mockUseDsrData.mockReturnValue({ data: undefined, isLoading: true, isError: false });

    render(<DsrDashboard />);

    expect(screen.getByText("Memuat...")).toBeInTheDocument();
  });

  it("renders a date input for selecting the DSR date", () => {
    mockUseDsrData.mockReturnValue({ data: mockRecords, isLoading: false, isError: false });

    render(<DsrDashboard />);

    const dateInput = screen.getByLabelText("Tanggal");
    expect(dateInput).toBeInTheDocument();
    expect(dateInput).toHaveAttribute("type", "date");
  });
});
