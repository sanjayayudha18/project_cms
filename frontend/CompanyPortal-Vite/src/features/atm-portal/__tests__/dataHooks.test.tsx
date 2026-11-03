/**
 * fe-coverage C4: atm-portal data layer — visit quota (kuota kunjungan), portal list /
 * cashpos, ATM profile history, profile URL state. HTTP client + router are mocked;
 * real TanStack hooks run.
 */

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const client = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }));
vi.mock("@/lib/api/client", () => ({ api: client }));

const nav = vi.hoisted(() => ({ navigate: vi.fn(), search: {} as Record<string, unknown> }));
vi.mock("@tanstack/react-router", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return { ...actual, useNavigate: () => nav.navigate, useSearch: () => nav.search };
});

import { useAtmCashposData, useAtmPortalData } from "../useAtmPortalData";
import {
  useAtmCashposHistory,
  useAtmMasterData,
  useAtmReplenishHistory,
} from "../useAtmProfileData";
import { useAtmProfileUrlState } from "../useAtmProfileUrlState";
import {
  useAtmVisitQuota,
  useCancelAtmVisit,
  useResetAtmQuota,
  useResetVendorQuota,
  visitSisa,
} from "../visitQuota";

/** `useHookRetry` keeps each hook's own retry policy (only the delay is shortened). */
function setup<T>(hook: () => T, useHookRetry = false) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: useHookRetry ? { retryDelay: 1 } : { retry: false },
      mutations: { retry: false },
    },
  });
  const invalidate = vi.spyOn(queryClient, "invalidateQueries");
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  return { ...renderHook(hook, { wrapper }), invalidate };
}

const invalidatedKeys = (spy: { mock: { calls: unknown[][] } }) =>
  spy.mock.calls.map((c) => (c[0] as { queryKey: unknown }).queryKey);

type Mutation = { mutateAsync: (v: unknown) => Promise<unknown> };

const portalParams = {
  mode: "replenish",
  page: 2,
  page_size: 25,
  search: "T00 1",
  status: "active",
  machine_type: "ATM",
  brand: "NCR",
  deployment_type: "offsite",
  region: "JKT",
  date_from: "2026-10-01",
  date_to: "2026-10-15",
  sort_by: "terminal_id",
  sort_order: "asc",
};

const history = { page: 1, page_size: 25, date_from: "2026-09-15", date_to: "2026-10-15" };

beforeEach(() => {
  client.get.mockReset();
  client.post.mockReset();
  nav.navigate.mockReset();
  nav.search = {};
});

describe("kuota kunjungan", () => {
  it.each([
    [5, { sisa: 5, kelebihan: 0 }],
    [0, { sisa: 0, kelebihan: 0 }],
    [-3, { sisa: 0, kelebihan: 3 }],
  ])("visitSisa(%i) splits remaining into sisa / kelebihan", (remaining, expected) => {
    expect(visitSisa(remaining)).toEqual(expected);
  });

  it("loads one ATM's quota (terminal id URL-encoded) and stays idle without one", async () => {
    const idle = setup(() => useAtmVisitQuota(""));
    expect(idle.result.current.fetchStatus).toBe("idle");

    client.get.mockResolvedValue({ data: { terminal_id: "T 01/A", remaining: -1 } });
    const { result } = setup(() => useAtmVisitQuota("T 01/A"));
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(client.get).toHaveBeenCalledWith("/atm-visit-quotas/atms/T%2001%2FA");
  });

  it.each<[string, () => Mutation, unknown, unknown[]]>([
    [
      "reset one ATM",
      () => useResetAtmQuota() as unknown as Mutation,
      "T 01",
      ["/atm-visit-quotas/atms/T%2001/reset"],
    ],
    [
      "reset a vendor",
      () => useResetVendorQuota() as unknown as Mutation,
      42,
      ["/atm-visit-quotas/vendors/42/reset"],
    ],
    [
      "cancel a visit",
      () => useCancelAtmVisit() as unknown as Mutation,
      { visitId: 7, reason: "Salah hitung" },
      ["/atm-visit-quotas/visits/7/cancel", { reason: "Salah hitung" }],
    ],
  ])("%s → endpoint, then quota + vendor requests refresh", async (_n, hook, vars, args) => {
    client.post.mockResolvedValue({ data: { reset_count: 3, skipped_count: 1 } });
    const { result, invalidate } = setup(hook);

    await act(async () => {
      await result.current.mutateAsync(vars);
    });

    expect(client.post).toHaveBeenCalledWith(...args);
    expect(invalidatedKeys(invalidate)).toEqual([["atm-visit-quota"], ["vendor-requests"]]);
  });

  it("a refused reset (e.g. RBAC 403) refreshes nothing", async () => {
    client.post.mockRejectedValue({ status: 403, message: "Akses ditolak" });
    const { result, invalidate } = setup(() => useResetAtmQuota());

    await act(async () => {
      await result.current.mutateAsync("T01").catch(() => {});
    });

    expect(invalidate).not.toHaveBeenCalled();
  });
});

describe("ATM portal list + cash position", () => {
  it("lists ATMs with every filter in the query string", async () => {
    client.get.mockResolvedValue({ data: { data: [], pagination: {} } });
    const { result } = setup(() => useAtmPortalData(portalParams as never));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(client.get).toHaveBeenCalledWith(
      "/atm-portal/atms?page=2&page_size=25&search=T00+1&status=active&machine_type=ATM&brand=NCR&deployment_type=offsite&region=JKT&date_from=2026-10-01&date_to=2026-10-15&sort_by=terminal_id&sort_order=asc",
    );
  });

  it("cash position loads only in cashpos mode, with only its own filters", async () => {
    const off = setup(() => useAtmCashposData(portalParams as never));
    expect(off.result.current.fetchStatus).toBe("idle");

    client.get.mockResolvedValue({ data: { data: [] } });
    const on = setup(() => useAtmCashposData({ ...portalParams, mode: "cashpos" } as never));
    await waitFor(() => expect(on.result.current.isSuccess).toBe(true));
    expect(client.get).toHaveBeenCalledWith(
      "/atm-portal/cashpos?page=2&page_size=25&search=T00+1&date_from=2026-10-01&date_to=2026-10-15&sort_by=terminal_id&sort_order=asc",
    );
  });
});

describe("ATM profile", () => {
  it("loads master data and both histories (only when enabled)", async () => {
    client.get.mockResolvedValue({ data: { ok: true } });

    const master = setup(() => useAtmMasterData("T01"));
    const replenish = setup(() => useAtmReplenishHistory("T01", history, true));
    const cashposOff = setup(() => useAtmCashposHistory("T01", history, false));
    const cashposOn = setup(() => useAtmCashposHistory("T01", history, true));

    await waitFor(() => expect(master.result.current.isSuccess).toBe(true));
    await waitFor(() => expect(replenish.result.current.isSuccess).toBe(true));
    await waitFor(() => expect(cashposOn.result.current.isSuccess).toBe(true));
    expect(cashposOff.result.current.fetchStatus).toBe("idle");
    expect(client.get.mock.calls.map((c) => c[0]).sort()).toEqual([
      "/atm-portal/atms/T01",
      "/atm-portal/atms/T01/cashpos?page=1&page_size=25&date_from=2026-09-15&date_to=2026-10-15",
      "/atm-portal/atms/T01/replenish?page=1&page_size=25&date_from=2026-09-15&date_to=2026-10-15",
    ]);
  });

  it("does not retry a 404 (unknown terminal), retries a 500 once", async () => {
    client.get.mockRejectedValue({ status: 404, message: "ATM tidak ditemukan" });
    const notFound = setup(() => useAtmMasterData("NOPE"), true);
    await waitFor(() => expect(notFound.result.current.isError).toBe(true));
    expect(client.get).toHaveBeenCalledTimes(1);

    client.get.mockReset();
    client.get.mockRejectedValue({ status: 500, message: "boom" });
    const serverError = setup(() => useAtmReplenishHistory("T01", history, true), true);
    await waitFor(() => expect(serverError.result.current.isError).toBe(true), {
      timeout: 3000,
    });
    expect(client.get).toHaveBeenCalledTimes(2);
  });

  it("URL state defaults to the replenish tab, page 1, 25 rows, last 30 days", () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date("2026-10-15T08:00:00Z"));
    const { result } = renderHook(() => useAtmProfileUrlState());
    vi.useRealTimers();

    expect(result.current.tab).toBe("replenish");
    expect(result.current.historyParams).toEqual({
      page: 1,
      page_size: 25,
      date_from: "2026-09-15",
      date_to: "2026-10-15",
    });
  });

  it("URL state reads the cashpos tab + params and merges updates into the URL", () => {
    nav.search = {
      tab: "cashpos",
      page: 3,
      page_size: 50,
      date_from: "2026-10-01",
      date_to: "2026-10-10",
    };
    const { result } = renderHook(() => useAtmProfileUrlState());

    expect(result.current.tab).toBe("cashpos");
    expect(result.current.historyParams).toEqual({
      page: 3,
      page_size: 50,
      date_from: "2026-10-01",
      date_to: "2026-10-10",
    });

    result.current.setTab("replenish");
    result.current.setHistoryParams({ page: 1 });
    const [[first], [second]] = nav.navigate.mock.calls as [
      { to: string; search: (p: object) => object },
    ][];
    expect(first.to).toBe(".");
    expect(first.search({ page: 3 })).toEqual({ page: 3, tab: "replenish" });
    expect(second.search({ tab: "cashpos", page: 3 })).toEqual({ tab: "cashpos", page: 1 });
  });
});
