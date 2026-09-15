/**
 * Component tests for VendorRequestDetail (Req 14). No live backend in
 * this environment, so `../hooks` is mocked directly (same approach as
 * VendorRequestCreate.test.tsx) to drive the state×role×creator action
 * gating — the page's non-trivial logic — and the reject-modal validation.
 */

import { useAuthStore } from "@/lib/auth/store";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { VendorRequestDetail } from "../VendorRequestDetail";
import type { VendorRequestDetail as VendorRequestDetailType } from "../types";

const mutateAsyncOk = () => vi.fn().mockResolvedValue(undefined);

const useVendorRequestMock = vi.fn();

vi.mock("../hooks", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../hooks")>();
  return {
    ...actual,
    useVendorRequest: (...args: unknown[]) => useVendorRequestMock(...args),
    useUpdateVendorRequestItems: () => ({ mutateAsync: mutateAsyncOk(), isPending: false }),
    useSubmitVendorRequest: () => ({ mutateAsync: mutateAsyncOk(), isPending: false }),
    useApproveVendorRequest: () => ({ mutateAsync: mutateAsyncOk(), isPending: false }),
    useRejectVendorRequest: () => ({ mutateAsync: mutateAsyncOk(), isPending: false }),
    useReviseVendorRequest: () => ({ mutateAsync: mutateAsyncOk(), isPending: false }),
    useCancelVendorRequest: () => ({ mutateAsync: mutateAsyncOk(), isPending: false }),
  };
});

vi.mock("@tanstack/react-router", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    useParams: () => ({ id: "42" }),
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
      <VendorRequestDetail />
    </QueryClientProvider>,
  );
}

const BASE_DETAIL: VendorRequestDetailType = {
  id: 42,
  request_number: "VR-20260912-0001",
  forecast_date: "2026-09-12",
  status: "draft",
  notes: "",
  created_by: { id: 1, full_name: "Maker Satu" },
  approved_by: null,
  rejected_by: null,
  rejection_reason: "",
  created_at: "2026-09-12T01:00:00Z",
  updated_at: "2026-09-12T01:00:00Z",
  submitted_at: null,
  approved_at: null,
  rejected_at: null,
  items: [
    {
      id: 1,
      terminal_id: "ATM001",
      periode_pred: "2026-09-12",
      denom: 100000,
      amount_replenish: 5_000_000,
      amount_refund: 0,
    },
  ],
  total_amount: 5_000_000,
  replenish_date: "2026-09-13",
  request_category: null,
  is_canceled: false,
  is_manual: false,
  cancellation_reason: null,
};

function mockDetail(overrides: Partial<VendorRequestDetailType>) {
  useVendorRequestMock.mockReturnValue({
    data: { ...BASE_DETAIL, ...overrides },
    isLoading: false,
    isError: false,
    error: null,
  });
}

function setUser(id: number, role: string) {
  useAuthStore.setState({
    user: {
      id,
      username: "u",
      fullName: "U",
      email: "u@x.com",
      role: role as never,
      isKaryawan: true,
      vendorId: null,
    },
  });
}

beforeEach(() => {
  useVendorRequestMock.mockReset();
});

describe("VendorRequestDetail", () => {
  it("shows a loading skeleton while the query is pending", () => {
    useVendorRequestMock.mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
      error: null,
    });
    setUser(1, "ATM-USER");

    renderWithProviders();

    expect(screen.getByText("Vendor Request")).toBeInTheDocument();
  });

  it("shows a not-found state on a 404 error", () => {
    useVendorRequestMock.mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      error: { status: 404, message: "not found" },
    });
    setUser(1, "ATM-USER");

    renderWithProviders();

    expect(screen.getByRole("heading", { name: /tidak ditemukan/i })).toBeInTheDocument();
  });

  it("draft + creator sees Edit Item, Submit, and Cancel — not Approve/Reject", () => {
    mockDetail({ status: "draft" });
    setUser(1, "ATM-USER");

    renderWithProviders();

    expect(screen.getByRole("button", { name: "Edit Item" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /kirim untuk approval/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Batalkan" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Setujui" })).not.toBeInTheDocument();
  });

  it("pending_approval + non-creator checker sees Approve/Reject (and Cancel, CIT-2 Task 13.3 union widening)", () => {
    mockDetail({ status: "pending_approval" });
    setUser(2, "ATM-SPV");

    renderWithProviders();

    expect(screen.getByRole("button", { name: "Setujui" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Tolak" })).toBeInTheDocument();
  });

  it("pending_approval + creator sees Cancel only (union rule, not self-approval)", () => {
    mockDetail({ status: "pending_approval" });
    setUser(1, "ATM-USER");

    renderWithProviders();

    expect(screen.getByRole("button", { name: "Batalkan" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Setujui" })).not.toBeInTheDocument();
  });

  it("rejected + creator sees Revise", () => {
    mockDetail({ status: "rejected", rejection_reason: "Data salah" });
    setUser(1, "ATM-USER");

    renderWithProviders();

    expect(screen.getByRole("button", { name: /revisi/i })).toBeInTheDocument();
    expect(screen.getByText("Data salah")).toBeInTheDocument();
  });

  it("reject modal keeps Confirm disabled until a valid reason is typed", async () => {
    mockDetail({ status: "pending_approval" });
    setUser(2, "ATM-SPV");
    const user = userEvent.setup();

    renderWithProviders();

    await user.click(screen.getByRole("button", { name: "Tolak" }));
    const confirmButton = screen.getByRole("button", { name: /tolak request/i });
    expect(confirmButton).toBeDisabled();

    await user.type(screen.getByLabelText(/alasan penolakan/i), "Jumlah tidak sesuai");
    expect(confirmButton).toBeEnabled();
  });
});

describe("VendorRequestDetail — CIT-2 replenish_date/category/canceled (Task 13)", () => {
  it("shows Tanggal Replenish and Kategori as distinct labeled fields, '-' when null", () => {
    mockDetail({ status: "draft", replenish_date: null, request_category: null });
    setUser(1, "ATM-USER");

    renderWithProviders();

    expect(screen.getByText("Tanggal Replenish")).toBeInTheDocument();
    expect(screen.getByText("Kategori")).toBeInTheDocument();
    const replenishValue = screen.getByText("Tanggal Replenish").nextSibling;
    expect(replenishValue).toHaveTextContent("-");
    const categoryValue = screen.getByText("Kategori").nextSibling;
    expect(categoryValue).toHaveTextContent("-");
  });

  it("shows the canceled badge (icon + text, not color alone) when is_canceled", () => {
    mockDetail({ status: "draft", is_canceled: true });
    setUser(1, "ATM-USER");

    renderWithProviders();

    expect(screen.getByText("Dibatalkan")).toBeInTheDocument();
  });

  it("draft: non-creator checker (SPV) sees Cancel too (widened Maker+SPV union)", () => {
    mockDetail({ status: "draft" });
    setUser(2, "ATM-SPV");

    renderWithProviders();

    expect(screen.getByRole("button", { name: "Batalkan" })).toBeInTheDocument();
  });

  it("pending_approval: non-creator checker (SPV) sees both Approve/Reject and Cancel", () => {
    mockDetail({ status: "pending_approval" });
    setUser(2, "ATM-SPV");

    renderWithProviders();

    expect(screen.getByRole("button", { name: "Setujui" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Batalkan" })).toBeInTheDocument();
  });
});

describe("VendorRequestDetail — approved-cancel (Task 16, Req 3.1, 3.7)", () => {
  it("approved + checker sees Batalkan", () => {
    mockDetail({ status: "approved" });
    setUser(2, "ATM-SPV");

    renderWithProviders();

    expect(screen.getByRole("button", { name: "Batalkan" })).toBeInTheDocument();
  });

  it("approved + non-checker creator does NOT see Batalkan (Checker-only, no four-eyes exemption)", () => {
    mockDetail({ status: "approved" });
    setUser(1, "ATM-USER");

    renderWithProviders();

    expect(screen.queryByRole("button", { name: "Batalkan" })).not.toBeInTheDocument();
  });

  it("cancel modal keeps Confirm disabled until a valid reason is typed", async () => {
    mockDetail({ status: "approved" });
    setUser(2, "ATM-SPV");
    const user = userEvent.setup();

    renderWithProviders();

    await user.click(screen.getByRole("button", { name: "Batalkan" }));
    const confirmButton = screen.getByRole("button", { name: /batalkan request/i });
    expect(confirmButton).toBeDisabled();

    await user.type(screen.getByLabelText(/alasan pembatalan/i), "Order salah kirim");
    expect(confirmButton).toBeEnabled();
  });

  it("shows Alasan Pembatalan when cancellation_reason is present (Req 3.11 Opsi B)", () => {
    mockDetail({ status: "draft", is_canceled: true, cancellation_reason: "Vendor batal" });
    setUser(1, "ATM-USER");

    renderWithProviders();

    expect(screen.getByText("Alasan Pembatalan")).toBeInTheDocument();
    expect(screen.getByText("Vendor batal")).toBeInTheDocument();
  });
});
