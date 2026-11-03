import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const client = vi.hoisted(() => ({ get: vi.fn() }));
vi.mock("@/lib/api/client", () => ({ api: client }));

import { useDsrLateDaily, useDsrLateSummary, useDsrLateWarnings } from "../api";

function setup<T>(hook: () => T) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={qc}>{children}</QueryClientProvider>
  );
  return renderHook(hook, { wrapper });
}

beforeEach(() => client.get.mockReset());

describe("dsr-late-report queries", () => {
  it("daily sends month, filters and paging as query params", async () => {
    client.get.mockResolvedValue({ data: { items: [] } });
    const { result } = setup(() =>
      useDsrLateDaily({ month: "2026-10", vendorId: 2, vendorBranchId: 5, status: "late" }, 3),
    );
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    const url = client.get.mock.calls[0]?.[0] as string;
    expect(url.startsWith("/dsr-late-reports/daily?")).toBe(true);
    const q = new URLSearchParams(url.split("?")[1]);
    expect(Object.fromEntries(q)).toEqual({
      month: "2026-10",
      vendor_id: "2",
      vendor_branch_id: "5",
      status: "late",
      page: "3",
      page_size: "20",
    });
  });

  it("summary and warnings omit unset filters", async () => {
    client.get.mockResolvedValue({ data: {} });
    const s = setup(() =>
      useDsrLateSummary({ month: "2026-10", vendorId: null, vendorBranchId: null }),
    );
    const w = setup(() => useDsrLateWarnings("2026-10", 4));
    await waitFor(() =>
      expect(s.result.current.isSuccess && w.result.current.isSuccess).toBe(true),
    );
    expect(client.get).toHaveBeenCalledWith("/dsr-late-reports/summary?month=2026-10");
    expect(client.get).toHaveBeenCalledWith("/dsr-late-reports/warnings?month=2026-10&vendor_id=4");
  });
});
