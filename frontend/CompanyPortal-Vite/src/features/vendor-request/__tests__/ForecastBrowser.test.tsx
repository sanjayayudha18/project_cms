/**
 * Component tests for ForecastBrowser's pagination footer (Task 5) and
 * cross-page select-all (Task 6), .kiro/specs/update-cit-forecast-browser.
 * Network is mocked at the `api.get` HTTP boundary (not at api.ts's own
 * `fetchForecast`/`fetchAllForecastForSelection`) so the real
 * fetchAllForecastForSelection pagination-and-cap logic runs against the
 * mock — mocking `fetchForecast` itself wouldn't affect
 * fetchAllForecastForSelection's internal call to it, since both live in
 * the same module and the internal reference isn't rebound by vi.mock.
 */
import { api } from "@/lib/api/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ForecastBrowser } from "../ForecastBrowser";
import type {
  ForecastResponse,
  ForecastRow,
  ForecastSummaryResponse,
  VendorOptionsResponse,
} from "../types";

vi.mock("@tanstack/react-router", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    useNavigate: () => vi.fn(),
    // CIT-2 (Task 11): the real Link requires a RouterProvider, which this
    // test harness doesn't mount (see renderPage) — stub it as a plain
    // anchor since the tests don't exercise navigation from the prompt panel.
    Link: ({ children, to, ...rest }: { children?: import("react").ReactNode; to?: string }) => (
      <a href={to} {...rest}>
        {children}
      </a>
    ),
  };
});

vi.mock("@/lib/api/client", () => ({ api: { get: vi.fn() } }));

const mockToast = vi.fn();
vi.mock("@/lib/hooks/useToast", () => ({
  useToast: () => ({ toast: mockToast }),
}));

const mockApiGet = vi.mocked(api.get);

function parsePageParams(path: string): { page: number; pageSize: number } {
  const query = new URLSearchParams(path.split("?")[1] ?? "");
  return {
    page: Number(query.get("page") ?? "1"),
    pageSize: Number(query.get("page_size") ?? "20"),
  };
}

function makeResponse(page: number, pageSize: number, totalCount: number): ForecastResponse {
  const totalPages = Math.max(1, Math.ceil(totalCount / pageSize));
  return {
    data: [],
    pagination: { page, page_size: pageSize, total_count: totalCount, total_pages: totalPages },
  };
}

// CIT-2 (Task 11): the required FLM Vendor / FLM Vendor Region selects fetch
// their options from GET /vendors, distinct from the forecast endpoint both
// mock helpers below already handle.
const VENDOR_OPTIONS_RESPONSE: VendorOptionsResponse = {
  vendors: [
    { id: 1, name: "TAG" },
    { id: 2, name: "ROH" },
  ],
  regions: ["Jawa Barat"],
};

// forecast-browser-summary: the recap (GET /forecast/summary) loads alongside
// the detail table on every render of the page.
const SUMMARY_RESPONSE: ForecastSummaryResponse = {
  forecast_date: "2027-01-15",
  totals: {
    atm_count: 3,
    requested_atm_count: 1,
    unrequested_atm_count: 2,
    unassigned_atm_count: 1,
    amount_replenish: 3000,
    unrequested_amount_replenish: 2000,
  },
  groups: [
    {
      flm_vendor: "TAG",
      flm_vendor_region: "Jawa Barat",
      atm_count: 2,
      requested_atm_count: 1,
      unrequested_atm_count: 1,
      amount_replenish: 2000,
      unrequested_amount_replenish: 1000,
    },
    {
      flm_vendor: "",
      flm_vendor_region: "",
      atm_count: 1,
      requested_atm_count: 0,
      unrequested_atm_count: 1,
      amount_replenish: 1000,
      unrequested_amount_replenish: 1000,
    },
    {
      flm_vendor: "ROH",
      flm_vendor_region: "",
      atm_count: 1,
      requested_atm_count: 0,
      unrequested_atm_count: 1,
      amount_replenish: 1000,
      unrequested_amount_replenish: 1000,
    },
  ],
};

/** Shared routing for the two non-forecast endpoints; null = not handled. */
function mockStaticEndpoints(path: string) {
  if (path.includes("/forecast/summary")) return { data: SUMMARY_RESPONSE, status: 200 };
  if (path.includes("/vendors")) return { data: VENDOR_OPTIONS_RESPONSE, status: 200 };
  return null;
}

function forecastCalls(): string[] {
  return mockApiGet.mock.calls.map(([p]) => p as string).filter((p) => p.includes("/forecast?"));
}

function mockSingleResponse(page: number, pageSize: number, totalCount: number): void {
  mockApiGet.mockImplementation(async (path: string) => {
    return (
      mockStaticEndpoints(path) ?? { data: makeResponse(page, pageSize, totalCount), status: 200 }
    );
  });
}

function makeRow(
  id: string,
  amountReplenish = 1000,
  overrides: Partial<ForecastRow> = {},
): ForecastRow {
  return {
    terminal_id: id,
    periode_pred: "2027-01-15",
    denom: 50000,
    amount_replenish: amountReplenish,
    amount_refund: 0,
    dmaa_file_id: 1,
    lokasi_atm: "",
    brand: "",
    priority_class: "",
    paket: "",
    escrow: null,
    flm_vendor: "TAG",
    flm_vendor_region: "Jawa Barat",
    is_requested: false,
    ...overrides,
  };
}

/** One page holding exactly `rows`. */
function mockRows(rows: ForecastRow[]): void {
  mockApiGet.mockImplementation(async (path: string) => {
    return (
      mockStaticEndpoints(path) ?? {
        data: {
          data: rows,
          pagination: { page: 1, page_size: 25, total_count: rows.length, total_pages: 1 },
        },
        status: 200,
      }
    );
  });
}

/** A synthetic dataset of `totalCount` distinct rows, paged however the requested URL asks. */
function mockPagedDataset(totalCount: number): void {
  mockApiGet.mockImplementation(async (path: string) => {
    const staticResponse = mockStaticEndpoints(path);
    if (staticResponse) return staticResponse;
    const { page, pageSize } = parsePageParams(path);
    const totalPages = Math.max(1, Math.ceil(totalCount / pageSize));
    const start = (page - 1) * pageSize;
    const count = Math.max(0, Math.min(pageSize, totalCount - start));
    const rows = Array.from({ length: count }, (_, i) => makeRow(`ATM-${start + i}`));
    return {
      data: {
        data: rows,
        pagination: {
          page,
          page_size: pageSize,
          total_count: totalCount,
          total_pages: totalPages,
        },
      },
      status: 200,
    };
  });
}

/** Whitespace-normalized page text — sidesteps text split across sibling DOM nodes. */
function pageText(): string {
  return document.body.textContent?.replace(/\s+/g, " ") ?? "";
}

function renderUnfiltered() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <ForecastBrowser />
    </QueryClientProvider>,
  );
}

// forecast-browser-summary relaxed CIT-2 Req 1.4: the page fetches without
// any vendor filter. The older tests below exercise vendor-scoped behavior
// (select-all needs a vendor), so renderPage still picks TAG / Jawa Barat.
async function renderPage() {
  const utils = renderUnfiltered();
  const user = userEvent.setup();
  const flmVendorSelect = await screen.findByLabelText("FLM Vendor");
  // Wait for useVendorOptions to resolve and populate the option, not just
  // for the (already-present, options-less) <select> element itself.
  await screen.findByRole("option", { name: "TAG" });
  await user.selectOptions(flmVendorSelect, "TAG");
  await user.selectOptions(screen.getByLabelText("FLM Vendor Region"), "Jawa Barat");
  return utils;
}

describe("ForecastBrowser — pagination footer (Task 5)", () => {
  beforeEach(() => {
    mockApiGet.mockReset();
    mockToast.mockReset();
  });

  it("resets to page 1 and refetches with the new page size when the selector changes", async () => {
    mockSingleResponse(1, 25, 60);
    const user = userEvent.setup();
    await renderPage();
    await waitFor(() => expect(mockApiGet).toHaveBeenCalled());

    const select = await screen.findByLabelText("Baris per halaman");
    mockSingleResponse(1, 50, 60);
    await user.selectOptions(select, "50");

    await waitFor(() => {
      const lastPath = mockApiGet.mock.calls.at(-1)?.[0] as string;
      expect(lastPath).toContain("page=1");
      expect(lastPath).toContain("page_size=50");
    });
  });

  it("disables Previous on page 1 while a later page is available (Next enabled)", async () => {
    mockSingleResponse(1, 25, 60); // 3 pages
    await renderPage();

    const prev = await screen.findByRole("button", { name: "Sebelumnya" });
    const next = screen.getByRole("button", { name: "Berikutnya" });
    await waitFor(() => expect(prev).toBeDisabled());
    expect(next).not.toBeDisabled();
  });

  it("disables both Previous and Next when there is exactly one page", async () => {
    mockSingleResponse(1, 25, 10); // 1 page
    await renderPage();

    const prev = await screen.findByRole("button", { name: "Sebelumnya" });
    const next = screen.getByRole("button", { name: "Berikutnya" });
    await waitFor(() => expect(prev).toBeDisabled());
    expect(next).toBeDisabled();
  });
});

describe("ForecastBrowser — cross-page select-all (Task 6)", () => {
  beforeEach(() => {
    mockApiGet.mockReset();
    mockToast.mockReset();
  });

  it("select-all across a >1-page result populates every row and sums amount_replenish", async () => {
    mockPagedDataset(150); // page_size=100 for select-all -> 2 pages (100 + 50)
    const user = userEvent.setup();
    await renderPage();

    const selectAllButton = await screen.findByRole("button", {
      name: "Pilih Semua Rekomendasi",
    });
    await waitFor(() => expect(selectAllButton).not.toBeDisabled());
    await user.click(selectAllButton);

    await waitFor(() => expect(pageText()).toContain("150 item terpilih"));
    expect(pageText()).toContain("Rp 150.000");
    // Two select-all pages (page_size=100) plus the initial page-1 browse call.
    const selectAllCalls = mockApiGet.mock.calls.filter(([p]) =>
      (p as string).includes("page_size=100"),
    );
    expect(selectAllCalls).toHaveLength(2);
  });

  it("preserves a prior manual tick through select-all (no row lost or double-counted)", async () => {
    mockPagedDataset(150);
    const user = userEvent.setup();
    await renderPage();

    const firstRowCheckbox = await screen.findByLabelText("Pilih baris ATM-0");
    await user.click(firstRowCheckbox);
    await waitFor(() => expect(pageText()).toContain("1 item terpilih"));

    const selectAllButton = screen.getByRole("button", { name: "Pilih Semua Rekomendasi" });
    await user.click(selectAllButton);

    await waitFor(() => expect(pageText()).toContain("150 item terpilih"));
    expect(firstRowCheckbox).toBeChecked();
  });

  it("shows a warning and applies no selection when the matching set exceeds the 1000-item cap", async () => {
    mockPagedDataset(1500);
    const user = userEvent.setup();
    await renderPage();

    const selectAllButton = await screen.findByRole("button", {
      name: "Pilih Semua Rekomendasi",
    });
    await waitFor(() => expect(selectAllButton).not.toBeDisabled());
    await user.click(selectAllButton);

    await waitFor(() => expect(mockToast).toHaveBeenCalled());
    const [toastArg] = mockToast.mock.calls.at(-1) ?? [];
    expect(toastArg).toMatchObject({ type: "warning" });
    expect(toastArg.message).toContain("1500");
    // Only the one page-1 probe call — no further pages fetched once over cap.
    const selectAllCalls = mockApiGet.mock.calls.filter(([p]) =>
      (p as string).includes("page_size=100"),
    );
    expect(selectAllCalls).toHaveLength(1);
    expect(pageText()).toContain("0 item terpilih");
  });

  it("clears the selection when the forecast date changes", async () => {
    mockPagedDataset(150);
    const user = userEvent.setup();
    await renderPage();

    const firstRowCheckbox = await screen.findByLabelText("Pilih baris ATM-0");
    await user.click(firstRowCheckbox);
    await waitFor(() => expect(pageText()).toContain("1 item terpilih"));

    const dateInput = screen.getByLabelText("Tanggal Forecast");
    await user.clear(dateInput);
    await user.type(dateInput, "2027-02-01");

    await waitFor(() => expect(pageText()).toContain("0 item terpilih"));
  });
});

describe("ForecastBrowser — vendor × region recap (forecast-browser-summary FR4)", () => {
  beforeEach(() => {
    mockApiGet.mockReset();
    mockToast.mockReset();
  });

  it("loads the recap and the unfiltered detail table without picking any filter", async () => {
    mockRows([makeRow("ATM-1")]);
    renderUnfiltered();

    expect(
      await screen.findByRole("table", { name: "Ringkasan per Vendor dan Region" }),
    ).toBeInTheDocument();
    expect(await screen.findByText("Perlu master data")).toBeInTheDocument();
    expect(await screen.findByLabelText("Pilih baris ATM-1")).toBeInTheDocument();

    const [firstForecast] = forecastCalls();
    expect(firstForecast).not.toContain("flm_vendor");
    expect(firstForecast).not.toContain("unassigned");
    expect(
      mockApiGet.mock.calls.some(([p]) =>
        (p as string).includes("/forecast/summary?forecast_date="),
      ),
    ).toBe(true);
  });

  it("clicking a recap row filters the detail table to that vendor and region", async () => {
    mockRows([makeRow("ATM-1")]);
    const user = userEvent.setup();
    renderUnfiltered();

    await user.click(await screen.findByRole("button", { name: "Lihat ATM TAG Jawa Barat" }));

    await waitFor(() => {
      const last = forecastCalls().at(-1) ?? "";
      expect(last).toContain("flm_vendor=TAG");
      expect(last).toContain("flm_vendor_region=Jawa+Barat");
    });
    expect(screen.getByLabelText("FLM Vendor")).toHaveValue("TAG");
    const activeRow = screen
      .getByRole("button", { name: "Lihat ATM TAG Jawa Barat" })
      .closest("tr");
    expect(activeRow).toHaveAttribute("aria-current", "true");
  });

  it("clicking the no-vendor recap row requests unassigned=true and shows a removable chip", async () => {
    mockRows([makeRow("ATM-U", 1000, { flm_vendor: "", flm_vendor_region: "" })]);
    const user = userEvent.setup();
    renderUnfiltered();

    await user.click(await screen.findByRole("button", { name: "Lihat ATM Tanpa vendor aktif" }));
    await waitFor(() => expect(forecastCalls().at(-1)).toContain("unassigned=true"));

    // Removing the chip returns to the unfiltered view — already cached from
    // the first render, so no new request; the chip itself must be gone.
    await user.click(screen.getByRole("button", { name: /Hanya ATM tanpa vendor aktif/ }));
    await waitFor(() =>
      expect(
        screen.queryByRole("button", { name: /Hanya ATM tanpa vendor aktif/ }),
      ).not.toBeInTheDocument(),
    );
  });

  it("clicking a vendor row without region requests no_region=true for that vendor (review R1)", async () => {
    mockRows([makeRow("ATM-R", 1000, { flm_vendor: "ROH", flm_vendor_region: "" })]);
    const user = userEvent.setup();
    renderUnfiltered();

    await user.click(await screen.findByRole("button", { name: "Lihat ATM ROH tanpa region" }));
    await waitFor(() => {
      const last = forecastCalls().at(-1) ?? "";
      expect(last).toContain("flm_vendor=ROH");
      expect(last).toContain("no_region=true");
      expect(last).not.toContain("flm_vendor_region");
    });
    const row = screen.getByRole("button", { name: "Lihat ATM ROH tanpa region" }).closest("tr");
    expect(row).toHaveAttribute("aria-current", "true");

    // Picking a region drops the recap-only flag (they can't be combined).
    await user.selectOptions(screen.getByLabelText("FLM Vendor Region"), "Jawa Barat");
    await waitFor(() => {
      const last = forecastCalls().at(-1) ?? "";
      expect(last).toContain("flm_vendor_region=Jawa+Barat");
      expect(last).not.toContain("no_region");
    });
    expect(screen.queryByRole("button", { name: /Tanpa region/ })).not.toBeInTheDocument();
  });

  it("disables Buat Vendor Request with a hint when the selection spans two vendors", async () => {
    mockRows([makeRow("ATM-1"), makeRow("ATM-2", 1000, { flm_vendor: "ROH" })]);
    const user = userEvent.setup();
    renderUnfiltered();

    await user.click(await screen.findByLabelText("Pilih baris ATM-1"));
    const create = screen.getByRole("button", { name: "Buat Vendor Request" });
    expect(create).not.toBeDisabled();

    await user.click(screen.getByLabelText("Pilih baris ATM-2"));
    expect(create).toBeDisabled();
    expect(screen.getByRole("status")).toHaveTextContent("Pilihan berisi 2 vendor");
  });

  it("disables Pilih Semua until a vendor is chosen, then skips already-requested rows", async () => {
    mockRows([makeRow("ATM-1"), makeRow("ATM-2", 1000, { is_requested: true })]);
    const user = userEvent.setup();
    renderUnfiltered();

    const selectAll = await screen.findByRole("button", { name: "Pilih Semua Rekomendasi" });
    expect(selectAll).toBeDisabled();

    await screen.findByRole("option", { name: "TAG" });
    await user.selectOptions(screen.getByLabelText("FLM Vendor"), "TAG");
    await waitFor(() => expect(selectAll).not.toBeDisabled());
    await user.click(selectAll);

    await waitFor(() => expect(pageText()).toContain("1 item terpilih"));
    expect(screen.getByLabelText("Pilih baris ATM-2")).toBeDisabled();
  });
});

describe("ForecastBrowser — FLM Vendor / Region filters (CIT-2 Req 1, now optional)", () => {
  beforeEach(() => {
    mockApiGet.mockReset();
    mockToast.mockReset();
  });

  it("fetches the forecast with the chosen vendor and region", async () => {
    mockSingleResponse(1, 25, 0);
    await renderPage(); // selects FLM Vendor="TAG" and FLM Vendor Region="Jawa Barat"

    await waitFor(() =>
      expect(mockApiGet.mock.calls.some(([p]) => (p as string).includes("/forecast?"))).toBe(true),
    );
    const lastPath = mockApiGet.mock.calls.at(-1)?.[0] as string;
    expect(lastPath).toContain("flm_vendor=TAG");
    expect(lastPath).toContain("flm_vendor_region=Jawa+Barat");
  });

  it("omits the brand param when Brand is left at Semua (empty)", async () => {
    mockSingleResponse(1, 25, 0);
    await renderPage();

    await waitFor(() => {
      const lastPath = mockApiGet.mock.calls.at(-1)?.[0] as string;
      expect(lastPath).toContain("/forecast?");
    });
    const lastPath = mockApiGet.mock.calls.at(-1)?.[0] as string;
    expect(lastPath).not.toContain("brand=");
  });

  it("resets to page 1 and refetches when a filter (Brand) changes after paging forward", async () => {
    mockSingleResponse(1, 25, 60);
    const user = userEvent.setup();
    await renderPage();
    await waitFor(() =>
      expect(mockApiGet.mock.calls.some(([p]) => (p as string).includes("/forecast?"))).toBe(true),
    );

    const next = await screen.findByRole("button", { name: "Berikutnya" });
    await user.click(next);
    await waitFor(() => {
      const lastPath = mockApiGet.mock.calls.at(-1)?.[0] as string;
      expect(lastPath).toContain("page=2");
    });

    await user.selectOptions(screen.getByLabelText("Brand"), "Hyosung");

    await waitFor(() => {
      const lastPath = mockApiGet.mock.calls.at(-1)?.[0] as string;
      expect(lastPath).toContain("page=1");
      expect(lastPath).toContain("brand=Hyosung");
    });
  });
});
