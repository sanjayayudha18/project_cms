import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  Outlet,
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

const getMock = vi.fn();
const postMock = vi.fn();
vi.mock("@/lib/api/client", () => ({
  api: {
    get: (...args: unknown[]) => getMock(...args),
    post: (...args: unknown[]) => postMock(...args),
  },
}));

import { OrderDetailPage } from "../OrderDetailPage";
import { OrdersPage } from "../OrdersPage";
import type { VendorOrderDetail, VendorOrderSummary } from "../api";

const summary: VendorOrderSummary = {
  id: 11,
  role: "replenish",
  status: "pending",
  request_number: "REP-GRD-JKT-20261009-001",
  replenish_date: "2026-10-09T00:00:00Z",
  request_status: "sent_to_vendor",
  is_canceled: false,
  branch_id: 27,
  branch_code: "GRD-JKT",
  branch_name: "Gardanet Jakarta",
  atm_count: 2,
  totals: [{ denom: 100000, amount: 2000000 }],
  currency: "IDR",
  sent_at: "2026-10-08T03:00:00Z",
  decided_at: null,
  can_decide: true,
};

const vaultDetail: VendorOrderDetail = {
  ...summary,
  id: 12,
  role: "vault",
  atm_count: 1,
  rejection_reason: null,
  content: {
    role: "vault",
    branch: { id: 30, code: "C1", name: "Cash Satu", vendor_id: 4, vendor_name: "Gardanet" },
    atms: [
      {
        terminal_id: "T0001",
        lokasi: "Mall A",
        ticket_number: "T0001_100K_20261009_001",
        denoms: [{ denom: 100000, amount: 1000000 }],
        counterpart: {
          id: 27,
          code: "GRD-JKT",
          name: "Gardanet Jakarta",
          vendor_id: 4,
          vendor_name: "Gardanet",
        },
      },
    ],
    totals: [{ denom: 100000, amount: 1000000 }],
    currency: "IDR",
  },
};

function renderAt(path: string) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const root = createRootRoute({
    component: () => (
      <QueryClientProvider client={client}>
        <Outlet />
      </QueryClientProvider>
    ),
  });
  const list = createRoute({ path: "/orders", getParentRoute: () => root, component: OrdersPage });
  const detail = createRoute({
    path: "/orders/$id",
    getParentRoute: () => root,
    component: OrderDetailPage,
  });
  const router = createRouter({
    routeTree: root.addChildren([list, detail]),
    history: createMemoryHistory({ initialEntries: [path] }),
  });
  return render(<RouterProvider router={router} />);
}

describe("OrdersPage (cit-send-vendor FR9.1)", () => {
  beforeEach(() => {
    getMock.mockReset();
    postMock.mockReset();
  });

  it("lists orders from the API with role, totals and a text status", async () => {
    const canceled = { ...summary, id: 13, request_number: "REP-X-002", is_canceled: true };
    getMock.mockResolvedValue({
      data: { items: [summary, canceled], total: 2, page: 1, page_size: 20 },
    });
    renderAt("/orders");

    const link = await screen.findByRole("link", { name: summary.request_number });
    expect(link).toHaveAttribute("href", "/orders/11");
    expect(screen.getAllByText("Replenish (pelaksana)")).toHaveLength(2);
    expect(screen.getAllByText("100K IDR 2.000.000")).toHaveLength(2);
    expect(screen.getByText("Dibatalkan")).toBeInTheDocument();
    expect(getMock).toHaveBeenCalledWith(expect.not.stringContaining("vendor_id"));
  });

  it("filters on the server and resets to page 1", async () => {
    const user = userEvent.setup();
    getMock.mockResolvedValue({ data: { items: [summary], total: 45, page: 1, page_size: 20 } });
    renderAt("/orders");
    await screen.findByText(summary.request_number);

    await user.click(screen.getByRole("button", { name: "Berikutnya" }));
    await waitFor(() =>
      expect(getMock).toHaveBeenLastCalledWith(expect.stringContaining("page=2")),
    );
    await user.click(screen.getByRole("tab", { name: "Ditolak" }));
    await waitFor(() =>
      expect(getMock).toHaveBeenLastCalledWith(
        expect.stringMatching(/page=1.*party_status=rejected/),
      ),
    );
  });

  it("shows an empty state", async () => {
    getMock.mockResolvedValue({ data: { items: [], total: 0, page: 1, page_size: 20 } });
    renderAt("/orders");
    expect(await screen.findByText("Belum ada request")).toBeInTheDocument();
  });
});

describe("OrderDetailPage (cit-send-vendor FR3/FR4/FR9.2)", () => {
  beforeEach(() => {
    getMock.mockReset();
    postMock.mockReset();
  });

  it("shows a vault party what to prepare and who collects, and accepts", async () => {
    const user = userEvent.setup();
    getMock.mockResolvedValue({ data: vaultDetail });
    postMock.mockResolvedValue({ data: { ...vaultDetail, status: "accepted", can_decide: false } });
    renderAt("/orders/12");

    expect(await screen.findByText("Total uang yang harus disiapkan")).toBeInTheDocument();
    expect(screen.getByText("IDR 1.000.000")).toBeInTheDocument();
    expect(screen.getByText("Diambil oleh (replenish)")).toBeInTheDocument();
    expect(screen.queryByText("Tiket")).not.toBeInTheDocument();
    expect(getMock).toHaveBeenCalledWith("/api/v1/vendor/replenish-orders/12");

    await user.click(screen.getByRole("button", { name: "Terima" }));
    await waitFor(() =>
      expect(postMock).toHaveBeenCalledWith("/api/v1/vendor/replenish-orders/12/accept"),
    );
    expect(await screen.findByText("Diterima")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Terima" })).not.toBeInTheDocument();
  });

  it("requires a 10-character reason to reject", async () => {
    const user = userEvent.setup();
    getMock.mockResolvedValue({ data: vaultDetail });
    postMock.mockResolvedValue({ data: { ...vaultDetail, status: "rejected", can_decide: false } });
    renderAt("/orders/12");

    await user.click(await screen.findByRole("button", { name: "Tolak" }));
    const submit = screen.getByRole("button", { name: "Kirim penolakan" });
    await user.type(screen.getByLabelText(/Alasan penolakan/), "pendek");
    expect(submit).toBeDisabled();
    await user.type(screen.getByLabelText(/Alasan penolakan/), " sekali ya");
    expect(submit).toBeEnabled();
    await user.click(submit);
    await waitFor(() =>
      expect(postMock).toHaveBeenCalledWith("/api/v1/vendor/replenish-orders/12/reject", {
        reason: "pendek sekali ya",
      }),
    );
  });

  it("explains a 409 when the order can no longer be decided", async () => {
    const user = userEvent.setup();
    getMock.mockResolvedValue({ data: vaultDetail });
    postMock.mockRejectedValue({ status: 409, message: "conflict" });
    renderAt("/orders/12");
    await user.click(await screen.findByRole("button", { name: "Terima" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(/sudah tidak dapat diterima/);
  });

  it("shows no actions on a request being revised by CIMB", async () => {
    getMock.mockResolvedValue({
      data: { ...vaultDetail, request_status: "vault_assignment", can_decide: false },
    });
    renderAt("/orders/12");
    expect(await screen.findByText("Sedang direvisi CIMB")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Terima" })).not.toBeInTheDocument();
  });

  it("shows a not-found message for an order outside the vendor's scope", async () => {
    getMock.mockRejectedValue({ status: 404, message: "not found" });
    renderAt("/orders/99");
    expect(await screen.findByRole("alert")).toHaveTextContent(/tidak ditemukan/);
  });
});
