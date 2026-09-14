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
import type { ForecastResponse, ForecastRow } from "../types";

vi.mock("@tanstack/react-router", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return { ...actual, useNavigate: () => vi.fn() };
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

function mockSingleResponse(page: number, pageSize: number, totalCount: number): void {
  mockApiGet.mockResolvedValue({ data: makeResponse(page, pageSize, totalCount), status: 200 });
}

function makeRow(id: string, amountReplenish = 1000): ForecastRow {
  return {
    terminal_id: id,
    periode_pred: "2027-01-15",
    denom: 50000,
    amount_replenish: amountReplenish,
    amount_refund: 0,
    dmaa_file_id: 1,
    lokasi_atm: "",
    brand: "",
    flm_vendor: "",
    flm_vendor_region: "",
  };
}

/** A synthetic dataset of `totalCount` distinct rows, paged however the requested URL asks. */
function mockPagedDataset(totalCount: number): void {
  mockApiGet.mockImplementation(async (path: string) => {
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

function renderPage() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <ForecastBrowser />
    </QueryClientProvider>,
  );
}

describe("ForecastBrowser — pagination footer (Task 5)", () => {
  beforeEach(() => {
    mockApiGet.mockReset();
    mockToast.mockReset();
  });

  it("resets to page 1 and refetches with the new page size when the selector changes", async () => {
    mockSingleResponse(1, 25, 60);
    const user = userEvent.setup();
    renderPage();
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
    renderPage();

    const prev = await screen.findByRole("button", { name: "Sebelumnya" });
    const next = screen.getByRole("button", { name: "Berikutnya" });
    await waitFor(() => expect(prev).toBeDisabled());
    expect(next).not.toBeDisabled();
  });

  it("disables both Previous and Next when there is exactly one page", async () => {
    mockSingleResponse(1, 25, 10); // 1 page
    renderPage();

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
    renderPage();

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
    renderPage();

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
    renderPage();

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
    renderPage();

    const firstRowCheckbox = await screen.findByLabelText("Pilih baris ATM-0");
    await user.click(firstRowCheckbox);
    await waitFor(() => expect(pageText()).toContain("1 item terpilih"));

    const dateInput = screen.getByLabelText("Tanggal Forecast");
    await user.clear(dateInput);
    await user.type(dateInput, "2027-02-01");

    await waitFor(() => expect(pageText()).toContain("0 item terpilih"));
  });
});
