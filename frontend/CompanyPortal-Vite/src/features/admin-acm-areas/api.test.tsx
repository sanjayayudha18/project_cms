/**
 * fe-coverage C2: Area ACM admin api (queries + one mutation dispatcher). Only the HTTP
 * client is mocked; the real TanStack hooks run. (Area ACM admin applies immediately —
 * GR#3 deviation, audit is server-side.)
 */

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const client = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), put: vi.fn() }));
vi.mock("@/lib/api/client", () => ({ api: client }));

import {
  type AcmAreaMutation,
  useAcmArea,
  useAcmAreaMutation,
  useAcmAreaWarnings,
  useAcmAreas,
  useAcmEligibleUsers,
  useAcmRegionOptions,
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

beforeEach(() => {
  client.get.mockReset();
  client.post.mockReset();
  client.put.mockReset();
});

describe("admin-acm-areas queries", () => {
  it.each([
    [
      "useAcmAreas",
      () => useAcmAreas(),
      "/admin/acm-areas?status=all",
      { areas: [{ id: 1 }] },
      [{ id: 1 }],
    ],
    [
      "useAcmRegionOptions",
      () => useAcmRegionOptions(),
      "/admin/acm-areas/region-options",
      { regions: [{ region_code: "JKT" }] },
      [{ region_code: "JKT" }],
    ],
    [
      "useAcmEligibleUsers",
      () => useAcmEligibleUsers(),
      "/admin/acm-areas/eligible-users",
      { users: [{ id: 5 }] },
      [{ id: 5 }],
    ],
    [
      "useAcmAreaWarnings",
      () => useAcmAreaWarnings(),
      "/admin/acm-areas/warnings",
      { requests_without_area: [{ vendor_request_id: 9 }] },
      [{ vendor_request_id: 9 }],
    ],
  ])("%s reads %s and unwraps the list", async (_n, hook, path, body, expected) => {
    client.get.mockResolvedValue({ data: body });
    const { result } = setup(hook as () => { isSuccess: boolean; data: unknown });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(client.get).toHaveBeenCalledWith(path);
    expect(result.current.data).toEqual(expected);
  });

  it("useAcmArea is idle until an area is selected", async () => {
    const idle = setup(() => useAcmArea(null));
    expect(idle.result.current.fetchStatus).toBe("idle");
    expect(client.get).not.toHaveBeenCalled();

    client.get.mockResolvedValue({ data: { id: 2, region_codes: [], members: [] } });
    const loaded = setup(() => useAcmArea(2));
    await waitFor(() => expect(loaded.result.current.isSuccess).toBe(true));
    expect(client.get).toHaveBeenCalledWith("/admin/acm-areas/2");
  });
});

describe("useAcmAreaMutation", () => {
  it.each<[string, AcmAreaMutation, [keyof typeof client, ...unknown[]]]>([
    [
      "create",
      { op: "create", name: "Area Barat" },
      ["post", "/admin/acm-areas", { name: "Area Barat" }],
    ],
    [
      "rename",
      { op: "rename", id: 2, name: "Area Timur" },
      ["put", "/admin/acm-areas/2", { name: "Area Timur" }],
    ],
    [
      "regions",
      { op: "regions", id: 2, region_codes: ["JKT", "BDG"] },
      ["put", "/admin/acm-areas/2/regions", { region_codes: ["JKT", "BDG"] }],
    ],
    [
      "members",
      { op: "members", id: 2, user_ids: [5, 6] },
      ["put", "/admin/acm-areas/2/members", { user_ids: [5, 6] }],
    ],
    ["disable", { op: "disable", id: 2 }, ["post", "/admin/acm-areas/2/disable"]],
    ["enable", { op: "enable", id: 2 }, ["post", "/admin/acm-areas/2/enable"]],
  ])("%s calls the right endpoint and refreshes areas", async (_n, mutation, [method, ...args]) => {
    client.post.mockResolvedValue({ data: {} });
    client.put.mockResolvedValue({ data: {} });
    const { result, invalidate } = setup(() => useAcmAreaMutation());

    await act(async () => {
      await result.current.mutateAsync(mutation);
    });

    expect(client[method]).toHaveBeenCalledWith(...args);
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ["admin-acm-areas"] });
  });

  it("a rejected change (e.g. region already in another area) is surfaced, nothing refreshed", async () => {
    client.put.mockRejectedValue({ status: 409, message: "Region sudah dipakai area lain" });
    const { result, invalidate } = setup(() => useAcmAreaMutation());

    await act(async () => {
      await result.current
        .mutateAsync({ op: "regions", id: 2, region_codes: ["JKT"] })
        .catch(() => {});
    });

    await waitFor(() =>
      expect(result.current.error?.message).toBe("Region sudah dipakai area lain"),
    );
    expect(invalidate).not.toHaveBeenCalled();
  });
});
