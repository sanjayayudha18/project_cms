import { render, screen, fireEvent } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import { Banknote, Cpu, Landmark, Truck } from "lucide-react";

import type { StatsCardData, VendorDayFlow, VendorConfig } from "../types";

// ─── Mock recharts (jsdom cannot render SVG) ─────────────────────────────────

vi.mock("recharts", () => ({
  BarChart: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="bar-chart">{children}</div>
  ),
  Bar: ({ dataKey }: { dataKey: string }) => <div data-testid={`bar-${dataKey}`} />,
  XAxis: () => <div data-testid="x-axis" />,
  YAxis: () => <div data-testid="y-axis" />,
  CartesianGrid: () => <div data-testid="cartesian-grid" />,
  Tooltip: () => <div data-testid="tooltip" />,
  Legend: () => <div data-testid="legend" />,
  ResponsiveContainer: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="responsive-container">{children}</div>
  ),
}));

// ─── Mock useCashFlowData hook ───────────────────────────────────────────────

const mockUseCashFlowData = vi.fn();

vi.mock("../useCashFlowData", () => ({
  useCashFlowData: () => mockUseCashFlowData(),
}));

// ─── Mock @/components/ui/PageHeader ─────────────────────────────────────────

vi.mock("@/components/ui/PageHeader", () => ({
  PageHeader: ({ title }: { title: string }) => <h1>{title}</h1>,
}));

// ─── Mock @/components/ui/Badge ──────────────────────────────────────────────

vi.mock("@/components/ui/Badge", () => ({
  Badge: ({ label }: { label: string }) => <span data-testid="badge">{label}</span>,
}));

// ─── Import components after mocks ───────────────────────────────────────────

import { StatsCardGrid } from "../StatsCardGrid";
import { VendorBarChart } from "../VendorBarChart";
import { CashFlowScreen } from "../CashFlowScreen";

// ─── Test Data ───────────────────────────────────────────────────────────────

const mockStats: StatsCardData[] = [
  {
    label: "Total Kas Beredar",
    icon: Banknote,
    value: "Rp 48,2 M",
    trend: { direction: "up", percentage: 2.4 },
  },
  {
    label: "Saldo Vault Vendor",
    icon: Landmark,
    value: "Rp 21,7 M",
  },
  {
    label: "Kas di Mesin ATM",
    icon: Cpu,
    value: "Rp 26,5 M",
    trend: { direction: "down", percentage: 1.1 },
  },
  {
    label: "Drop CIT Hari Ini",
    icon: Truck,
    value: "Rp 3,9 M",
    subtitle: "6 order",
  },
];

const mockVendorData: VendorDayFlow[] = [
  { date: "2026-07-15", Abacus: 8.2, "Bijak Jakarta": 6.1, Advantage: 5.4, SSI: 3.9 },
  { date: "2026-07-16", Abacus: 7.5, "Bijak Jakarta": 6.8, Advantage: 5.1, SSI: 4.1 },
];

const mockVendors: VendorConfig[] = [
  { name: "Abacus", color: "oklch(0.58 0.12 245)" },
  { name: "Bijak Jakarta", color: "oklch(0.56 0.13 155)" },
  { name: "Advantage", color: "oklch(0.552 0.205 29)" },
  { name: "SSI", color: "oklch(0.76 0.15 78)" },
];

const mockCashFlowData = {
  stats: mockStats,
  vendorChart: { data: mockVendorData, vendors: mockVendors },
  atmLevels: [
    { id: "ATM-00417", label: "ATM-00417", percentage: 82 },
    { id: "ATM-00523", label: "ATM-00523", percentage: 65 },
  ],
};

// ─── StatsCardGrid Tests ─────────────────────────────────────────────────────

describe("StatsCardGrid", () => {
  it("renders correct number of stat cards matching the stats array length", () => {
    render(<StatsCardGrid stats={mockStats} />);

    expect(screen.getByText("Total Kas Beredar")).toBeInTheDocument();
    expect(screen.getByText("Saldo Vault Vendor")).toBeInTheDocument();
    expect(screen.getByText("Kas di Mesin ATM")).toBeInTheDocument();
    expect(screen.getByText("Drop CIT Hari Ini")).toBeInTheDocument();
  });

  it("renders stat values from ATM data correctly", () => {
    render(<StatsCardGrid stats={mockStats} />);

    expect(screen.getByText("Rp 48,2 M")).toBeInTheDocument();
    expect(screen.getByText("Rp 21,7 M")).toBeInTheDocument();
    expect(screen.getByText("Rp 26,5 M")).toBeInTheDocument();
    expect(screen.getByText("Rp 3,9 M")).toBeInTheDocument();
  });

  it("renders trend indicators when present", () => {
    render(<StatsCardGrid stats={mockStats} />);

    expect(screen.getByText("2.4%")).toBeInTheDocument();
    expect(screen.getByText("1.1%")).toBeInTheDocument();
  });

  it("renders subtitle when provided", () => {
    render(<StatsCardGrid stats={mockStats} />);

    expect(screen.getByText("6 order")).toBeInTheDocument();
  });

  it("renders empty grid with no stats", () => {
    const { container } = render(<StatsCardGrid stats={[]} />);

    // Grid container should exist but contain no cards
    const grid = container.querySelector(".grid");
    expect(grid).toBeInTheDocument();
    expect(grid?.children.length).toBe(0);
  });
});

// ─── VendorBarChart Tests ────────────────────────────────────────────────────

describe("VendorBarChart", () => {
  it("renders without errors with valid data", () => {
    render(<VendorBarChart data={mockVendorData} vendors={mockVendors} />);

    expect(screen.getByTestId("responsive-container")).toBeInTheDocument();
    expect(screen.getByTestId("bar-chart")).toBeInTheDocument();
  });

  it("renders a bar for each vendor", () => {
    render(<VendorBarChart data={mockVendorData} vendors={mockVendors} />);

    expect(screen.getByTestId("bar-Abacus")).toBeInTheDocument();
    expect(screen.getByTestId("bar-Bijak Jakarta")).toBeInTheDocument();
    expect(screen.getByTestId("bar-Advantage")).toBeInTheDocument();
    expect(screen.getByTestId("bar-SSI")).toBeInTheDocument();
  });

  it("renders chart axis elements", () => {
    render(<VendorBarChart data={mockVendorData} vendors={mockVendors} />);

    expect(screen.getByTestId("x-axis")).toBeInTheDocument();
    expect(screen.getByTestId("y-axis")).toBeInTheDocument();
    expect(screen.getByTestId("cartesian-grid")).toBeInTheDocument();
  });

  it("has accessible aria-label on chart container", () => {
    render(<VendorBarChart data={mockVendorData} vendors={mockVendors} />);

    expect(
      screen.getByLabelText(
        "Grafik batang menampilkan arus kas harian per vendor selama 7 hari terakhir",
      ),
    ).toBeInTheDocument();
  });

  it("renders with empty data without crashing", () => {
    render(<VendorBarChart data={[]} vendors={mockVendors} />);

    expect(screen.getByTestId("responsive-container")).toBeInTheDocument();
  });
});

// ─── CashFlowScreen Error/Loading State Tests ────────────────────────────────

describe("CashFlowScreen", () => {
  beforeEach(() => {
    mockUseCashFlowData.mockReset();
  });

  it("renders loading skeleton when data is loading", () => {
    mockUseCashFlowData.mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
      error: null,
      refetch: vi.fn(),
    });

    const { container } = render(<CashFlowScreen />);

    // Should render pulse animation (skeleton)
    expect(container.querySelector(".animate-pulse")).toBeInTheDocument();
  });

  it("renders error state with message and retry button when fetch fails", () => {
    const mockRefetch = vi.fn();
    mockUseCashFlowData.mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      error: new Error("Network error"),
      refetch: mockRefetch,
    });

    render(<CashFlowScreen />);

    // Error message in Bahasa Indonesia
    expect(
      screen.getByText("Gagal memuat data cash flow. Silakan coba lagi."),
    ).toBeInTheDocument();

    // Retry button
    const retryButton = screen.getByText("Coba Lagi");
    expect(retryButton).toBeInTheDocument();
  });

  it("calls refetch when retry button is clicked", () => {
    const mockRefetch = vi.fn();
    mockUseCashFlowData.mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      error: new Error("Network error"),
      refetch: mockRefetch,
    });

    render(<CashFlowScreen />);

    fireEvent.click(screen.getByText("Coba Lagi"));
    expect(mockRefetch).toHaveBeenCalledTimes(1);
  });

  it("renders stats, chart, and table when data loads successfully", () => {
    mockUseCashFlowData.mockReturnValue({
      data: mockCashFlowData,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    });

    render(<CashFlowScreen />);

    // Page title
    expect(screen.getByText("Cash Flow Monitoring")).toBeInTheDocument();

    // Stats card data
    expect(screen.getByText("Total Kas Beredar")).toBeInTheDocument();
    expect(screen.getByText("Rp 48,2 M")).toBeInTheDocument();

    // Chart renders
    expect(screen.getByTestId("responsive-container")).toBeInTheDocument();
  });
});
