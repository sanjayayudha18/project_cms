import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { ReconciliationException } from "../types";

// ─── Hoisted mock data (static, used by component at import time) ────────────

const { MOCK_DATA, mockToast } = vi.hoisted(() => ({
  MOCK_DATA: [
    {
      id: "REC-001",
      atmId: "ATM-JKT-008",
      lastCountTime: "2026-07-21T08:42:00+07:00",
      location: "Cibubur Junction",
      countedAmount: 1875000000,
      escrowAmount: 2000000000,
      difference: -125000000,
      severity: "high",
      owner: "R. Sanjaya",
    },
    {
      id: "REC-002",
      atmId: "ATM-BDG-003",
      lastCountTime: "2026-07-21T09:15:00+07:00",
      location: "Cihampelas Walk",
      countedAmount: 950000000,
      escrowAmount: 1100000000,
      difference: -150000000,
      severity: "high",
      owner: null,
    },
    {
      id: "REC-003",
      atmId: "ATM-SBY-002",
      lastCountTime: "2026-07-21T07:58:00+07:00",
      location: "Galaxy Mall",
      countedAmount: 2150000000,
      escrowAmount: 2100000000,
      difference: 50000000,
      severity: "medium",
      owner: "A. Pratama",
    },
    {
      id: "REC-004",
      atmId: "ATM-JKT-005",
      lastCountTime: "2026-07-21T10:03:00+07:00",
      location: "Menteng Square",
      countedAmount: 780000000,
      escrowAmount: 850000000,
      difference: -70000000,
      severity: "medium",
      owner: "D. Wijaya",
    },
    {
      id: "REC-005",
      atmId: "ATM-SBY-005",
      lastCountTime: "2026-07-21T08:22:00+07:00",
      location: "Ciputra World",
      countedAmount: 1620000000,
      escrowAmount: 1800000000,
      difference: -180000000,
      severity: "high",
      owner: null,
    },
  ] as ReconciliationException[],
  mockToast: vi.fn(),
}));

// ─── Mock modules ────────────────────────────────────────────────────────────

vi.mock("@/data/reconciliation-exceptions.json", () => ({ default: MOCK_DATA }));
vi.mock("@/lib/hooks/useToast", () => ({
  useToast: () => ({ toast: mockToast }),
}));

// ─── Import component after mocks ───────────────────────────────────────────

import { ReconciliationScreen } from "../ReconciliationScreen";

// ─── Column Rendering Tests ──────────────────────────────────────────────────

describe("ReconciliationScreen - Column rendering", () => {
  it("renders all expected column headers", () => {
    render(<ReconciliationScreen />);

    expect(screen.getByText("Mesin")).toBeInTheDocument();
    expect(screen.getByText("Lokasi")).toBeInTheDocument();
    expect(screen.getByText("Terhitung")).toBeInTheDocument();
    expect(screen.getByText("Escrow")).toBeInTheDocument();
    expect(screen.getByText("Selisih")).toBeInTheDocument();
    // "Tingkat" appears both as filter label and column header
    expect(screen.getAllByText("Tingkat").length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText("Penanggung Jawab")).toBeInTheDocument();
  });

  it("renders ATM IDs in the Mesin column", () => {
    render(<ReconciliationScreen />);

    // Default filter is "Open exceptions" → shows REC-002, REC-005 (owner === null)
    expect(screen.getByText("ATM-BDG-003")).toBeInTheDocument();
    expect(screen.getByText("ATM-SBY-005")).toBeInTheDocument();
  });

  it("renders location data correctly", () => {
    render(<ReconciliationScreen />);

    // Under "Open exceptions" default filter
    expect(screen.getByText("Cihampelas Walk")).toBeInTheDocument();
    expect(screen.getByText("Ciputra World")).toBeInTheDocument();
  });

  it("renders severity badges with correct labels", () => {
    render(<ReconciliationScreen />);

    // Default shows open exceptions (REC-002, REC-005 both high)
    // "Tinggi" appears as badge text (×2) + as filter option label (×1) = 3 total
    const highBadges = screen.getAllByText("Tinggi");
    expect(highBadges.length).toBeGreaterThanOrEqual(2);
  });

  it('renders "Belum ditugaskan" for unassigned exceptions', () => {
    render(<ReconciliationScreen />);

    // Default shows open exceptions (owner === null)
    const unassigned = screen.getAllByText("Belum ditugaskan");
    expect(unassigned).toHaveLength(2);
  });

  it("renders owner names for assigned exceptions", () => {
    render(<ReconciliationScreen />);

    // Change to show all records to see assigned owners
    const selects = screen.getAllByRole("combobox");
    fireEvent.change(selects[0], { target: { value: "All records" } });

    expect(screen.getByText("R. Sanjaya")).toBeInTheDocument();
    expect(screen.getByText("A. Pratama")).toBeInTheDocument();
    expect(screen.getByText("D. Wijaya")).toBeInTheDocument();
  });

  it("renders page header with correct title and description", () => {
    render(<ReconciliationScreen />);

    expect(screen.getByText("Rekonsiliasi")).toBeInTheDocument();
    expect(screen.getByText(/Bandingkan perhitungan kas fisik/)).toBeInTheDocument();
  });

  it("renders the warning notice banner", () => {
    render(<ReconciliationScreen />);

    expect(screen.getByText("Batas waktu 14:00 WIB")).toBeInTheDocument();
    expect(screen.getByText(/exception tingkat tinggi belum terselesaikan/)).toBeInTheDocument();
  });
});

// ─── Filter Behavior Tests ───────────────────────────────────────────────────

describe("ReconciliationScreen - Filter behavior", () => {
  it('defaults to "Open exceptions" filter showing only unassigned records', () => {
    render(<ReconciliationScreen />);

    // REC-002 and REC-005 have owner === null
    expect(screen.getByText("ATM-BDG-003")).toBeInTheDocument();
    expect(screen.getByText("ATM-SBY-005")).toBeInTheDocument();

    // REC-001 has owner assigned, should NOT be visible
    expect(screen.queryByText("Cibubur Junction")).not.toBeInTheDocument();
  });

  it('shows all records when exception type filter is set to "All records"', () => {
    render(<ReconciliationScreen />);

    const selects = screen.getAllByRole("combobox");
    fireEvent.change(selects[0], { target: { value: "All records" } });

    // All 5 records visible
    expect(screen.getByText("ATM-JKT-008")).toBeInTheDocument();
    expect(screen.getByText("ATM-BDG-003")).toBeInTheDocument();
    expect(screen.getByText("ATM-SBY-002")).toBeInTheDocument();
    expect(screen.getByText("ATM-JKT-005")).toBeInTheDocument();
    expect(screen.getByText("ATM-SBY-005")).toBeInTheDocument();
  });

  it('filters by severity "High" showing only high-severity records', () => {
    render(<ReconciliationScreen />);

    const selects = screen.getAllByRole("combobox");
    // Show all records first
    fireEvent.change(selects[0], { target: { value: "All records" } });
    // Then filter by High severity
    fireEvent.change(selects[1], { target: { value: "High" } });

    // High-severity: REC-001, REC-002, REC-005
    expect(screen.getByText("ATM-JKT-008")).toBeInTheDocument();
    expect(screen.getByText("ATM-BDG-003")).toBeInTheDocument();
    expect(screen.getByText("ATM-SBY-005")).toBeInTheDocument();

    // Medium-severity should NOT be visible
    expect(screen.queryByText("ATM-SBY-002")).not.toBeInTheDocument();
    expect(screen.queryByText("ATM-JKT-005")).not.toBeInTheDocument();
  });

  it('filters by severity "Medium" showing only medium-severity records', () => {
    render(<ReconciliationScreen />);

    const selects = screen.getAllByRole("combobox");
    // Show all records first
    fireEvent.change(selects[0], { target: { value: "All records" } });
    // Then filter by Medium severity
    fireEvent.change(selects[1], { target: { value: "Medium" } });

    // Medium-severity: REC-003, REC-004
    expect(screen.getByText("ATM-SBY-002")).toBeInTheDocument();
    expect(screen.getByText("ATM-JKT-005")).toBeInTheDocument();

    // High-severity should NOT be visible
    expect(screen.queryByText("ATM-JKT-008")).not.toBeInTheDocument();
    expect(screen.queryByText("ATM-BDG-003")).not.toBeInTheDocument();
  });

  it('shows "Resolved" records when type filter set to "Resolved"', () => {
    render(<ReconciliationScreen />);

    const selects = screen.getAllByRole("combobox");
    fireEvent.change(selects[0], { target: { value: "Resolved" } });

    // Resolved (owner !== null): REC-001, REC-003, REC-004
    expect(screen.getByText("ATM-JKT-008")).toBeInTheDocument();
    expect(screen.getByText("ATM-SBY-002")).toBeInTheDocument();
    expect(screen.getByText("ATM-JKT-005")).toBeInTheDocument();

    // Unassigned should NOT be visible
    expect(screen.queryByText("ATM-BDG-003")).not.toBeInTheDocument();
    expect(screen.queryByText("ATM-SBY-005")).not.toBeInTheDocument();
  });

  it("combines both filters (Open exceptions + High severity)", () => {
    render(<ReconciliationScreen />);

    const selects = screen.getAllByRole("combobox");
    // Default is "Open exceptions", set severity to High
    fireEvent.change(selects[1], { target: { value: "High" } });

    // Open + High: owner === null AND severity === "high" → REC-002, REC-005
    expect(screen.getByText("ATM-BDG-003")).toBeInTheDocument();
    expect(screen.getByText("ATM-SBY-005")).toBeInTheDocument();

    // REC-001 is high but has owner (not open)
    expect(screen.queryByText("Cibubur Junction")).not.toBeInTheDocument();
  });

  it("displays exception count reflecting filtered results", () => {
    render(<ReconciliationScreen />);

    // Default: Open exceptions → 2 records (REC-002, REC-005)
    expect(screen.getByText("2 exception")).toBeInTheDocument();
  });

  it("updates exception count when filter changes", () => {
    render(<ReconciliationScreen />);

    const selects = screen.getAllByRole("combobox");
    // Show all records
    fireEvent.change(selects[0], { target: { value: "All records" } });

    expect(screen.getByText("5 exception")).toBeInTheDocument();
  });

  it("shows DataTable empty message when combined filter matches no records", () => {
    render(<ReconciliationScreen />);

    const selects = screen.getAllByRole("combobox");
    // "Open exceptions" (default) + "Medium" → owner === null AND severity === "medium"
    // In our data, no record has both owner === null AND severity === "medium"
    fireEvent.change(selects[1], { target: { value: "Medium" } });

    expect(
      screen.getByText("Tidak ada exception yang sesuai dengan filter saat ini."),
    ).toBeInTheDocument();
  });
});

// ─── Empty State Tests (requires module reset for empty data) ────────────────

describe("ReconciliationScreen - Empty state", () => {
  it("displays empty state message when data is empty array", async () => {
    vi.resetModules();
    vi.doMock("@/data/reconciliation-exceptions.json", () => ({ default: [] }));
    vi.doMock("@/lib/hooks/useToast", () => ({
      useToast: () => ({ toast: vi.fn() }),
    }));

    const { ReconciliationScreen: EmptyScreen } = await import("../ReconciliationScreen");
    render(<EmptyScreen />);

    expect(screen.getByText("Tidak ada exception rekonsiliasi yang tersedia.")).toBeInTheDocument();
  });

  it("still renders page header in empty state", async () => {
    vi.resetModules();
    vi.doMock("@/data/reconciliation-exceptions.json", () => ({ default: [] }));
    vi.doMock("@/lib/hooks/useToast", () => ({
      useToast: () => ({ toast: vi.fn() }),
    }));

    const { ReconciliationScreen: EmptyScreen } = await import("../ReconciliationScreen");
    render(<EmptyScreen />);

    expect(screen.getByText("Rekonsiliasi")).toBeInTheDocument();
  });

  it("does not render filter controls in empty state", async () => {
    vi.resetModules();
    vi.doMock("@/data/reconciliation-exceptions.json", () => ({ default: [] }));
    vi.doMock("@/lib/hooks/useToast", () => ({
      useToast: () => ({ toast: vi.fn() }),
    }));

    const { ReconciliationScreen: EmptyScreen } = await import("../ReconciliationScreen");
    render(<EmptyScreen />);

    const selects = screen.queryAllByRole("combobox");
    expect(selects).toHaveLength(0);
  });

  it("does not render warning banner in empty state", async () => {
    vi.resetModules();
    vi.doMock("@/data/reconciliation-exceptions.json", () => ({ default: [] }));
    vi.doMock("@/lib/hooks/useToast", () => ({
      useToast: () => ({ toast: vi.fn() }),
    }));

    const { ReconciliationScreen: EmptyScreen } = await import("../ReconciliationScreen");
    render(<EmptyScreen />);

    expect(screen.queryByText("Batas waktu 14:00 WIB")).not.toBeInTheDocument();
  });
});
