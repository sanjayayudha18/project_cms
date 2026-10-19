/**
 * fe-coverage C5: EOD monitoring queries (Python eod_retry_scheduler via /api/eod/*).
 * The URL on the wire (/api/eod/*, no /api/v1 prefix) is asserted in eodUrls.test.tsx.
 */

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const client = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }));
vi.mock("@/lib/api/client", () => ({ api: client }));

import {
  eodKeys,
  getErrorMessage,
  useEodAudit,
  useEodLate,
  useEodStatus,
  useEodSummary,
  useFileHistory,
  useRetryFile,
} from "./useEodQueries";

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

const ok = <T,>(data: T) => ({ data: { status: "success", data, error: null } });
const endsWith = (suffix: string) =>
  expect.stringMatching(new RegExp(`${suffix.replace(/[?]/g, "\\?")}$`));

type Query = {
  isSuccess: boolean;
  isError: boolean;
  data: unknown;
  error: unknown;
  fetchStatus: string;
};
const q = (hook: () => unknown) => hook as () => Query;

beforeEach(() => {
  client.get.mockReset();
  client.post.mockReset();
});

describe("EOD queries", () => {
  it.each<[string, () => Query, string]>([
    [
      "summary",
      q(() => useEodSummary("2026-10-15", false)),
      "/eod/summary?processing_date=2026-10-15",
    ],
    [
      "file status",
      q(() => useEodStatus("2026-10-15", false)),
      "/eod/status?processing_date=2026-10-15",
    ],
    [
      "late detections",
      q(() => useEodLate("2026-10-15", false)),
      "/eod/late?processing_date=2026-10-15",
    ],
    ["file history", q(() => useFileHistory("f-1")), "/eod/status/f-1/history"],
    [
      "audit (no filter)",
      q(() => useEodAudit("2026-10-15", null, null, false)),
      "/eod/audit?processing_date=2026-10-15",
    ],
    [
      "audit (file type + trigger)",
      q(() => useEodAudit("2026-10-15", "dmaa", "manual", false)),
      "/eod/audit?processing_date=2026-10-15&file_type=dmaa&trigger=manual",
    ],
  ])("%s unwraps the success envelope", async (_n, hook, suffix) => {
    client.get.mockResolvedValue(ok({ value: 1 }));
    const { result } = setup(hook);

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(client.get).toHaveBeenCalledWith(endsWith(suffix), { baseURL: "" });
    expect(result.current.data).toEqual({ value: 1 });
  });

  it("an error envelope becomes the query error", async () => {
    client.get.mockResolvedValue({
      data: { status: "error", data: null, error: "Scheduler down" },
    });
    const { result } = setup(() => useEodSummary("2026-10-15", false));

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect((result.current.error as Error).message).toBe("Scheduler down");
  });

  it("a success envelope without data is treated as an error too", async () => {
    client.get.mockResolvedValue({ data: { status: "success", data: null, error: null } });
    const { result } = setup(() => useEodStatus("2026-10-15", false));

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect((result.current.error as Error).message).toBe("Unknown error");
  });

  it("file history waits for a selected file", () => {
    const { result } = setup(() => useFileHistory(null));

    expect(result.current.fetchStatus).toBe("idle");
    expect(client.get).not.toHaveBeenCalled();
  });
});

describe("manual retry", () => {
  it("posts the retry and refreshes every EOD query", async () => {
    client.post.mockResolvedValue(ok({ file_id: "f-1", status: "processing" }));
    const { result, invalidate } = setup(() => useRetryFile());

    let returned: unknown;
    await act(async () => {
      returned = await result.current.mutateAsync("f-1");
    });

    expect(client.post).toHaveBeenCalledWith(endsWith("/eod/retry/f-1"), undefined, {
      baseURL: "",
    });
    expect(returned).toEqual({ file_id: "f-1", status: "processing" });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: eodKeys.all });
  });

  it("a refused retry (error envelope) is an error and refreshes nothing", async () => {
    client.post.mockResolvedValue({
      data: { status: "error", data: null, error: "Sudah diproses" },
    });
    const { result, invalidate } = setup(() => useRetryFile());

    await act(async () => {
      await result.current.mutateAsync("f-1").catch(() => {});
    });

    expect(invalidate).not.toHaveBeenCalled();
  });
});

describe("getErrorMessage", () => {
  it.each([
    [new Error("boom"), "boom"],
    [{ status: 502, message: "Bad Gateway" }, "Bad Gateway"],
    [{ message: 1 }, "Terjadi kesalahan yang tidak diketahui"],
    [undefined, "Terjadi kesalahan yang tidak diketahui"],
  ])("%j → %s", (err, expected) => {
    expect(getErrorMessage(err)).toBe(expected);
  });
});
