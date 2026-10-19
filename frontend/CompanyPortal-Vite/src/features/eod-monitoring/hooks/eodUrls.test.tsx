/**
 * Bugfix 2026-10-19: EOD Monitoring talks to the Python eod_retry_scheduler, which the Vite
 * proxy and nginx.conf route under /api/eod/* (not /api/v1). Uses the real API client with
 * `fetch` stubbed, so the URL on the wire is asserted (it used to be /api/v1/api/eod/... →
 * the Go API → 404). The Bearer token must still be sent (RETRY_AUTH_MODE=jwt).
 */

import { useAuthStore } from "@/lib/auth/store";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/api/config", () => ({ apiConfig: { baseURL: "/api/v1" } }));

import { useEodAudit, useEodSummary, useFileHistory, useRetryFile } from "./useEodQueries";

const mockFetch = vi.fn();

function setup<T>(hook: () => T) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  return renderHook(hook, { wrapper });
}

const envelope = (data: unknown) =>
  new Response(JSON.stringify({ status: "success", data, error: null }), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });

beforeEach(() => {
  mockFetch.mockReset();
  mockFetch.mockImplementation(async () => envelope({ ok: true }));
  vi.stubGlobal("fetch", mockFetch);
  useAuthStore.setState({ accessToken: "admin-jwt" });
});

afterEach(() => {
  vi.unstubAllGlobals();
  useAuthStore.setState({ accessToken: null });
});

const sent = () => mockFetch.mock.calls.map(([url, init]) => [url, init as RequestInit] as const);

describe("EOD requests go to /api/eod/* with the user's token", () => {
  it.each<[string, () => { isSuccess: boolean }, string]>([
    [
      "summary",
      () => useEodSummary("2026-10-15", false),
      "/api/eod/summary?processing_date=2026-10-15",
    ],
    ["file history", () => useFileHistory("f-1"), "/api/eod/status/f-1/history"],
    [
      "audit",
      () => useEodAudit("2026-10-15", "dmaa", null, false),
      "/api/eod/audit?processing_date=2026-10-15&file_type=dmaa",
    ],
  ])("%s", async (_n, hook, url) => {
    const { result } = setup(hook);

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    const [[sentUrl, init]] = sent();
    expect(sentUrl).toBe(url);
    expect((init.headers as Record<string, string>).Authorization).toBe("Bearer admin-jwt");
  });

  it("manual retry", async () => {
    const { result } = setup(() => useRetryFile());

    await act(async () => {
      await result.current.mutateAsync("f-1");
    });

    const [[sentUrl, init]] = sent();
    expect(sentUrl).toBe("/api/eod/retry/f-1");
    expect(init.method).toBe("POST");
  });
});
