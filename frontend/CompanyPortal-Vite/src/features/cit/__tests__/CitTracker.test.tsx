import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { EnrichedCitOrder } from "../types";

// ─── Mock useCitData hook ────────────────────────────────────────────────────

const mockUseCitData = vi.fn();

vi.mock("../useCitData", () => ({
  useCitData: () => mockUseCitData(),
}));

// ─── Mock @/data/vendors.json ────────────────────────────────────────────────

vi.mock("@/data/vendors.json", () => ({
  default: [
    { id: "V-001", name: "PT Gardanet" },
    { id: "V-002", name: "PT SSI" },
    { id: "V-003", name: "PT G4S" },
  ],
}));

// ─── Import components after mocks ───────────────────────────────────────────

import { CitSummary } from "../CitSummary";
import { CitTable } from "../CitTable";
import { CitTracker } from "../CitTracker";

// ─── Test Data ───────────────────────────────────────────────────────────────

const mockOrders: EnrichedCitOrder[] = [
  {
    id: "CIT-001",
    atmId: "ATM-JKT-001",
    vendorId: "V-001",
    vendorName: "PT Gardanet",
    atmLocation: "Jakarta Selatan",
    orderDate: "2024-01-15",
    scheduledDate: "2024-01-16",
    amount: 250000000,
    status: "Completed",
    evidenceUrl: "https://storage.example.com/evidence/CIT-001.pdf",
  },
  {
    id: "CIT-002",
    atmId: "ATM-JKT-003",
    vendorId: "V-002",
    vendorName: "PT SSI",
    atmLocation: "Jakarta Barat",
    orderDate: "2024-01-15",
    scheduledDate: "2024-01-16",
    amount: 350000000,
    status: "Scheduled",
    evidenceUrl: null,
  },
  {
    id: "CIT-003",
    atmId: "ATM-BDG-003",
    vendorId: "V-003",
    vendorName: "PT G4S",
    atmLocation: "Bandung",
    orderDate: "2024-01-15",
    scheduledDate: "2024-01-16",
    amount: 280000000,
    status: "Failed",
    evidenceUrl: null,
  },
  {
    id: "CIT-004",
    atmId: "ATM-SBY-001",
    vendorId: "V-001",
    vendorName: "PT Gardanet",
    atmLocation: "Surabaya",
    orderDate: "2024-01-16",
    scheduledDate: "2024-01-17",
    amount: 300000000,
    status: "In Transit",
    evidenceUrl: null,
  },
  {
    id: "CIT-005",
    atmId: "ATM-JKT-005",
    vendorId: "V-002",
    vendorName: "PT SSI",
    atmLocation: "Jakarta Pusat",
    orderDate: "2024-01-16",
    scheduledDate: "2024-01-17",
    amount: 320000000,
    status: "Completed",
    evidenceUrl: "https://storage.example.com/evidence/CIT-005.pdf",
  },
];

// ─── CitSummary Tests ────────────────────────────────────────────────────────

describe("CitSummary", () => {
  it("renders correct counts per status category", () => {
    render(<CitSummary data={mockOrders} />);

    // Verify each status label is present
    expect(screen.getByText("Scheduled")).toBeInTheDocument();
    expect(screen.getByText("In Transit")).toBeInTheDocument();
    expect(screen.getByText("Completed")).toBeInTheDocument();
    expect(screen.getByText("Failed")).toBeInTheDocument();

    // Check specific counts - find count values near their labels
    const scheduledCard = screen.getByText("Scheduled").closest("div")!.parentElement!;
    expect(scheduledCard).toHaveTextContent("1");

    const inTransitCard = screen.getByText("In Transit").closest("div")!.parentElement!;
    expect(inTransitCard).toHaveTextContent("1");

    const completedCard = screen.getByText("Completed").closest("div")!.parentElement!;
    expect(completedCard).toHaveTextContent("2");

    const failedCard = screen.getByText("Failed").closest("div")!.parentElement!;
    expect(failedCard).toHaveTextContent("1");
  });

  it("renders zero counts when data is empty", () => {
    render(<CitSummary data={[]} />);

    // All statuses should show 0
    const zeroValues = screen.getAllByText("0");
    expect(zeroValues).toHaveLength(4);
  });

  it("renders all four status cards in a grid", () => {
    const { container } = render(<CitSummary data={mockOrders} />);

    const grid = container.querySelector(".grid");
    expect(grid).toBeInTheDocument();
    // 4 status cards
    expect(grid?.children.length).toBe(4);
  });
});

// ─── CitTable Tests ──────────────────────────────────────────────────────────

describe("CitTable", () => {
  it("renders all column headers correctly", () => {
    render(<CitTable data={mockOrders} />);

    expect(screen.getByText("Order ID")).toBeInTheDocument();
    expect(screen.getByText("ATM ID")).toBeInTheDocument();
    expect(screen.getByText("Vendor")).toBeInTheDocument();
    expect(screen.getByText("Tanggal Order")).toBeInTheDocument();
    expect(screen.getByText("Tanggal Jadwal")).toBeInTheDocument();
    expect(screen.getByText("Jumlah (IDR)")).toBeInTheDocument();
    expect(screen.getByText("Status")).toBeInTheDocument();
    expect(screen.getByText("Bukti")).toBeInTheDocument();
  });

  it("renders order data in table rows", () => {
    render(<CitTable data={mockOrders} />);

    // Check some order IDs are rendered
    expect(screen.getByText("CIT-001")).toBeInTheDocument();
    expect(screen.getByText("CIT-002")).toBeInTheDocument();
    expect(screen.getByText("CIT-003")).toBeInTheDocument();

    // Check ATM IDs
    expect(screen.getByText("ATM-JKT-001")).toBeInTheDocument();
    expect(screen.getByText("ATM-BDG-003")).toBeInTheDocument();

    // Check vendor names
    expect(screen.getAllByText("PT Gardanet")).toHaveLength(2);
    expect(screen.getAllByText("PT SSI")).toHaveLength(2);
    expect(screen.getByText("PT G4S")).toBeInTheDocument();
  });

  it("renders amount as IDR-formatted value", () => {
    render(<CitTable data={mockOrders} />);

    // formatIDR(250000000) = "250.000.000"
    expect(screen.getByText("250.000.000")).toBeInTheDocument();
    expect(screen.getByText("350.000.000")).toBeInTheDocument();
  });

  it("renders status badges with correct labels", () => {
    render(<CitTable data={mockOrders} />);

    // Status badge labels (in addition to header "Status")
    expect(screen.getAllByText("Completed").length).toBeGreaterThanOrEqual(2);
    expect(screen.getAllByText("Scheduled").length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText("Failed").length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText("In Transit").length).toBeGreaterThanOrEqual(1);
  });

  it("renders evidence link for orders with evidenceUrl", () => {
    render(<CitTable data={mockOrders} />);

    // Orders with evidenceUrl show "Lihat" link
    const evidenceLinks = screen.getAllByText("Lihat");
    expect(evidenceLinks.length).toBeGreaterThanOrEqual(2);

    // Link opens in new tab
    const link = evidenceLinks[0].closest("a");
    expect(link).toHaveAttribute("target", "_blank");
    expect(link).toHaveAttribute("rel", "noopener noreferrer");
  });

  it("renders dash for orders without evidenceUrl", () => {
    render(<CitTable data={mockOrders} />);

    // Orders without evidenceUrl show mdash
    const dashes = screen.getAllByText("—");
    expect(dashes.length).toBeGreaterThanOrEqual(1);
  });

  it("renders empty message when data is empty", () => {
    render(<CitTable data={[]} />);

    expect(screen.getByText("Tidak ada order CIT yang sesuai filter")).toBeInTheDocument();
  });
});

// ─── CitTracker Integration Tests ────────────────────────────────────────────

describe("CitTracker", () => {
  beforeEach(() => {
    mockUseCitData.mockReset();
  });

  it("renders summary and table when data is available", () => {
    mockUseCitData.mockReturnValue({ data: mockOrders });

    render(<CitTracker />);

    // Page title
    expect(screen.getByText("CIT Tracker")).toBeInTheDocument();

    // Summary grid present (4 status cards)
    const summaryGrid = screen.getByText("CIT Tracker").closest("div")!.querySelector(".grid");
    expect(summaryGrid?.children.length).toBe(4);

    // Table columns present
    expect(screen.getByText("Order ID")).toBeInTheDocument();

    // Data rendered
    expect(screen.getByText("CIT-001")).toBeInTheDocument();
  });

  it("renders filter controls for status and vendor", () => {
    mockUseCitData.mockReturnValue({ data: mockOrders });

    render(<CitTracker />);

    // Selects present (status and vendor)
    const selects = screen.getAllByRole("combobox");
    expect(selects).toHaveLength(2);

    // Check "Semua" default option present in both selects
    const allOptions = screen.getAllByText("Semua");
    expect(allOptions).toHaveLength(2);
  });

  it("filters by status and updates summary and table", () => {
    mockUseCitData.mockReturnValue({ data: mockOrders });

    render(<CitTracker />);

    // Select "Failed" status filter
    const selects = screen.getAllByRole("combobox");
    const statusSelect = selects[0];
    fireEvent.change(statusSelect, { target: { value: "Failed" } });

    // After filtering by Failed, only CIT-003 should be in the table
    expect(screen.getByText("CIT-003")).toBeInTheDocument();
    expect(screen.queryByText("CIT-001")).not.toBeInTheDocument();
    expect(screen.queryByText("CIT-002")).not.toBeInTheDocument();

    // Summary should reflect filtered data: check the summary grid counts
    // The summary grid has 4 cards with the count in a tabular-nums paragraph
    const countElements = screen.getAllByText("0");
    // With 1 Failed order, Scheduled=0, In Transit=0, Completed=0 → three zeroes
    expect(countElements.length).toBe(3);
    // And one "1" count for Failed
    expect(screen.getByText("1")).toBeInTheDocument();
  });

  it("filters by vendor and updates summary and table", () => {
    mockUseCitData.mockReturnValue({ data: mockOrders });

    render(<CitTracker />);

    // Select vendor V-001 (PT Gardanet)
    const selects = screen.getAllByRole("combobox");
    const vendorSelect = selects[1];
    fireEvent.change(vendorSelect, { target: { value: "V-001" } });

    // PT Gardanet orders: CIT-001 (Completed), CIT-004 (In Transit)
    expect(screen.getByText("CIT-001")).toBeInTheDocument();
    expect(screen.getByText("CIT-004")).toBeInTheDocument();
    expect(screen.queryByText("CIT-002")).not.toBeInTheDocument();
    expect(screen.queryByText("CIT-003")).not.toBeInTheDocument();
    expect(screen.queryByText("CIT-005")).not.toBeInTheDocument();
  });

  it("displays empty state when filtered results are empty", () => {
    const completedOnly: EnrichedCitOrder[] = [
      {
        id: "CIT-010",
        atmId: "ATM-JKT-010",
        vendorId: "V-001",
        vendorName: "PT Gardanet",
        atmLocation: "Jakarta",
        orderDate: "2024-01-20",
        scheduledDate: "2024-01-21",
        amount: 100000000,
        status: "Completed",
        evidenceUrl: null,
      },
    ];

    mockUseCitData.mockReturnValue({ data: completedOnly });

    render(<CitTracker />);

    // Filter by "Failed" status — no matching orders
    const selects = screen.getAllByRole("combobox");
    fireEvent.change(selects[0], { target: { value: "Failed" } });

    // Empty state message should be displayed
    expect(screen.getByText("Tidak ada order CIT yang sesuai filter")).toBeInTheDocument();
  });

  it("displays empty state when no data is returned", () => {
    mockUseCitData.mockReturnValue({ data: [] });

    render(<CitTracker />);

    // Empty state message
    expect(screen.getByText("Tidak ada order CIT yang sesuai filter")).toBeInTheDocument();
  });

  it("resets filter back to show all when selecting empty value", () => {
    mockUseCitData.mockReturnValue({ data: mockOrders });

    render(<CitTracker />);

    const selects = screen.getAllByRole("combobox");
    const statusSelect = selects[0];

    // Filter by "Failed"
    fireEvent.change(statusSelect, { target: { value: "Failed" } });
    expect(screen.queryByText("CIT-001")).not.toBeInTheDocument();

    // Reset filter
    fireEvent.change(statusSelect, { target: { value: "" } });
    expect(screen.getByText("CIT-001")).toBeInTheDocument();
    expect(screen.getByText("CIT-002")).toBeInTheDocument();
  });
});
