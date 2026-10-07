import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const getMock = vi.fn();
const postMock = vi.fn();
vi.mock("@/lib/api/client", () => ({
  api: {
    get: (...args: unknown[]) => getMock(...args),
    post: (...args: unknown[]) => postMock(...args),
  },
}));

import {
  useMarkAllAsRead,
  useMarkAsRead,
  useNotifications,
  useUnreadCount,
} from "../useNotifications";

const items = [
  {
    id: 1,
    type: "t",
    title: "Satu",
    body: "",
    link: null,
    is_read: false,
    created_at: "2026-10-07T02:00:00Z",
  },
  {
    id: 2,
    type: "t",
    title: "Dua",
    body: "",
    link: null,
    is_read: false,
    created_at: "2026-10-06T02:00:00Z",
  },
];

function wrapper({ children }: { children: ReactNode }) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}

function useAll() {
  return {
    list: useNotifications(),
    unread: useUnreadCount(),
    markOne: useMarkAsRead(),
    markAll: useMarkAllAsRead(),
  };
}

describe("VendorPortal notifications hooks", () => {
  beforeEach(() => {
    getMock.mockReset();
    postMock.mockReset();
    getMock.mockImplementation(async (path: string) =>
      path.endsWith("/unread-count")
        ? { data: { unread_count: 2 } }
        : { data: { items, total: 2, page: 1, page_size: 100 } },
    );
    postMock.mockResolvedValue({ data: { updated: 1 } });
  });

  it("loads from the real API instead of the JSON mock", async () => {
    const { result } = renderHook(useAll, { wrapper });
    await waitFor(() => expect(result.current.list.data).toHaveLength(2));
    expect(getMock).toHaveBeenCalledWith("/api/v1/notifications?page=1&page_size=100");
    await waitFor(() => expect(result.current.unread.data).toBe(2));
    expect(getMock).toHaveBeenCalledWith("/api/v1/notifications/unread-count");
  });

  it("mark one read updates the cache without refetching", async () => {
    const { result } = renderHook(useAll, { wrapper });
    await waitFor(() => expect(result.current.unread.data).toBe(2));
    const calls = getMock.mock.calls.length;

    await act(() => result.current.markOne.mutateAsync(1));
    expect(postMock).toHaveBeenCalledWith("/api/v1/notifications/1/read");
    await waitFor(() => expect(result.current.unread.data).toBe(1));
    expect(result.current.list.data?.find((n) => n.id === 1)?.is_read).toBe(true);
    expect(getMock.mock.calls.length).toBe(calls);
  });

  it("mark all read zeroes the count", async () => {
    const { result } = renderHook(useAll, { wrapper });
    await waitFor(() => expect(result.current.unread.data).toBe(2));

    await act(() => result.current.markAll.mutateAsync());
    expect(postMock).toHaveBeenCalledWith("/api/v1/notifications/read-all");
    await waitFor(() => expect(result.current.unread.data).toBe(0));
    expect(result.current.list.data?.every((n) => n.is_read)).toBe(true);
  });
});
