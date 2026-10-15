/**
 * fe-coverage C4: vendor-request api + hooks (order flow, laporan selesai + kuota).
 * Only the HTTP client is mocked, so real fetchers and real TanStack hooks run.
 * vendorSide.ts is intentionally not covered here: its paths carry a duplicated
 * "/api/v1" prefix (reported in fe-coverage/tests.md C4), and a test must not lock that in.
 */

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const client = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), put: vi.fn() }));
vi.mock("@/lib/api/client", () => ({ api: client }));

import { fetchAllForecastForSelection } from "../api";
import * as h from "../hooks";

type Method = keyof typeof client;
type Query = { isSuccess: boolean; data: unknown; fetchStatus: string };
type Mutation = { mutateAsync: (v: unknown) => Promise<unknown> };

function setup<T>(hook: () => T) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const invalidate = vi.spyOn(queryClient, "invalidateQueries");
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  return { ...renderHook(hook, { wrapper }), invalidate };
}

const invalidatedKeys = (spy: { mock: { calls: unknown[][] } }) =>
  spy.mock.calls.map((c) => (c[0] as { queryKey: unknown }).queryKey);

const q = (hook: () => unknown) => hook as () => Query;
const m = (hook: () => unknown) => hook as () => Mutation;

const page = (n: number, totalPages: number, totalCount: number) => ({
  data: {
    data: [{ terminal_id: `T${n}` }],
    pagination: { page: n, total_pages: totalPages, total_count: totalCount },
  },
});

beforeEach(() => {
  client.get.mockReset();
  client.post.mockReset();
  client.put.mockReset();
});

describe("vendor-request reads", () => {
  it.each<[string, () => Query, string]>([
    ["vendor options", q(h.useVendorOptions), "/vendor-requests/vendors"],
    [
      "forecast browse (date only → page 1, 20 rows)",
      q(() => h.useForecastBrowse({ forecastDate: "2026-10-16" })),
      "/vendor-requests/forecast?forecast_date=2026-10-16&page=1&page_size=20",
    ],
    [
      "forecast browse (every filter)",
      q(() =>
        h.useForecastBrowse({
          forecastDate: "2026-10-16",
          atmId: "T001",
          brand: "NCR",
          flmVendor: "GARDA",
          flmVendorRegion: "JKT",
          unassigned: true,
          noRegion: true,
          page: 3,
          pageSize: 50,
        } as never),
      ),
      "/vendor-requests/forecast?forecast_date=2026-10-16&atm_id=T001&brand=NCR&flm_vendor=GARDA&flm_vendor_region=JKT&unassigned=true&no_region=true&page=3&page_size=50",
    ],
    [
      "forecast summary",
      q(() => h.useForecastSummary("2026-10-16")),
      "/vendor-requests/forecast/summary?forecast_date=2026-10-16",
    ],
    [
      "request list (defaults: page 1, 10 rows)",
      q(() => h.useVendorRequests({})),
      "/vendor-requests?page=1&page_size=10",
    ],
    [
      "request list (every filter)",
      q(() =>
        h.useVendorRequests({
          status: ["submitted", "approved"],
          forecastDate: "2026-10-16",
          createdBy: 7,
          requestNumber: "REP-G-JKT",
          includeCanceled: true,
          page: 2,
          pageSize: 25,
        } as never),
      ),
      "/vendor-requests?status=submitted%2Capproved&forecast_date=2026-10-16&created_by=7&request_number=REP-G-JKT&include_canceled=true&page=2&page_size=25",
    ],
    ["request detail", q(() => h.useVendorRequest(5)), "/vendor-requests/5"],
    ["audit log", q(() => h.useVendorRequestAuditLog(5)), "/vendor-requests/5/audit-log"],
  ])("%s", async (_n, hook, path) => {
    client.get.mockResolvedValue({ data: { ok: true } });
    const { result } = setup(hook);

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(client.get).toHaveBeenCalledWith(path);
    expect(result.current.data).toEqual({ ok: true });
  });

  it.each<[string, () => Query]>([
    ["forecast browse without a date", q(() => h.useForecastBrowse({ forecastDate: "" }))],
    [
      "forecast browse when disabled",
      q(() => h.useForecastBrowse({ forecastDate: "2026-10-16" }, false)),
    ],
    ["forecast summary without a date", q(() => h.useForecastSummary(""))],
    ["detail without an id", q(() => h.useVendorRequest(null))],
    ["audit log without an id", q(() => h.useVendorRequestAuditLog(null))],
    [
      "select-all until explicitly refetched",
      q(() => h.useForecastSelectAll({ forecastDate: "2026-10-16" }, 500)),
    ],
  ])("%s stays idle", (_n, hook) => {
    const { result } = setup(hook);

    expect(result.current.fetchStatus).toBe("idle");
    expect(client.get).not.toHaveBeenCalled();
  });
});

describe("fetchAllForecastForSelection (select all across pages)", () => {
  it("pages through every page at 100 rows when the total fits under the cap", async () => {
    client.get
      .mockResolvedValueOnce(page(1, 3, 250))
      .mockResolvedValueOnce(page(2, 3, 250))
      .mockResolvedValueOnce(page(3, 3, 250));

    const result = await fetchAllForecastForSelection({ forecastDate: "2026-10-16" }, 500);

    expect(result).toEqual({
      rows: [{ terminal_id: "T1" }, { terminal_id: "T2" }, { terminal_id: "T3" }],
      totalCount: 250,
      exceededCap: false,
    });
    expect(client.get.mock.calls.map((c) => c[0])).toEqual([
      "/vendor-requests/forecast?forecast_date=2026-10-16&page=1&page_size=100",
      "/vendor-requests/forecast?forecast_date=2026-10-16&page=2&page_size=100",
      "/vendor-requests/forecast?forecast_date=2026-10-16&page=3&page_size=100",
    ]);
  });

  it("stops after page 1 with no rows when the total exceeds the cap (never a partial selection)", async () => {
    client.get.mockResolvedValueOnce(page(1, 12, 1150));

    const result = await fetchAllForecastForSelection({ forecastDate: "2026-10-16" }, 1000);

    expect(result).toEqual({ rows: [], totalCount: 1150, exceededCap: true });
    expect(client.get).toHaveBeenCalledTimes(1);
  });
});

const ITEMS = [{ terminal_id: "T001", denom: 100000, amount: "500000000" }];
const RESULTS = [{ terminal_id: "T001", result: "success" }];

describe("vendor-request actions", () => {
  it.each<[string, () => Mutation, unknown, [Method, ...unknown[]]]>([
    [
      "create",
      m(h.useCreateVendorRequest),
      { forecast_date: "2026-10-16", items: ITEMS },
      ["post", "/vendor-requests", { forecast_date: "2026-10-16", items: ITEMS }],
    ],
    [
      "edit items",
      m(h.useUpdateVendorRequestItems),
      { id: 5, items: ITEMS },
      ["put", "/vendor-requests/5/items", { items: ITEMS }],
    ],
    ["submit", m(h.useSubmitVendorRequest), 5, ["post", "/vendor-requests/5/submit"]],
    ["approve", m(h.useApproveVendorRequest), 5, ["post", "/vendor-requests/5/approve"]],
    [
      "reject",
      m(h.useRejectVendorRequest),
      { id: 5, reason: "Nominal salah" },
      ["post", "/vendor-requests/5/reject", { rejection_reason: "Nominal salah" }],
    ],
    ["revise", m(h.useReviseVendorRequest), 5, ["post", "/vendor-requests/5/revise"]],
    [
      "cancel",
      m(h.useCancelVendorRequest),
      { id: 5, reason: "ATM rusak" },
      ["post", "/vendor-requests/5/cancel", { cancellation_reason: "ATM rusak" }],
    ],
    [
      "submit laporan selesai",
      m(h.useSubmitVendorRequestCompletion),
      { id: 5, results: RESULTS },
      ["post", "/vendor-requests/5/complete", { results: RESULTS }],
    ],
    [
      "reject laporan selesai",
      m(h.useRejectVendorRequestCompletion),
      { id: 5, reason: "Foto tidak jelas" },
      ["post", "/vendor-requests/5/complete/reject", { reason: "Foto tidak jelas" }],
    ],
  ])("%s → endpoint, then every vendor-request query refreshes", async (_n, hook, vars, call) => {
    client.post.mockResolvedValue({ data: { id: 5 } });
    client.put.mockResolvedValue({ data: { id: 5 } });
    const { result, invalidate } = setup(hook);

    await act(async () => {
      await result.current.mutateAsync(vars);
    });

    const [method, ...args] = call;
    expect(client[method]).toHaveBeenCalledWith(...args);
    expect(invalidatedKeys(invalidate)).toEqual([h.vendorRequestKeys.all]);
  });

  it("approving a laporan selesai also refreshes the ATM visit quota", async () => {
    client.post.mockResolvedValue({ data: { id: 5, over_quota_terminals: ["T001"] } });
    const { result, invalidate } = setup(() => h.useApproveVendorRequestCompletion());

    let returned: unknown;
    await act(async () => {
      returned = await result.current.mutateAsync(5);
    });

    expect(client.post).toHaveBeenCalledWith("/vendor-requests/5/complete/approve");
    expect(returned).toEqual({ id: 5, over_quota_terminals: ["T001"] });
    expect(invalidatedKeys(invalidate)).toEqual([h.vendorRequestKeys.all, ["atm-visit-quota"]]);
  });

  it("a rejected action (e.g. maker approving own request) refreshes nothing", async () => {
    client.post.mockRejectedValue({ status: 403, message: "Maker tidak boleh menyetujui" });
    const { result, invalidate } = setup(() => h.useApproveVendorRequest());

    await act(async () => {
      await result.current.mutateAsync(5).catch(() => {});
    });

    expect(invalidate).not.toHaveBeenCalled();
  });
});

describe("getVendorRequestErrorMessage", () => {
  it.each([
    [new Error("boom"), "boom"],
    [{ status: 409, message: "Status sudah berubah" }, "Status sudah berubah"],
    [{ status: 500, message: 42 }, "Terjadi kesalahan yang tidak diketahui"],
    [null, "Terjadi kesalahan yang tidak diketahui"],
    ["text", "Terjadi kesalahan yang tidak diketahui"],
  ])("%j → %s", (err, expected) => {
    expect(h.getVendorRequestErrorMessage(err)).toBe(expected);
  });
});
