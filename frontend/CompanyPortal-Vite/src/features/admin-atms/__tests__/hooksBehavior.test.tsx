/**
 * Behavioral tests for the admin-atms TanStack Query hooks -- real hooks,
 * real QueryClient, only ../api mocked so no network runs. Complements
 * __tests__/hooks.test.ts (query-key equality/uniqueness). Mirrors the
 * renderHook pattern in rbac-settings/__tests__/hooks.test.tsx and
 * cash-flow/__tests__/useCashFlowData.test.ts.
 */

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mockApi = vi.hoisted(() => ({
  listATMs: vi.fn(),
  getATM: vi.fn(),
  createATM: vi.fn(),
  updateATM: vi.fn(),
  disableATM: vi.fn(),
  enableATM: vi.fn(),
  listLocationOptions: vi.fn(),
}));

vi.mock("../api", () => mockApi);

import {
  useATM,
  useATMsList,
  useCreateATM,
  useDisableATM,
  useEnableATM,
  useLocationOptions,
  useUpdateATM,
} from "../hooks";

function createWrapper(queryClient: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
  };
}

function newClient() {
  return new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
}

beforeEach(() => {
  for (const fn of Object.values(mockApi)) fn.mockReset();
});

describe("admin-atms hooks — reads", () => {
  it("useATMsList fetches and returns the list response", async () => {
    mockApi.listATMs.mockResolvedValue({ atms: [{ id: 1 }], page: 1, page_size: 25, total: 1 });
    const { result } = renderHook(() => useATMsList({ page: 1, page_size: 25 }), {
      wrapper: createWrapper(newClient()),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockApi.listATMs).toHaveBeenCalledWith({ page: 1, page_size: 25 });
    expect(result.current.data?.total).toBe(1);
  });

  it("useATM is disabled when id is null, enabled once an id is provided", async () => {
    mockApi.getATM.mockResolvedValue({ id: 5 });
    const { result, rerender } = renderHook(({ id }: { id: number | null }) => useATM(id), {
      wrapper: createWrapper(newClient()),
      initialProps: { id: null },
    });

    expect(result.current.fetchStatus).toBe("idle");
    expect(mockApi.getATM).not.toHaveBeenCalled();

    rerender({ id: 5 });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockApi.getATM).toHaveBeenCalledWith(5);
  });

  it("useLocationOptions fetches the location list", async () => {
    mockApi.listLocationOptions.mockResolvedValue({ locations: [{ id: 10, name: "Jakarta" }] });
    const { result } = renderHook(() => useLocationOptions(), {
      wrapper: createWrapper(newClient()),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.locations).toHaveLength(1);
  });
});

describe("admin-atms hooks — mutations invalidate the list on success", () => {
  it("useCreateATM invalidates adminATMsKeys.all", async () => {
    mockApi.createATM.mockResolvedValue({ id: 1 });
    const queryClient = newClient();
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useCreateATM(), { wrapper: createWrapper(queryClient) });

    result.current.mutate({ terminal_id: "TATM001" } as never);
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["admin-atms"] });
  });

  it("useUpdateATM invalidates adminATMsKeys.all", async () => {
    mockApi.updateATM.mockResolvedValue({ id: 1 });
    const queryClient = newClient();
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useUpdateATM(), { wrapper: createWrapper(queryClient) });

    result.current.mutate({ id: 1, payload: {} as never });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockApi.updateATM).toHaveBeenCalledWith(1, {});
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["admin-atms"] });
  });

  it("useDisableATM invalidates adminATMsKeys.all", async () => {
    mockApi.disableATM.mockResolvedValue({ message: "ok" });
    const queryClient = newClient();
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useDisableATM(), { wrapper: createWrapper(queryClient) });

    result.current.mutate(1);
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["admin-atms"] });
  });

  it("useEnableATM invalidates adminATMsKeys.all", async () => {
    mockApi.enableATM.mockResolvedValue({ message: "ok" });
    const queryClient = newClient();
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useEnableATM(), { wrapper: createWrapper(queryClient) });

    result.current.mutate(1);
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["admin-atms"] });
  });
});
