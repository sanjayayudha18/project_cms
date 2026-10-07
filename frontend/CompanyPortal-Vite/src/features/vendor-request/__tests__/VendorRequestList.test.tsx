/**
 * Component tests for VendorRequestList (Task 13.4): canceled badge on
 * rows, replenish_date column, and the default-hides-canceled toggle
 * (CIT-2 Req 5.5, 5.6, 2.9). `../hooks` is mocked directly (same approach
 * as VendorRequestDetail.test.tsx) — no live backend in this environment.
 */

import { useAuthStore } from "@/lib/auth/store";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { VendorRequestList } from "../VendorRequestList";
import type { VendorRequestListResponse, VendorRequestSummary } from "../types";

const useVendorRequestsMock = vi.fn();

vi.mock("../hooks", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../hooks")>();
  return {
    ...actual,
    useVendorRequests: (...args: unknown[]) => useVendorRequestsMock(...args),
  };
});

vi.mock("@tanstack/react-router", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    useNavigate: () => vi.fn(),
    useSearch: () => ({}),
    Link: ({ children, className }: { children?: ReactNode; className?: string }) => (
      <a href="/replenishment/vendor-requests" className={className}>
        {children}
      </a>
    ),
  };
});

function renderWithProviders() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <VendorRequestList />
    </QueryClientProvider>,
  );
}

const ROW: VendorRequestSummary = {
  id: 1,
  request_number: "VR-20260912-0001",
  forecast_date: "2026-09-12",
  status: "draft",
  notes: "",
  item_count: 1,
  total_amount: 5_000_000,
  created_by: { id: 1, full_name: "Maker Satu" },
  approved_by: null,
  created_at: "2026-09-12T01:00:00Z",
  submitted_at: null,
  approved_at: null,
  rejected_at: null,
  replenish_date: "2026-09-13",
  request_category: null,
  is_canceled: false,
  is_manual: false,
  cancellation_reason: null,
};

function mockRows(rows: VendorRequestSummary[]) {
  const response: VendorRequestListResponse = {
    data: rows,
    pagination: { page: 1, page_size: 10, total_count: rows.length, total_pages: 1 },
  };
  useVendorRequestsMock.mockReturnValue({
    data: response,
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  });
}

beforeEach(() => {
  useVendorRequestsMock.mockReset();
  useAuthStore.setState({
    user: {
      id: 1,
      username: "u",
      fullName: "U",
      email: "u@x.com",
      role: "ATM-USER" as never,
      isKaryawan: true,
      vendorId: null,
    },
  });
});

describe("VendorRequestList — CIT-2 replenish_date + canceled badge (Task 13.2)", () => {
  it("shows the Tanggal Replenish column value for each row", () => {
    mockRows([ROW]);

    renderWithProviders();

    expect(screen.getByText("Tanggal Replenish")).toBeInTheDocument();
  });

  it("shows a canceled badge (icon + text, not color alone) when is_canceled", () => {
    mockRows([{ ...ROW, is_canceled: true }]);

    renderWithProviders();

    const row = screen.getByText(ROW.request_number).closest("tr");
    expect(row).toHaveTextContent("Dibatalkan");
  });

  it("requests without include_canceled by default, and toggles it on when checked", async () => {
    mockRows([ROW]);
    const user = userEvent.setup();

    renderWithProviders();

    expect(useVendorRequestsMock).toHaveBeenLastCalledWith(
      expect.objectContaining({ includeCanceled: false }),
    );

    await user.click(screen.getByRole("checkbox", { name: /tampilkan yang dibatalkan/i }));

    expect(useVendorRequestsMock).toHaveBeenLastCalledWith(
      expect.objectContaining({ includeCanceled: true }),
    );
  });
});
