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

import { RealizationsPage, rangeProblem } from "@/features/realizations/RealizationsPage";
import { OrderDetailPage } from "../OrderDetailPage";
import { type AtmDraft, draftErrors, toInput } from "../RealizationPanel";
import type { PartyAtm, VendorOrderDetail } from "../api";

// replenishment-realisasi S8: vendor realisasi report (FR2, FR5.1) + Realisasi page (FR5.2).

const ATM: PartyAtm = {
  terminal_id: "T0001",
  lokasi: "Mall A",
  ticket_number: "T0001_100K_20261009_001",
  denoms: [
    { denom: 100000, amount: 2000000 },
    { denom: 50000, amount: 500000 },
  ],
};

const order: VendorOrderDetail = {
  id: 11,
  role: "replenish",
  status: "accepted",
  request_number: "REP-GRD-JKT-20261009-001",
  replenish_date: "2026-10-09T00:00:00Z",
  request_status: "vendor_accepted",
  is_canceled: false,
  branch_id: 27,
  branch_code: "GRD-JKT",
  branch_name: "Gardanet Jakarta",
  atm_count: 2,
  vault_count: 0,
  totals: [{ denom: 100000, amount: 3000000 }],
  currency: "IDR",
  sent_at: "2026-10-08T03:00:00Z",
  decided_at: "2026-10-08T04:00:00Z",
  can_decide: false,
  rejection_reason: null,
  can_report: true,
  realizations: [],
  completion_rejection_reason: null,
  content: {
    role: "replenish",
    branch: {
      id: 27,
      code: "GRD-JKT",
      name: "Gardanet Jakarta",
      vendor_id: 4,
      vendor_name: "Gardanet",
    },
    atms: [
      ATM,
      {
        ...ATM,
        terminal_id: "T0002",
        ticket_number: "T0002_100K_20261009_001",
        denoms: [{ denom: 100000, amount: 1000000 }],
      },
    ],
    vaults: [],
    replenish_branches: [],
    totals: [{ denom: 100000, amount: 3000000 }],
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
  const detail = createRoute({
    path: "/orders/$id",
    getParentRoute: () => root,
    component: OrderDetailPage,
  });
  const list = createRoute({ path: "/orders", getParentRoute: () => root, component: () => null });
  const page = createRoute({
    path: "/realizations",
    getParentRoute: () => root,
    component: RealizationsPage,
  });
  const router = createRouter({
    routeTree: root.addChildren([list, detail, page]),
    history: createMemoryHistory({ initialEntries: [path] }),
  });
  return render(<RouterProvider router={router} />);
}

describe("draftErrors / toInput (FR2.2, K1)", () => {
  const ok: AtmDraft = {
    result: "success",
    date: "2026-10-09",
    amounts: { 100000: "2000000", 50000: "0" },
  };

  it("accepts a valid draft and builds the API row", () => {
    expect(draftErrors(ATM, ok, "2026-10-09", "2026-10-10")).toEqual([]);
    expect(toInput(ATM, ok)).toEqual({
      terminal_id: "T0001",
      result: "success",
      realized_date: "2026-10-09",
      amounts: { "100000": 2000000, "50000": 0 },
    });
    expect(toInput(ATM, { result: "failed", date: "", amounts: {} })).toEqual({
      terminal_id: "T0001",
      result: "failed",
    });
  });

  it("flags date bounds, non-multiples and an all-zero report", () => {
    expect(draftErrors(ATM, { ...ok, date: "2026-10-11" }, "2026-10-09", "2026-10-10")).toContain(
      "tanggal tidak boleh setelah hari ini",
    );
    expect(draftErrors(ATM, { ...ok, date: "2026-10-01" }, "2026-10-09", "2026-10-10")[0]).toMatch(
      /7 hari/,
    );
    expect(
      draftErrors(ATM, { ...ok, amounts: { 100000: "150000" } }, "2026-10-09", "2026-10-10")[0],
    ).toMatch(/kelipatan/);
    expect(
      draftErrors(ATM, { ...ok, amounts: { 100000: "0" } }, "2026-10-09", "2026-10-10"),
    ).toContain("isi minimal satu nominal");
    expect(
      draftErrors(ATM, { result: "failed", date: "", amounts: {} }, "2026-10-09", "2026-10-10"),
    ).toEqual([]);
  });
});

describe("RealizationPanel on the order detail (FR5.1)", () => {
  beforeEach(() => {
    getMock.mockReset();
    postMock.mockReset();
  });

  it("prefills the order, marks one ATM failed and posts every ATM", async () => {
    getMock.mockResolvedValue({ data: order });
    postMock.mockResolvedValue({ data: { ...order, can_report: false } });
    const user = userEvent.setup();
    renderAt("/orders/11");

    expect(await screen.findByRole("heading", { name: "Laporan realisasi" })).toBeInTheDocument();
    expect(screen.getByLabelText(/Nominal 100K/, { selector: "#rz-T0001-100000" })).toHaveValue(
      "2000000",
    );
    await user.click(screen.getAllByRole("radio", { name: "Gagal diisi" })[1] as HTMLElement);
    await user.click(screen.getByRole("button", { name: "Kirim laporan realisasi" }));

    await waitFor(() => expect(postMock).toHaveBeenCalled());
    const [path, body] = postMock.mock.calls[0] as [string, { realizations: unknown[] }];
    expect(path).toBe("/api/v1/vendor/replenish-orders/11/realization");
    expect(body.realizations).toEqual([
      expect.objectContaining({
        terminal_id: "T0001",
        result: "success",
        amounts: { "100000": 2000000, "50000": 500000 },
      }),
      { terminal_id: "T0002", result: "failed" },
    ]);
  });

  it("blocks sending a non-multiple amount", async () => {
    getMock.mockResolvedValue({ data: order });
    const user = userEvent.setup();
    renderAt("/orders/11");
    const input = await screen.findByLabelText(/Nominal 100K/, { selector: "#rz-T0001-100000" });
    await user.clear(input);
    await user.type(input, "150000");
    expect(screen.getByText(/harus kelipatan/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Kirim laporan realisasi" })).toBeDisabled();
  });

  it("shows CIMB's rejection reason, and the sent report once it can no longer be edited", async () => {
    getMock.mockResolvedValueOnce({
      data: { ...order, completion_rejection_reason: "Nominal T0001 salah" },
    });
    const first = renderAt("/orders/11");
    expect(await screen.findByText("Nominal T0001 salah")).toBeInTheDocument();
    first.unmount();

    getMock.mockResolvedValueOnce({
      data: {
        ...order,
        can_report: false,
        request_status: "completion_pending",
        realizations: [
          {
            terminal_id: "T0001",
            ticket_number: ATM.ticket_number,
            result: "success",
            realized_date: "2026-10-09T00:00:00Z",
            amounts: [{ denom: 100000, amount: 2000000 }],
            submitted_at: "2026-10-09T08:00:00Z",
            submitted_by_name: "Operator",
          },
        ],
      },
    });
    renderAt("/orders/11");
    expect(await screen.findByText("Berhasil")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Kirim laporan realisasi" }),
    ).not.toBeInTheDocument();
  });

  it("is hidden for a vault party", async () => {
    getMock.mockResolvedValue({
      data: { ...order, role: "vault", content: { ...order.content, role: "vault", atms: [] } },
    });
    renderAt("/orders/11");
    expect(await screen.findByText(order.request_number)).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "Laporan realisasi" })).not.toBeInTheDocument();
  });
});

describe("RealizationsPage (FR5.2)", () => {
  beforeEach(() => getMock.mockReset());

  it("lists the vendor's classifications with label, offset and amount gap", async () => {
    getMock.mockResolvedValue({
      data: {
        items: [
          {
            id: 1,
            ticket_number: ATM.ticket_number,
            request_number: order.request_number,
            terminal_id: "T0001",
            branch_code: "GRD-JKT",
            scheduled_date: "2026-10-09T00:00:00Z",
            realized_date: "2026-10-12T00:00:00Z",
            workday_offset: 1,
            classification: "late",
            has_amount_diff: true,
            vendor_mismatch: true,
            vendor_report_late: false,
            amounts: [
              {
                denom: 100000,
                order_amount: 2000000,
                itm_amount: 1500000,
                vendor_amount: 2000000,
                diff: -500000,
              },
            ],
          },
        ],
        total: 1,
        page: 1,
        page_size: 20,
      },
    });
    renderAt("/realizations");
    expect(await screen.findByText("Mundur (1 hari kerja)")).toBeInTheDocument();
    expect(screen.getByText("Beda dengan laporan Anda")).toBeInTheDocument();
    const url = getMock.mock.calls[0]?.[0] as string;
    expect(url).toMatch(
      /^\/api\/v1\/vendor\/replenish-realizations\?date_from=\d{4}-\d{2}-\d{2}&date_to=/,
    );
    expect(url).not.toContain("vendor_id");
  });

  it("rangeProblem allows up to 31 days", () => {
    expect(rangeProblem("2026-10-01", "2026-10-31")).toBeNull();
    expect(rangeProblem("2026-10-01", "2026-11-01")).toMatch(/31 hari/);
    expect(rangeProblem("2026-10-05", "2026-10-01")).toMatch(/sebelum/);
  });
});
