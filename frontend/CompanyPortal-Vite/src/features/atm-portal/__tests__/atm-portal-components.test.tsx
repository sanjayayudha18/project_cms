/**
 * Task 13.1 unit tests (design.md Testing Strategy table), covering every
 * row that isn't already exercised by Sidebar.test.tsx or
 * navigation.test.ts. Each `it` block is annotated with the exact table row
 * and requirement it validates.
 */

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AriaLiveRegion } from "../components/AriaLiveRegion";
import { AtmTable } from "../components/AtmTable";
import { FilterBar } from "../components/FilterBar";
import { PaginationControls } from "../components/PaginationControls";
import { StatusBadge } from "../components/StatusBadge";
import { SummaryCardsGrid } from "../components/SummaryCardsGrid";
import { STATUS_BADGE_CONFIG } from "../constants";
import type { AtmPortalParams, AtmPortalResponse, AtmRecord, AtmSummary } from "../types";

// ─── Fixtures ─────────────────────────────────────────────────────────────────

const atmFixture: AtmRecord = {
  terminal_id: "T001",
  location_name: "Jakarta Pusat",
  address: "Jl. Sudirman",
  machine_type: "ATM",
  brand: "Hyosung",
  deployment_type: "ONSITE",
  low_threshold: 1000000,
  critical_threshold: 500000,
  last_replenish_date: "2026-08-19",
  last_replenish_time: "08:00:00",
  refund_total: 1500000,
  replenish_total: 5000000,
  escrow: 3500000,
  status: "critical",
};

const summaryFixture: AtmSummary = {
  total: 150,
  critical: 12,
  low: 30,
  normal: 100,
  unconfigured: 5,
  no_data: 3,
};

// ─── Req 3.3: Summary cards appear in order Total, Critical, Low, Normal ──────

describe("SummaryCardsGrid — Req 3.3, 8.1, 11.3", () => {
  it("renders cards in order: Total, Critical, Low, Normal", () => {
    render(<SummaryCardsGrid summary={summaryFixture} isLoading={false} />);
    const cards = screen.getAllByRole("group");
    const labels = cards.map((c) => c.getAttribute("aria-label"));
    expect(labels).toEqual(["Total ATM: 150", "Critical: 12", "Low: 30", "Normal: 100"]);
  });

  it("has aria-label with metric name + value on each card (Req 11.3)", () => {
    render(<SummaryCardsGrid summary={summaryFixture} isLoading={false} />);
    expect(screen.getByLabelText("Total ATM: 150")).toBeInTheDocument();
    expect(screen.getByLabelText("Critical: 12")).toBeInTheDocument();
  });

  it("loading state shows 4 skeleton cards (Req 8.1)", () => {
    const { container } = render(<SummaryCardsGrid isLoading={true} />);
    // No aria-label group cards are rendered while loading — only skeletons.
    expect(screen.queryAllByRole("group")).toHaveLength(0);
    const grid = container.firstElementChild;
    expect(grid?.children).toHaveLength(4);
  });
});

// ─── Req 4.5: Status badges render correct icon + label per variant ──────────

describe("StatusBadge — Req 4.5", () => {
  it.each(Object.keys(STATUS_BADGE_CONFIG) as (keyof typeof STATUS_BADGE_CONFIG)[])(
    "renders the correct label for status=%s",
    (status) => {
      render(<StatusBadge status={status} />);
      expect(screen.getByText(STATUS_BADGE_CONFIG[status].label)).toBeInTheDocument();
    },
  );
});

// ─── Req 11.1: Table uses semantic HTML ───────────────────────────────────────
// ─── Req 8.1, 8.2, 8.4, 11.7: loading/error/empty states, aria-sort ───────────

describe("AtmTable — Req 8.1, 8.2, 8.4, 11.1, 11.7", () => {
  const noop = () => {};

  it("uses semantic table/thead/tbody/th structure (Req 11.1)", () => {
    render(
      <AtmTable
        data={[atmFixture]}
        isLoading={false}
        isError={false}
        onRetry={noop}
        sortBy="terminal_id"
        sortOrder="asc"
        onSortChange={noop}
      />,
    );
    const table = screen.getByRole("table");
    expect(within(table).getByRole("columnheader", { name: /Terminal ID/ })).toBeInTheDocument();
    expect(table.querySelector("thead")).not.toBeNull();
    expect(table.querySelector("tbody")).not.toBeNull();
  });

  it("loading state shows 5 skeleton rows (Req 8.1)", () => {
    const { container } = render(
      <AtmTable
        data={[]}
        isLoading={true}
        isError={false}
        onRetry={noop}
        sortBy="terminal_id"
        sortOrder="asc"
        onSortChange={noop}
      />,
    );
    expect(container.querySelectorAll("tbody tr")).toHaveLength(5);
  });

  it("error state shows error icon, message, and retry button (Req 8.2)", () => {
    render(
      <AtmTable
        data={[]}
        isLoading={false}
        isError={true}
        onRetry={noop}
        sortBy="terminal_id"
        sortOrder="asc"
        onSortChange={noop}
      />,
    );
    expect(screen.getByText("Gagal memuat data ATM")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Coba Lagi" })).toBeInTheDocument();
  });

  it("retry button click invokes onRetry (Req 8.3)", async () => {
    const onRetry = vi.fn();
    render(
      <AtmTable
        data={[]}
        isLoading={false}
        isError={true}
        onRetry={onRetry}
        sortBy="terminal_id"
        sortOrder="asc"
        onSortChange={noop}
      />,
    );
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Coba Lagi" }));
    expect(onRetry).toHaveBeenCalledTimes(1);
  });

  it("empty state shows 'Tidak ada ATM yang sesuai filter' (Req 8.4)", () => {
    render(
      <AtmTable
        data={[]}
        isLoading={false}
        isError={false}
        onRetry={noop}
        sortBy="terminal_id"
        sortOrder="asc"
        onSortChange={noop}
      />,
    );
    expect(screen.getByText("Tidak ada ATM yang sesuai filter")).toBeInTheDocument();
  });

  it("sort interaction updates aria-sort on the clicked header (Req 11.7)", async () => {
    const onSortChange = vi.fn();
    const { rerender } = render(
      <AtmTable
        data={[atmFixture]}
        isLoading={false}
        isError={false}
        onRetry={noop}
        sortBy="terminal_id"
        sortOrder="asc"
        onSortChange={onSortChange}
      />,
    );
    const header = screen.getByRole("columnheader", { name: /Terminal ID/ });
    expect(header).toHaveAttribute("aria-sort", "ascending");

    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: /Terminal ID/ }));
    expect(onSortChange).toHaveBeenCalledWith("terminal_id", "desc");

    // Simulate the parent applying the new sort state.
    rerender(
      <AtmTable
        data={[atmFixture]}
        isLoading={false}
        isError={false}
        onRetry={noop}
        sortBy="terminal_id"
        sortOrder="desc"
        onSortChange={onSortChange}
      />,
    );
    expect(screen.getByRole("columnheader", { name: /Terminal ID/ })).toHaveAttribute(
      "aria-sort",
      "descending",
    );
  });

  it("renders Total Replenish column with formatted replenish_total (fix1)", () => {
    render(
      <AtmTable
        data={[atmFixture]}
        isLoading={false}
        isError={false}
        onRetry={noop}
        sortBy="terminal_id"
        sortOrder="asc"
        onSortChange={noop}
      />,
    );
    expect(screen.getByRole("columnheader", { name: /Total Replenish/ })).toBeInTheDocument();
    // formatRupiah(5000000) → "Rp 5.000.000"
    expect(screen.getByText("Rp 5.000.000")).toBeInTheDocument();
  });

  it("shows em dash when replenish_total is null (fix1)", () => {
    render(
      <AtmTable
        data={[{ ...atmFixture, replenish_total: null, refund_total: null, low_threshold: null }]}
        isLoading={false}
        isError={false}
        onRetry={noop}
        sortBy="terminal_id"
        sortOrder="asc"
        onSortChange={noop}
      />,
    );
    const dashes = screen.getAllByText("—");
    expect(dashes.length).toBeGreaterThanOrEqual(1);
  });

  it("Total Replenish header sort invokes onSortChange with replenish_total (fix1)", async () => {
    const onSortChange = vi.fn();
    render(
      <AtmTable
        data={[atmFixture]}
        isLoading={false}
        isError={false}
        onRetry={noop}
        sortBy="terminal_id"
        sortOrder="asc"
        onSortChange={onSortChange}
      />,
    );
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: /Total Replenish/ }));
    expect(onSortChange).toHaveBeenCalledWith("replenish_total", "asc");
  });
});

// ─── Req 4.8: Pagination page size options ────────────────────────────────────

describe("PaginationControls — Req 4.8", () => {
  it("offers page size options [10, 25, 50, 100]", () => {
    render(
      <PaginationControls
        page={1}
        pageSize={25}
        total={100}
        onPageChange={() => {}}
        onPageSizeChange={() => {}}
      />,
    );
    const select = screen.getByLabelText("Baris per halaman");
    const options = within(select as HTMLSelectElement).getAllByRole("option");
    expect(options.map((o) => o.textContent)).toEqual(["10", "25", "50", "100"]);
  });
});

// ─── Req 11.5: Search input aria-label ─────────────────────────────────────────

describe("FilterBar — Req 11.5 + date range (fix1)", () => {
  const defaultFilterBarProps = {
    searchInput: "",
    onSearchInputChange: () => {},
    status: "all",
    machineType: "",
    brand: "",
    deploymentType: "",
    dateFrom: "",
    dateTo: "",
    onFilterChange: () => {},
    onClearAll: () => {},
  };

  it("search input has a correct aria-label", () => {
    render(<FilterBar {...defaultFilterBarProps} />);
    expect(screen.getByLabelText("Cari berdasarkan Terminal ID atau lokasi")).toBeInTheDocument();
  });

  it("renders Dari tanggal and Sampai tanggal inputs", () => {
    render(<FilterBar {...defaultFilterBarProps} />);
    expect(screen.getByLabelText("Dari tanggal")).toBeInTheDocument();
    expect(screen.getByLabelText("Sampai tanggal")).toBeInTheDocument();
  });

  it("date change fires onFilterChange with date_from/date_to and page 1", async () => {
    const onFilterChange = vi.fn();
    render(<FilterBar {...defaultFilterBarProps} onFilterChange={onFilterChange} />);
    const user = userEvent.setup();
    await user.type(screen.getByLabelText("Dari tanggal"), "2026-08-01");
    expect(onFilterChange).toHaveBeenCalled();
    const lastFrom = onFilterChange.mock.calls.at(-1)?.[0];
    expect(lastFrom).toMatchObject({ page: 1 });
    expect(lastFrom).toHaveProperty("date_from");

    onFilterChange.mockClear();
    await user.type(screen.getByLabelText("Sampai tanggal"), "2026-08-21");
    const lastTo = onFilterChange.mock.calls.at(-1)?.[0];
    expect(lastTo).toMatchObject({ page: 1 });
    expect(lastTo).toHaveProperty("date_to");
  });

  it("active filter count includes date bounds; Clear All is shown", () => {
    const onClearAll = vi.fn();
    render(
      <FilterBar
        {...defaultFilterBarProps}
        dateFrom="2026-08-01"
        dateTo="2026-08-21"
        onClearAll={onClearAll}
      />,
    );
    expect(screen.getByText("2 filter aktif")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Clear All" })).toBeInTheDocument();
  });
});

// ─── Req 11.8: aria-live announces state transitions ──────────────────────────

describe("AriaLiveRegion — Req 11.8", () => {
  it("announces distinct text per state (loading/error/empty/loaded)", () => {
    const { rerender } = render(
      <AriaLiveRegion isLoading={true} isError={false} resultCount={0} />,
    );
    expect(screen.getByText("Memuat data ATM…")).toBeInTheDocument();

    rerender(<AriaLiveRegion isLoading={false} isError={true} resultCount={0} />);
    expect(screen.getByText("Gagal memuat data ATM")).toBeInTheDocument();

    rerender(<AriaLiveRegion isLoading={false} isError={false} resultCount={0} />);
    expect(screen.getByText("Tidak ada ATM yang sesuai filter")).toBeInTheDocument();

    rerender(<AriaLiveRegion isLoading={false} isError={false} resultCount={5} />);
    expect(screen.getByText("Menampilkan 5 ATM")).toBeInTheDocument();
  });
});

// ─── Req 3.2, 8.3: PageHeader content + retry triggers refetch (composed) ─────

const mockParams: AtmPortalParams = {
  page: 1,
  page_size: 25,
  search: "",
  status: "all",
  machine_type: "",
  brand: "",
  deployment_type: "",
  region: "",
  date_from: "",
  date_to: "",
  sort_by: "terminal_id",
  sort_order: "asc",
};

vi.mock("../useAtmPortalUrlState", () => ({
  useAtmPortalUrlState: () => ({
    params: mockParams,
    searchInput: "",
    setSearchInput: vi.fn(),
    setParams: vi.fn(),
  }),
}));

const mockUseAtmPortalData = vi.fn();
vi.mock("../useAtmPortalData", () => ({
  useAtmPortalData: () => mockUseAtmPortalData(),
}));

const mockResponse: AtmPortalResponse = {
  data: [atmFixture],
  summary: summaryFixture,
  total: 1,
  page: 1,
  page_size: 25,
  last_updated: "2026-08-20T10:00:00Z",
};

async function importAtmPortalScreen() {
  const mod = await import("../AtmPortalScreen");
  return mod.AtmPortalScreen;
}

function renderScreenWithClient() {
  const queryClient = new QueryClient();
  const refetchSpy = vi.spyOn(queryClient, "refetchQueries");
  return { queryClient, refetchSpy };
}

describe("AtmPortalScreen composition — Req 3.2, 8.3", () => {
  beforeEach(() => {
    mockUseAtmPortalData.mockReset();
  });

  it("renders PageHeader with the correct title and description (Req 3.2)", async () => {
    mockUseAtmPortalData.mockReturnValue({ data: mockResponse, isLoading: false, isError: false });
    const AtmPortalScreen = await importAtmPortalScreen();
    const { queryClient } = renderScreenWithClient();

    render(
      <QueryClientProvider client={queryClient}>
        <AtmPortalScreen />
      </QueryClientProvider>,
    );

    expect(screen.getByRole("heading", { name: "ATM Portal" })).toBeInTheDocument();
    expect(
      screen.getByText("Monitor posisi kas dan status replenishment seluruh ATM"),
    ).toBeInTheDocument();
  });

  it("retry click triggers a refetch (Req 8.3)", async () => {
    mockUseAtmPortalData.mockReturnValue({ data: undefined, isLoading: false, isError: true });
    const AtmPortalScreen = await importAtmPortalScreen();
    const { queryClient, refetchSpy } = renderScreenWithClient();

    render(
      <QueryClientProvider client={queryClient}>
        <AtmPortalScreen />
      </QueryClientProvider>,
    );

    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Coba Lagi" }));

    expect(refetchSpy).toHaveBeenCalledWith({
      queryKey: ["atm-portal", "list", mockParams],
    });
  });
});
