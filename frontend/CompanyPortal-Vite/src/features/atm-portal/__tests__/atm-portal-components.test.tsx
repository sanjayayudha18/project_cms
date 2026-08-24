/**
 * Task 13.1 unit tests (design.md Testing Strategy table), covering every
 * row that isn't already exercised by Sidebar.test.tsx or
 * navigation.test.ts. Each `it` block is annotated with the exact table row
 * and requirement it validates.
 */

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AriaLiveRegion } from "../components/AriaLiveRegion";
import { AtmCashposTable } from "../components/AtmCashposTable";
import { AtmTable } from "../components/AtmTable";
import { FilterBar } from "../components/FilterBar";
import { PaginationControls } from "../components/PaginationControls";
import { StatusBadge } from "../components/StatusBadge";
import { SummaryCardsGrid } from "../components/SummaryCardsGrid";
import { TableModeSelect } from "../components/TableModeSelect";
import { STATUS_BADGE_CONFIG } from "../constants";
import type {
  AtmCashposRecord,
  AtmPortalParams,
  AtmPortalResponse,
  AtmRecord,
  AtmSummary,
} from "../types";

// AtmTable's terminal_id cell renders a router <Link>, which throws outside a
// RouterProvider. Tests here render AtmTable/AtmPortalScreen standalone (no
// router context), so Link is stubbed as a plain anchor — same approach
// AppShell.test.tsx uses for router hooks.
vi.mock("@tanstack/react-router", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    Link: ({ children, className }: { children?: ReactNode; className?: string }) => (
      <a href="/atm-portal/stub" className={className}>
        {children}
      </a>
    ),
  };
});

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
  mode: "replenish",
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

const mockSetMode = vi.fn();
const mockUrlState = {
  params: mockParams as AtmPortalParams,
  searchInput: "",
  setSearchInput: vi.fn(),
  setParams: vi.fn(),
  setMode: mockSetMode,
};

vi.mock("../useAtmPortalUrlState", () => ({
  useAtmPortalUrlState: () => mockUrlState,
}));

const mockUseAtmPortalData = vi.fn();
const mockUseAtmCashposData = vi.fn();
vi.mock("../useAtmPortalData", () => ({
  useAtmPortalData: () => mockUseAtmPortalData(),
  useAtmCashposData: () => mockUseAtmCashposData(),
}));

const cashposFixture: AtmCashposRecord = {
  id: 1,
  file_id: 2,
  cashpos_date: "2026-08-20",
  terminal_id: "ATM001",
  machine_type: "ATM100K",
  teller_id: "T001",
  branch_code: "BR01",
  starting_cash_10k: "1000000.00",
  cash_in_10k: "50.50",
  cash_out_10k: "25.25",
  cash_position_10k: "1025.25",
  starting_cash_20k: "0.00",
  cash_in_20k: "0.00",
  cash_out_20k: "0.00",
  cash_position_20k: "0.00",
  starting_cash_50k: "0.00",
  cash_in_50k: "0.00",
  cash_out_50k: "0.00",
  cash_position_50k: "0.00",
  starting_cash_100k: "9999999999999999.99",
  cash_in_100k: "1.01",
  cash_out_100k: "2.02",
  cash_position_100k: "3.03",
  position_source: "CURRENT",
  created_at: "2026-08-20T10:30:00Z",
};

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
    mockUseAtmCashposData.mockReset();
    mockSetMode.mockReset();
    mockUrlState.params = { ...mockParams };
    mockUseAtmCashposData.mockReturnValue({ data: undefined, isLoading: false, isError: false });
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
    expect(screen.getByLabelText("Tampilan tabel")).toBeInTheDocument();
    expect(screen.getByRole("table", { name: "Daftar ATM" })).toBeInTheDocument();
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
      queryKey: ["atm-portal", "list", mockUrlState.params],
    });
  });

  it("cashpos mode renders cashpos table with same filter chrome as replenish", async () => {
    mockUrlState.params = {
      ...mockParams,
      mode: "cashpos",
      sort_by: "cashpos_date",
      sort_order: "desc",
    };
    mockUseAtmPortalData.mockReturnValue({ data: mockResponse, isLoading: false, isError: false });
    mockUseAtmCashposData.mockReturnValue({
      data: { data: [cashposFixture], total: 1, page: 1, page_size: 25 },
      isLoading: false,
      isError: false,
    });
    const AtmPortalScreen = await importAtmPortalScreen();
    const { queryClient } = renderScreenWithClient();

    render(
      <QueryClientProvider client={queryClient}>
        <AtmPortalScreen />
      </QueryClientProvider>,
    );

    expect(screen.getByRole("table", { name: "Daftar ATM Cashpos" })).toBeInTheDocument();
    expect(screen.queryByRole("table", { name: "Daftar ATM" })).not.toBeInTheDocument();
    expect(screen.getByLabelText(/Total ATM/)).toBeInTheDocument();
    expect(screen.getByText("Status")).toBeInTheDocument();
    expect(screen.getByText("Brand")).toBeInTheDocument();
    expect(screen.getByLabelText("Dari tanggal")).toBeInTheDocument();
    expect(screen.getByLabelText("Tampilan tabel")).toBeInTheDocument();
    expect(screen.getByText("ATM001")).toBeInTheDocument();
    expect(screen.getByText("Rp 9.999.999.999.999.999,99")).toBeInTheDocument();
  });
});

describe("TableModeSelect", () => {
  it("exposes exactly ATM Replenish and ATM Cashpos options", () => {
    render(<TableModeSelect mode="replenish" onModeChange={() => {}} />);
    const select = screen.getByLabelText("Tampilan tabel");
    const options = within(select as HTMLSelectElement).getAllByRole("option");
    expect(options.map((o) => o.textContent)).toEqual(["ATM Replenish", "ATM Cashpos"]);
  });

  it("calls onModeChange when selection changes", async () => {
    const onModeChange = vi.fn();
    render(<TableModeSelect mode="replenish" onModeChange={onModeChange} />);
    const user = userEvent.setup();
    await user.selectOptions(screen.getByLabelText("Tampilan tabel"), "cashpos");
    expect(onModeChange).toHaveBeenCalledWith("cashpos");
  });
});

describe("AtmCashposTable", () => {
  const noop = () => {};

  it("renders all schema field headers", () => {
    render(
      <AtmCashposTable
        data={[cashposFixture]}
        isLoading={false}
        isError={false}
        onRetry={noop}
        sortBy="cashpos_date"
        sortOrder="desc"
        onSortChange={noop}
      />,
    );
    const headers = screen.getAllByRole("columnheader").map((h) => h.textContent?.trim());
    expect(headers).toEqual([
      "ID",
      "File ID",
      "Created At",
      "Cashpos Date",
      "Terminal ID",
      "Machine Type",
      "Teller ID",
      "Branch Code",
      "Position Source",
      "Starting 10K",
      "In 10K",
      "Out 10K",
      "Pos 10K",
      "Starting 20K",
      "In 20K",
      "Out 20K",
      "Pos 20K",
      "Starting 50K",
      "In 50K",
      "Out 50K",
      "Pos 50K",
      "Starting 100K",
      "In 100K",
      "Out 100K",
      "Pos 100K",
    ]);
  });

  it("loading/error/empty/retry states", async () => {
    const { rerender } = render(
      <AtmCashposTable
        data={[]}
        isLoading={true}
        isError={false}
        onRetry={noop}
        sortBy="id"
        sortOrder="asc"
        onSortChange={noop}
      />,
    );
    expect(document.querySelectorAll("tbody tr")).toHaveLength(5);

    const onRetry = vi.fn();
    rerender(
      <AtmCashposTable
        data={[]}
        isLoading={false}
        isError={true}
        onRetry={onRetry}
        sortBy="id"
        sortOrder="asc"
        onSortChange={noop}
      />,
    );
    expect(screen.getByText("Gagal memuat data ATM Cashpos")).toBeInTheDocument();
    await userEvent.setup().click(screen.getByRole("button", { name: "Coba Lagi" }));
    expect(onRetry).toHaveBeenCalled();

    rerender(
      <AtmCashposTable
        data={[]}
        isLoading={false}
        isError={false}
        onRetry={noop}
        sortBy="id"
        sortOrder="asc"
        onSortChange={noop}
      />,
    );
    expect(screen.getByText("Tidak ada data cashpos yang sesuai filter")).toBeInTheDocument();
  });
});

describe("FilterBar shared chrome", () => {
  it("always shows status, brand, machine type, deployment, and date filters", () => {
    render(
      <FilterBar
        searchInput=""
        onSearchInputChange={() => {}}
        status="all"
        machineType=""
        brand=""
        deploymentType=""
        dateFrom=""
        dateTo=""
        onFilterChange={() => {}}
        onClearAll={() => {}}
      />,
    );
    expect(screen.getByText("Status")).toBeInTheDocument();
    expect(screen.getByText("Brand")).toBeInTheDocument();
    expect(screen.getByText("Machine Type")).toBeInTheDocument();
    expect(screen.getByLabelText("Dari tanggal")).toBeInTheDocument();
  });
});
