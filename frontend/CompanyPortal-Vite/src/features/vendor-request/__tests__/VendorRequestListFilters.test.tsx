/**
 * fe-coverage C5: Vendor Request list — filters, paging, row navigation, maker-only action.
 * Data hook + router mocked; the real URL-state hook runs.
 * (Replenish date / cancelled badge / include_canceled: VendorRequestList.test.tsx.)
 */

import { useAuthStore } from "@/lib/auth/store";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const m = vi.hoisted(() => ({
  navigate: vi.fn(),
  search: {} as Record<string, unknown>,
  list: {} as Record<string, unknown>,
  refetch: vi.fn(),
}));
vi.mock("../hooks", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../hooks")>();
  return { ...actual, useVendorRequests: () => ({ ...m.list, refetch: m.refetch }) };
});
vi.mock("@tanstack/react-router", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    useNavigate: () => m.navigate,
    useSearch: () => m.search,
    Link: ({ children, onClick }: { children?: ReactNode; onClick?: (e: unknown) => void }) => (
      <a href="/replenishment/vendor-requests/7" onClick={(e) => onClick?.(e)}>
        {children}
      </a>
    ),
  };
});

import { VendorRequestList } from "../VendorRequestList";

const ROW = {
  id: 7,
  request_number: "REP-G-JKT-20261016-001",
  forecast_date: "2026-10-15",
  status: "pending_approval",
  notes: "",
  item_count: 3,
  total_amount: 1_500_000_000,
  created_by: { id: 1, full_name: "Maker Satu" },
  approved_by: null,
  created_at: "2026-10-15T01:00:00Z",
  submitted_at: null,
  approved_at: null,
  rejected_at: null,
  replenish_date: null,
  request_category: null,
  is_canceled: false,
  is_manual: false,
  cancellation_reason: null,
};

function setRole(role: string) {
  useAuthStore.setState({
    user: {
      id: 1,
      username: "u",
      fullName: "U",
      email: "u@x",
      role: role as never,
      isKaryawan: true,
      vendorId: null,
    },
  });
}

const ui = () => (
  <QueryClientProvider client={new QueryClient()}>
    <VendorRequestList />
  </QueryClientProvider>
);

beforeEach(() => {
  m.navigate.mockReset();
  m.refetch.mockReset();
  m.search = {};
  m.list = {
    isLoading: false,
    isError: false,
    data: {
      data: [ROW],
      pagination: { page: 1, page_size: 10, total_count: 30, total_pages: 3 },
    },
  };
  setRole("ATM-USER");
});

describe("VendorRequestList filters + navigation", () => {
  it("toggles statuses (resets to page 1), and the summary counts them", async () => {
    m.search = { page: 2, status: ["draft"] };
    const user = userEvent.setup();
    render(ui());

    expect(screen.getByText("Status (1)")).toBeInTheDocument();
    await user.click(screen.getByRole("checkbox", { name: "Menunggu Approval" }));
    expect(m.navigate).toHaveBeenLastCalledWith({
      to: ".",
      search: { status: ["draft", "pending_approval"] },
    });
    await user.click(screen.getByRole("checkbox", { name: "Draft" }));
    expect(m.navigate).toHaveBeenLastCalledWith({ to: ".", search: {} });
  });

  it("filters by forecast date and page size (both back to page 1)", () => {
    m.search = { page: 3 };
    render(ui());

    fireEvent.change(screen.getByLabelText("Tanggal Forecast"), {
      target: { value: "2026-10-16" },
    });
    expect(m.navigate).toHaveBeenLastCalledWith({
      to: ".",
      search: { forecastDate: "2026-10-16" },
    });
    fireEvent.change(screen.getByLabelText("Baris per halaman"), { target: { value: "50" } });
    expect(m.navigate).toHaveBeenLastCalledWith({ to: ".", search: { pageSize: 50 } });
  });

  it("searching by request number is debounced into the URL", async () => {
    render(ui());

    await userEvent.setup().type(screen.getByLabelText("Cari No. Request"), "REP-G");

    expect(m.navigate).not.toHaveBeenCalled();
    await waitFor(() =>
      expect(m.navigate).toHaveBeenLastCalledWith({ to: ".", search: { search: "REP-G" } }),
    );
  });

  it("pages forward and back", async () => {
    m.search = { page: 2 };
    const user = userEvent.setup();
    render(ui());

    expect(screen.getByText("Halaman 2 dari 3")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Berikutnya" }));
    expect(m.navigate).toHaveBeenLastCalledWith({ to: ".", search: { page: 3 } });
    await user.click(screen.getByRole("button", { name: "Sebelumnya" }));
    expect(m.navigate).toHaveBeenLastCalledWith({ to: ".", search: {} });
  });

  it("a row click opens the detail; the 'Lihat Detail' link does not double-navigate", async () => {
    const user = userEvent.setup();
    render(ui());

    expect(screen.getByText("1.500.000.000")).toBeInTheDocument();
    await user.click(screen.getByText("REP-G-JKT-20261016-001"));
    expect(m.navigate).toHaveBeenCalledWith({
      to: "/replenishment/vendor-requests/$id",
      params: { id: "7" },
    });

    m.navigate.mockReset();
    await user.click(screen.getByText("Lihat Detail"));
    expect(m.navigate).not.toHaveBeenCalled();
  });

  it("only makers see 'Vendor Request Baru', which opens the Forecast Browser", async () => {
    render(ui());
    await userEvent.setup().click(screen.getByRole("button", { name: "Vendor Request Baru" }));
    expect(m.navigate).toHaveBeenCalledWith({ to: "/replenishment/forecast-browser" });
  });

  it("a checker (ATM-SPV) gets no create button", () => {
    setRole("ATM-SPV");
    render(ui());

    expect(screen.queryByRole("button", { name: "Vendor Request Baru" })).not.toBeInTheDocument();
  });

  it("shows loading, error (retry) and empty rows", async () => {
    m.list = { isLoading: true, isError: false, data: undefined };
    const { rerender } = render(ui());
    expect(screen.queryByText("REP-G-JKT-20261016-001")).not.toBeInTheDocument();
    expect(screen.getByText("Halaman 1 dari 1")).toBeInTheDocument();

    m.list = { isLoading: false, isError: true, data: undefined };
    rerender(ui());
    expect(screen.getByText("Gagal memuat daftar vendor request")).toBeInTheDocument();
    await userEvent.setup().click(screen.getByRole("button", { name: "Coba Lagi" }));
    expect(m.refetch).toHaveBeenCalled();

    m.list = {
      isLoading: false,
      isError: false,
      data: { data: [], pagination: { page: 1, page_size: 10, total_count: 0, total_pages: 0 } },
    };
    rerender(ui());
    expect(screen.getByText(/Tidak ada vendor request yang sesuai/)).toBeInTheDocument();
  });
});
