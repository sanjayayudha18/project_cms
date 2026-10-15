/**
 * fe-coverage C4: vault-plan api (pemenuhan vault ACM v3) + money formatters.
 * Only the HTTP client is mocked; the real TanStack hooks run.
 */

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const client = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), put: vi.fn() }));
vi.mock("@/lib/api/client", () => ({ api: client }));

import {
  formatAmount,
  formatDenoms,
  useRequestVaultDecision,
  useRequestVaultReview,
  useVaultCandidates,
  useVaultPlan,
  useVaultPlanMutation,
  useVaultPlans,
} from "./api";

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

beforeEach(() => {
  client.get.mockReset();
  client.post.mockReset();
  client.put.mockReset();
});

describe("vault-plan queries", () => {
  it("list sends only the filters that are set and unwraps plans", async () => {
    client.get.mockResolvedValue({ data: { plans: [{ id: 1 }] } });
    const { result } = setup(() => useVaultPlans({ status: "draft", from: "", to: "2026-10-31" }));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(client.get).toHaveBeenCalledWith("/vault-plans?status=draft&to=2026-10-31");
    expect(result.current.data).toEqual([{ id: 1 }]);
  });

  it("detail is idle without an id and loads with one", async () => {
    const idle = setup(() => useVaultPlan(null));
    expect(idle.result.current.fetchStatus).toBe("idle");

    client.get.mockResolvedValue({ data: { id: 4 } });
    const loaded = setup(() => useVaultPlan(4));
    await waitFor(() => expect(loaded.result.current.isSuccess).toBe(true));
    expect(client.get).toHaveBeenCalledWith("/vault-plans/4");
  });

  it("candidates always ask for every tier (urgent=true), only when enabled", async () => {
    const off = setup(() => useVaultCandidates(4, false));
    expect(off.result.current.fetchStatus).toBe("idle");

    client.get.mockResolvedValue({ data: { branches: [{ branch_id: 3, tier: 1 }] } });
    const on = setup(() => useVaultCandidates(4, true));
    await waitFor(() => expect(on.result.current.isSuccess).toBe(true));
    expect(client.get).toHaveBeenCalledWith("/vault-plans/4/candidates?urgent=true");
    expect(on.result.current.data).toEqual([{ branch_id: 3, tier: 1 }]);
  });

  it("ATM-SPV review returns the request's live plan, or null outside the vault flow", async () => {
    client.get.mockResolvedValueOnce({ data: { plan: { id: 4 } } });
    const withPlan = setup(() => useRequestVaultReview(9, true));
    await waitFor(() => expect(withPlan.result.current.isSuccess).toBe(true));
    expect(client.get).toHaveBeenCalledWith("/vendor-requests/9/vault-assignments");
    expect(withPlan.result.current.data).toEqual({ id: 4 });

    client.get.mockResolvedValueOnce({ data: { plan: null } });
    const legacy = setup(() => useRequestVaultReview(10, true));
    await waitFor(() => expect(legacy.result.current.isSuccess).toBe(true));
    expect(legacy.result.current.data).toBeNull();
  });
});

describe("vault-plan mutations", () => {
  const allocations = [
    {
      vendor_vault_id: 5,
      amounts: { "100000": "500000000", "50000": "0" },
      is_urgent: false,
      urgent_reason: null,
    },
  ];

  it.each<[string, unknown, ["post" | "put", ...unknown[]]]>([
    [
      "save (allocations + buffer reason)",
      { op: "save", id: 4, allocations, bufferReason: "Cadangan libur panjang" },
      [
        "put",
        "/vault-plans/4/allocations",
        { allocations, buffer_reason: "Cadangan libur panjang" },
      ],
    ],
    ["submit", { op: "submit", id: 4 }, ["post", "/vault-plans/4/submit"]],
    ["approve", { op: "approve", id: 4 }, ["post", "/vault-plans/4/approve"]],
    [
      "reject",
      { op: "reject", id: 4, reason: "Saldo vault kurang" },
      ["post", "/vault-plans/4/reject", { rejection_reason: "Saldo vault kurang" }],
    ],
  ])("%s → endpoint, then plans refresh", async (_n, mutation, [method, ...args]) => {
    client.post.mockResolvedValue({ data: {} });
    client.put.mockResolvedValue({ data: {} });
    const { result, invalidate } = setup(() => useVaultPlanMutation());

    await act(async () => {
      await result.current.mutateAsync(mutation as never);
    });

    expect(client[method]).toHaveBeenCalledWith(...args);
    expect(invalidatedKeys(invalidate)).toEqual([["vault-plans"]]);
  });

  it("ATM-SPV vault approve/reject refreshes plans and vendor requests", async () => {
    client.post.mockResolvedValue({ data: {} });
    const { result, invalidate } = setup(() => useRequestVaultDecision());

    await act(async () => {
      await result.current.mutateAsync({ id: 9, approve: true });
      await result.current.mutateAsync({ id: 9, approve: false, reason: "Vault salah pilih" });
    });

    expect(client.post.mock.calls).toEqual([
      ["/vendor-requests/9/vault-approve"],
      ["/vendor-requests/9/vault-reject", { vault_rejection_reason: "Vault salah pilih" }],
    ]);
    expect(invalidatedKeys(invalidate)).toEqual([
      ["vault-plans"],
      ["vendor-requests"],
      ["vault-plans"],
      ["vendor-requests"],
    ]);
  });

  it("a rejected save (e.g. amount not a multiple of the denom) refreshes nothing", async () => {
    client.put.mockRejectedValue({ status: 422, message: "Bukan kelipatan pecahan" });
    const { result, invalidate } = setup(() => useVaultPlanMutation());

    await act(async () => {
      await result.current
        .mutateAsync({ op: "save", id: 4, allocations, bufferReason: null })
        .catch(() => {});
    });

    expect(invalidate).not.toHaveBeenCalled();
  });
});

describe("money formatting", () => {
  it.each([
    [undefined, "-"],
    ["0", "0"],
    ["1500000", "1.500.000"],
    ["9007199254740991", "9.007.199.254.740.991"],
  ])("formatAmount(%j) = %s", (value, expected) => {
    expect(formatAmount(value)).toBe(expected);
  });

  it("formatDenoms lists denoms high to low and drops zero entries", () => {
    expect(formatDenoms({ "50000": "300000", "100000": "1500000", "20000": "0" } as never)).toBe(
      "100K 1.500.000 · 50K 300.000",
    );
    expect(formatDenoms({ "100000": "0" } as never)).toBe("0");
    expect(formatDenoms(null)).toBe("Tidak diketahui");
  });
});
