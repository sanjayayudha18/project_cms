/**
 * Bugfix 2026-10-19: vendorSide.ts must send paths relative to apiConfig.baseURL like every
 * other feature. Uses the real API client with `fetch` stubbed, so the URL on the wire is
 * what is asserted (it used to be /api/v1/api/v1/vendor-requests/... → 404).
 */

import { useAuthStore } from "@/lib/auth/store";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/api/config", () => ({ apiConfig: { baseURL: "/api/v1" } }));

import { vendorRequestKeys } from "../hooks";
import { useVendorParties, useVendorReturn } from "../vendorSide";

const mockFetch = vi.fn();

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

const json = (body: unknown) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });

beforeEach(() => {
  mockFetch.mockReset();
  vi.stubGlobal("fetch", mockFetch);
  useAuthStore.setState({ accessToken: "tok" });
});

afterEach(() => {
  vi.unstubAllGlobals();
  useAuthStore.setState({ accessToken: null });
});

describe("vendorSide requests", () => {
  it("loads the vendor parties from /api/v1/vendor-requests/{id}/vendor-parties", async () => {
    mockFetch.mockResolvedValue(json({ parties: [], events: [] }));
    const { result } = setup(() => useVendorParties(5));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    const [url, init] = mockFetch.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("/api/v1/vendor-requests/5/vendor-parties");
    expect((init.headers as Record<string, string>).Authorization).toBe("Bearer tok");
  });

  it("returns a request to the vendor at /api/v1/vendor-requests/{id}/vendor-return", async () => {
    mockFetch.mockResolvedValue(json({ id: 5, status: "rejected" }));
    const { result, invalidate } = setup(() => useVendorReturn());

    await act(async () => {
      await result.current.mutateAsync({ id: 5, reason: "Vendor salah cabang" });
    });

    const [url, init] = mockFetch.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("/api/v1/vendor-requests/5/vendor-return");
    expect(init.method).toBe("POST");
    expect(init.body).toBe(JSON.stringify({ rejection_reason: "Vendor salah cabang" }));
    expect(invalidate).toHaveBeenCalledWith({ queryKey: vendorRequestKeys.all });
  });
});
