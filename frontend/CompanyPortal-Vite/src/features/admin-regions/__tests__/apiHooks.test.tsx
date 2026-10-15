/**
 * fe-coverage C2: admin-regions api + hooks. Only the HTTP client is mocked, so the real
 * fetchers (paths, query string) and the real TanStack hooks (enabled, invalidation) run.
 */

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const client = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), put: vi.fn() }));
vi.mock("@/lib/api/client", () => ({ api: client }));

import {
  createRegion,
  disableRegion,
  enableRegion,
  getRegion,
  listRegions,
  updateRegionName,
} from "../api";
import {
  adminRegionsKeys,
  useCreateRegion,
  useDisableRegion,
  useEnableRegion,
  useRegion,
  useRegionsList,
  useUpdateRegion,
} from "../hooks";
import type { CreateRegionPayload, UpdateRegionPayload } from "../types";

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

type Mutation = { mutateAsync: (v: unknown) => Promise<unknown> };

const createPayload = { code: "JKT", name: "Jakarta" } as unknown as CreateRegionPayload;
const updatePayload = { name: "Jakarta Raya" } as unknown as UpdateRegionPayload;

beforeEach(() => {
  client.get.mockReset();
  client.post.mockReset();
  client.put.mockReset();
});

describe("admin-regions api", () => {
  it("listRegions always sends page + page_size and only the filters that are set", async () => {
    client.get.mockResolvedValue({ data: { regions: [], total: 0 } });

    await listRegions({ page: 1, page_size: 25 });
    await listRegions({ page: 3, page_size: 50, status: "active", q: "jak" } as never);

    expect(client.get.mock.calls.map((c) => c[0])).toEqual([
      "/admin/regions?page=1&page_size=25",
      "/admin/regions?page=3&page_size=50&status=active&q=jak",
    ]);
  });

  it("get / create / rename / disable / enable hit the region endpoints", async () => {
    client.get.mockResolvedValue({ data: { id: 4 } });
    client.post.mockResolvedValue({ data: { id: 4 } });
    client.put.mockResolvedValue({ data: { id: 4 } });

    await getRegion(4);
    await createRegion(createPayload);
    await updateRegionName(4, updatePayload);
    await disableRegion(4);
    await enableRegion(4);

    expect(client.get).toHaveBeenCalledWith("/admin/regions/4");
    expect(client.put).toHaveBeenCalledWith("/admin/regions/4", updatePayload);
    expect(client.post.mock.calls).toEqual([
      ["/admin/regions", createPayload],
      ["/admin/regions/4/disable"],
      ["/admin/regions/4/enable"],
    ]);
  });
});

describe("admin-regions hooks", () => {
  it("useRegionsList loads the requested page", async () => {
    client.get.mockResolvedValue({ data: { regions: [{ id: 1 }], total: 1 } });
    const { result } = setup(() => useRegionsList({ page: 1, page_size: 25 }));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual({ regions: [{ id: 1 }], total: 1 });
  });

  it("useRegion is idle without an id and loads with one", async () => {
    const idle = setup(() => useRegion(null));
    expect(idle.result.current.fetchStatus).toBe("idle");

    client.get.mockResolvedValue({ data: { id: 6 } });
    const loaded = setup(() => useRegion(6));
    await waitFor(() => expect(loaded.result.current.isSuccess).toBe(true));
    expect(client.get).toHaveBeenCalledWith("/admin/regions/6");
  });

  it.each([
    ["useCreateRegion", () => useCreateRegion(), createPayload],
    ["useUpdateRegion", () => useUpdateRegion(), { id: 4, payload: updatePayload }],
    ["useDisableRegion", () => useDisableRegion(), 4],
    ["useEnableRegion", () => useEnableRegion(), 4],
  ])("%s refreshes the regions list after success", async (_name, hook, variables) => {
    client.post.mockResolvedValue({ data: { id: 4 } });
    client.put.mockResolvedValue({ data: { id: 4 } });
    const { result, invalidate } = setup(hook as () => Mutation);

    await act(async () => {
      await result.current.mutateAsync(variables);
    });

    expect(invalidate).toHaveBeenCalledWith({ queryKey: adminRegionsKeys.all });
  });

  it("disable blocked by an active location (409) keeps the list and surfaces the message", async () => {
    client.post.mockRejectedValue({ status: 409, message: "Region masih dipakai lokasi aktif" });
    const { result, invalidate } = setup(() => useDisableRegion());

    await act(async () => {
      await result.current.mutateAsync(4).catch(() => {});
    });

    await waitFor(() =>
      expect(result.current.error?.message).toBe("Region masih dipakai lokasi aktif"),
    );
    expect(invalidate).not.toHaveBeenCalled();
  });
});
