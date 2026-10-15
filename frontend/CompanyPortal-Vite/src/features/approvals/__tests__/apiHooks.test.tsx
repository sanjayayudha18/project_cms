/**
 * fe-coverage C2: approvals api + hooks. Only the HTTP client is mocked, so the real
 * fetchers (paths) and the real TanStack hooks (enabled flags, invalidation) run.
 */

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const client = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }));
vi.mock("@/lib/api/client", () => ({ api: client }));

import { approveRequest, getInbox, getMasterDataDetail, rejectRequest } from "../api";
import { approvalKeys, useApprove, useInbox, useMasterDataDetail, useReject } from "../hooks";

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
});

describe("approvals api", () => {
  it("reads the inbox and the master-data detail of a request", async () => {
    client.get.mockResolvedValueOnce({ data: [{ step_id: 1 }] });
    client.get.mockResolvedValueOnce({ data: { request_id: 9, changes: [] } });

    await expect(getInbox()).resolves.toEqual([{ step_id: 1 }]);
    await expect(getMasterDataDetail(9)).resolves.toEqual({ request_id: 9, changes: [] });

    expect(client.get.mock.calls.map((c) => c[0])).toEqual([
      "/approvals/inbox",
      "/approvals/9/master-data",
    ]);
  });

  it("approve and reject post to the request's decision endpoint", async () => {
    client.post.mockResolvedValue({ data: undefined });

    await approveRequest(5);
    await rejectRequest(6);

    expect(client.post.mock.calls.map((c) => c[0])).toEqual([
      "/approvals/5/approve",
      "/approvals/6/reject",
    ]);
  });
});

describe("approvals hooks", () => {
  it("useInbox returns the inbox items", async () => {
    client.get.mockResolvedValue({ data: [{ step_id: 3, request_id: 30 }] });
    const { result } = setup(() => useInbox());

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual([{ step_id: 3, request_id: 30 }]);
  });

  it("useMasterDataDetail stays idle without a request id", () => {
    const { result } = setup(() => useMasterDataDetail(null));

    expect(result.current.fetchStatus).toBe("idle");
    expect(client.get).not.toHaveBeenCalled();
  });

  it("useMasterDataDetail loads the selected request", async () => {
    client.get.mockResolvedValue({ data: { request_id: 4, changes: [] } });
    const { result } = setup(() => useMasterDataDetail(4));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(client.get).toHaveBeenCalledWith("/approvals/4/master-data");
  });

  it("useApprove refreshes approvals, vendors and ATMs after success", async () => {
    client.post.mockResolvedValue({ data: undefined });
    const { result, invalidate } = setup(() => useApprove());

    await act(async () => {
      await result.current.mutateAsync(12);
    });

    expect(invalidatedKeys(invalidate)).toEqual([
      approvalKeys.all,
      ["admin-vendors"],
      ["admin-atms"],
    ]);
  });

  it("useApprove also refreshes after a failed approve (e.g. 409 stale), so the inbox is current", async () => {
    client.post.mockRejectedValue({ status: 409, message: "stale" });
    const { result, invalidate } = setup(() => useApprove());

    await act(async () => {
      await result.current.mutateAsync(12).catch(() => {});
    });

    await waitFor(() => expect(result.current.error).toEqual({ status: 409, message: "stale" }));
    expect(invalidatedKeys(invalidate)).toContainEqual(approvalKeys.all);
  });

  it("useReject refreshes after success but not after failure", async () => {
    client.post.mockResolvedValueOnce({ data: undefined });
    const ok = setup(() => useReject());
    await act(async () => {
      await ok.result.current.mutateAsync(7);
    });
    expect(invalidatedKeys(ok.invalidate)).toContainEqual(approvalKeys.all);

    client.post.mockRejectedValueOnce({ status: 403, message: "maker" });
    const failed = setup(() => useReject());
    await act(async () => {
      await failed.result.current.mutateAsync(7).catch(() => {});
    });
    expect(failed.invalidate).not.toHaveBeenCalled();
  });
});
