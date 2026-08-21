import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { EnrichedForecastRecord } from "../types";

// ─── Mock useForecastData hook ───────────────────────────────────────────────

const mockUseForecastData = vi.fn();

vi.mock("../useForecastData", () => ({
  useForecastData: () => mockUseForecastData(),
}));

// ─── Import components after mocks ───────────────────────────────────────────

import { ForecastTable } from "../ForecastTable";
import { ForecastView } from "../ForecastView";
import { ScheduleList } from "../ScheduleList";

// ─── Test Data ───────────────────────────────────────────────────────────────

const mockForecastRecords: EnrichedForecastRecord[] = [
  {
    id: "FC-001",
    atmId: "ATM-JKT-001",
    currentBalance: 500000000,
    predictedUsageH1: 120000000,
    predictedUsageH2: 110000000,
    recommendedReplenishment: 250000000,
    priority: "High",
    location: "Sudirman Plaza",
    vendorName: "PT Gardanet",
  },
  {
    id: "FC-002",
    atmId: "ATM-BDG-001",
    currentBalance: 800000000,
    predictedUsageH1: 80000000,
    predictedUsageH2: 75000000,
    recommendedReplenishment: 0,
    priority: "Low",
    location: "Dago Plaza",
    vendorName: "PT G4S",
  },
  {
    id: "FC-003",
    atmId: "ATM-SBY-001",
    currentBalance: 300000000,
    predictedUsageH1: 100000000,
    predictedUsageH2: 95000000,
    recommendedReplenishment: 150000000,
    priority: "Medium",
    location: "Tunjungan Plaza",
    vendorName: "PT SSI",
  },
];

// ─── ForecastTable Tests ─────────────────────────────────────────────────────

describe("ForecastTable", () => {
  it("renders all column headers", () => {
    render(<ForecastTable data={mockForecastRecords} />);

    expect(screen.getByText("ATM ID")).toBeInTheDocument();
    expect(screen.getByText("Lokasi")).toBeInTheDocument();
    expect(screen.getByText("Vendor")).toBeInTheDocument();
    expect(screen.getByText("Saldo Saat Ini")).toBeInTheDocument();
    expect(screen.getByText("Prediksi H+1")).toBeInTheDocument();
    expect(screen.getByText("Prediksi H+2")).toBeInTheDocument();
    expect(screen.getByText("Rekomendasi Pengisian")).toBeInTheDocument();
    expect(screen.getByText("Prioritas")).toBeInTheDocument();
  });

  it("renders forecast records with correct ATM IDs and locations", () => {
    render(<ForecastTable data={mockForecastRecords} />);

    expect(screen.getByText("ATM-JKT-001")).toBeInTheDocument();
    expect(screen.getByText("ATM-BDG-001")).toBeInTheDocument();
    expect(screen.getByText("ATM-SBY-001")).toBeInTheDocument();

    expect(screen.getByText("Sudirman Plaza")).toBeInTheDocument();
    expect(screen.getByText("Dago Plaza")).toBeInTheDocument();
    expect(screen.getByText("Tunjungan Plaza")).toBeInTheDocument();
  });

  it("renders vendor names", () => {
    render(<ForecastTable data={mockForecastRecords} />);

    expect(screen.getByText("PT Gardanet")).toBeInTheDocument();
    expect(screen.getByText("PT G4S")).toBeInTheDocument();
    expect(screen.getByText("PT SSI")).toBeInTheDocument();
  });

  it("renders IDR-formatted currency values", () => {
    render(<ForecastTable data={mockForecastRecords} />);

    // currentBalance of FC-001: 500000000
    expect(screen.getByText("500.000.000")).toBeInTheDocument();
    // recommendedReplenishment of FC-001: 250000000
    expect(screen.getByText("250.000.000")).toBeInTheDocument();
  });

  it("renders priority badges with correct labels", () => {
    render(<ForecastTable data={mockForecastRecords} />);

    expect(screen.getByText("High")).toBeInTheDocument();
    expect(screen.getByText("Medium")).toBeInTheDocument();
    expect(screen.getByText("Low")).toBeInTheDocument();
  });

  it("renders empty message when data is empty", () => {
    render(<ForecastTable data={[]} />);

    expect(
      screen.getByText("Tidak ada ATM yang sesuai dengan prioritas yang dipilih"),
    ).toBeInTheDocument();
  });
});

// ─── ScheduleList Tests ──────────────────────────────────────────────────────

describe("ScheduleList", () => {
  it("renders the section heading", () => {
    render(<ScheduleList />);

    expect(screen.getByText("Jadwal Pengisian (3 Hari ke Depan)")).toBeInTheDocument();
  });

  it("renders grouped vendor names", () => {
    render(<ScheduleList />);

    expect(screen.getByText("PT G4S")).toBeInTheDocument();
    expect(screen.getByText("PT Gardanet")).toBeInTheDocument();
    expect(screen.getByText("PT SSI")).toBeInTheDocument();
  });

  it("renders ATM IDs and locations for schedule entries", () => {
    render(<ScheduleList />);

    // Check some specific entries
    expect(screen.getByText(/ATM-JKT-001 — Sudirman Plaza/)).toBeInTheDocument();
    expect(screen.getByText(/ATM-JKT-002 — Thamrin City/)).toBeInTheDocument();
    expect(screen.getByText(/ATM-BDG-001 — Dago Plaza/)).toBeInTheDocument();
  });

  it("renders formatted IDR amounts for schedule entries", () => {
    render(<ScheduleList />);

    // amount 250000000 appears multiple times
    const amounts = screen.getAllByText("250.000.000");
    expect(amounts.length).toBeGreaterThanOrEqual(1);

    // amount 350000000
    const largeAmounts = screen.getAllByText("350.000.000");
    expect(largeAmounts.length).toBeGreaterThanOrEqual(1);
  });

  it("renders formatted dates in id-ID locale", () => {
    render(<ScheduleList />);

    // The scheduleData has dates 2024-01-21, 2024-01-22, 2024-01-23
    // Each date appears across multiple vendors, so use getAllByText
    const jan21 = screen.getAllByText(/21 Jan 2024/);
    expect(jan21.length).toBeGreaterThanOrEqual(1);

    const jan22 = screen.getAllByText(/22 Jan 2024/);
    expect(jan22.length).toBeGreaterThanOrEqual(1);

    const jan23 = screen.getAllByText(/23 Jan 2024/);
    expect(jan23.length).toBeGreaterThanOrEqual(1);
  });
});

// ─── ForecastView Integration Tests ──────────────────────────────────────────

describe("ForecastView", () => {
  beforeEach(() => {
    mockUseForecastData.mockReset();
  });

  it("renders page header and table when data is available", () => {
    mockUseForecastData.mockReturnValue({
      data: mockForecastRecords,
      isError: false,
    });

    render(<ForecastView />);

    // Page title
    expect(screen.getByText("Forecast")).toBeInTheDocument();
    expect(
      screen.getByText("Proyeksi kebutuhan kas dan rekomendasi pengisian"),
    ).toBeInTheDocument();

    // Table should render data
    expect(screen.getByText("ATM-JKT-001")).toBeInTheDocument();
  });

  it("renders ScheduleList section alongside the table", () => {
    mockUseForecastData.mockReturnValue({
      data: mockForecastRecords,
      isError: false,
    });

    render(<ForecastView />);

    // ScheduleList heading should be visible
    expect(screen.getByText("Jadwal Pengisian (3 Hari ke Depan)")).toBeInTheDocument();
  });

  it("displays EmptyState when data is empty array", () => {
    mockUseForecastData.mockReturnValue({
      data: [],
      isError: false,
    });

    render(<ForecastView />);

    expect(screen.getByText("Tidak ada data forecast tersedia.")).toBeInTheDocument();
  });

  it("displays error state when isError is true", () => {
    mockUseForecastData.mockReturnValue({
      data: undefined,
      isError: true,
    });

    render(<ForecastView />);

    expect(
      screen.getByText("Gagal memuat data forecast. Silakan periksa file data."),
    ).toBeInTheDocument();
  });

  it("renders filter select and summary card", () => {
    mockUseForecastData.mockReturnValue({
      data: mockForecastRecords,
      isError: false,
    });

    render(<ForecastView />);

    // Filter select is present (combobox with priority options)
    expect(screen.getByRole("combobox")).toBeInTheDocument();
    expect(screen.getByText("Semua Prioritas")).toBeInTheDocument();

    // Summary card label
    expect(screen.getByText("Total Rekomendasi Pengisian")).toBeInTheDocument();
  });
});
