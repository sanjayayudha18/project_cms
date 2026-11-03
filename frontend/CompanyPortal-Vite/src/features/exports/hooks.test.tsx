import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const apiMock = vi.hoisted(() => ({
  requestExport: vi.fn(),
  listMyExports: vi.fn(),
  getExportJob: vi.fn(),
  downloadExportFile: vi.fn(),
}));
vi.mock("./api", async (orig) => ({ ...(await orig<typeof import("./api")>()), ...apiMock }));

import { resetAutoDownloadClaims } from "./autoDownload";
import { useExportFlow, useMyExports } from "./hooks";
import { useBeforeUnloadGuard } from "./useBeforeUnloadGuard";

const mkJob = (status: string, extra = {}) => ({
  id: 5,
  status,
  file_name: "f.csv",
  row_count: 0,
  error_message: null,
  ...extra,
});

function wrapper() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={qc}>{children}</QueryClientProvider>
  );
}

beforeEach(() => {
  resetAutoDownloadClaims();
  for (const m of Object.values(apiMock)) m.mockReset();
});
afterEach(() => vi.restoreAllMocks());

describe("useBeforeUnloadGuard", () => {
  it("attaches only while active and prevents default", () => {
    const add = vi.spyOn(window, "addEventListener");
    const remove = vi.spyOn(window, "removeEventListener");
    const { rerender, unmount } = renderHook(({ a }) => useBeforeUnloadGuard(a), {
      initialProps: { a: false },
    });
    expect(add.mock.calls.some((c) => c[0] === "beforeunload")).toBe(false);
    rerender({ a: true });
    const call = add.mock.calls.find((c) => c[0] === "beforeunload");
    expect(call).toBeDefined();
    const ev = new Event("beforeunload", { cancelable: true });
    (call?.[1] as EventListener)(ev);
    expect(ev.defaultPrevented).toBe(true);
    unmount();
    expect(remove.mock.calls.some((c) => c[0] === "beforeunload")).toBe(true);
  });
});

describe("useMyExports polling", () => {
  it("polls while a job is active", async () => {
    apiMock.listMyExports.mockResolvedValue([mkJob("running")]);
    const { result } = renderHook(() => useMyExports(), { wrapper: wrapper() });
    await waitFor(() => expect(result.current.data).toHaveLength(1));
    const q = result.current;
    expect(q.isSuccess).toBe(true);
  });

  it("refetchInterval is 3000 with an active job and false otherwise", async () => {
    // Observe the option through the query cache.
    const qc = new QueryClient();
    const w = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={qc}>{children}</QueryClientProvider>
    );
    apiMock.listMyExports.mockResolvedValue([mkJob("queued")]);
    const { result } = renderHook(() => useMyExports(), { wrapper: w });
    await waitFor(() => expect(result.current.data).toBeDefined());
    const interval = () => {
      const opt = qc.getQueryCache().getAll()[0]?.observers[0]?.options.refetchInterval;
      return typeof opt === "function"
        ? (opt as (q: unknown) => unknown)(qc.getQueryCache().getAll()[0])
        : opt;
    };
    expect(interval()).toBe(3000);
    qc.setQueryData(qc.getQueryCache().getAll()[0].queryKey, [mkJob("downloaded")]);
    expect(interval()).toBe(false);
  });
});

describe("useExportFlow", () => {
  const params = { kind: "master_data_global", entity: "atms" } as const;

  it("goes waiting -> downloading -> done and auto-downloads exactly once", async () => {
    apiMock.requestExport.mockResolvedValue({ job: mkJob("queued"), created: true });
    apiMock.getExportJob.mockResolvedValue(mkJob("succeeded"));
    apiMock.downloadExportFile.mockResolvedValue(undefined);
    const { result } = renderHook(() => useExportFlow(), { wrapper: wrapper() });

    await act(async () => {
      await result.current.request(params);
    });
    expect(result.current.dialogProps.open).toBe(true);
    await waitFor(() => expect(result.current.dialogProps.phase).toBe("done"));
    expect(apiMock.downloadExportFile).toHaveBeenCalledTimes(1);
    expect(apiMock.downloadExportFile).toHaveBeenCalledWith(5, "f.csv");

    // Same job again (200 reuse) in the same tab: no second download.
    await act(async () => {
      await result.current.request(params);
    });
    await waitFor(() => expect(apiMock.getExportJob).toHaveBeenCalled());
    expect(apiMock.downloadExportFile).toHaveBeenCalledTimes(1);
  });

  it("shows failed with error_message and retries", async () => {
    apiMock.requestExport.mockResolvedValue({ job: mkJob("queued"), created: true });
    apiMock.getExportJob.mockResolvedValue(mkJob("failed", { error_message: "boom" }));
    const { result } = renderHook(() => useExportFlow(), { wrapper: wrapper() });
    await act(async () => {
      await result.current.request(params);
    });
    await waitFor(() => expect(result.current.dialogProps.phase).toBe("failed"));
    expect(result.current.dialogProps.message).toBe("boom");
    act(() => result.current.dialogProps.onRetry());
    await waitFor(() => expect(apiMock.requestExport).toHaveBeenCalledTimes(2));
  });

  it("download failure -> download_failed", async () => {
    apiMock.requestExport.mockResolvedValue({ job: mkJob("queued"), created: true });
    apiMock.getExportJob.mockResolvedValue(mkJob("succeeded"));
    apiMock.downloadExportFile.mockRejectedValue({ status: 410, message: "Sudah dihapus" });
    const { result } = renderHook(() => useExportFlow(), { wrapper: wrapper() });
    await act(async () => {
      await result.current.request(params);
    });
    await waitFor(() => expect(result.current.dialogProps.phase).toBe("download_failed"));
    expect(result.current.dialogProps.message).toContain("Sudah dihapus");
  });

  it("report variant is passed through to the request and follows the same flow", async () => {
    const report = {
      kind: "report",
      report: "dsr-late-daily",
      params: { month: "2026-10", status: "late" },
    } as const;
    apiMock.requestExport.mockResolvedValue({
      job: mkJob("queued", { kind: "dsr_late_daily" }),
      created: true,
    });
    apiMock.getExportJob.mockResolvedValue(mkJob("succeeded", { kind: "dsr_late_daily" }));
    apiMock.downloadExportFile.mockResolvedValue(undefined);
    const { result } = renderHook(() => useExportFlow(), { wrapper: wrapper() });
    await act(async () => {
      await result.current.request(report);
    });
    expect(apiMock.requestExport.mock.lastCall?.[0]).toEqual(report);
    await waitFor(() => expect(result.current.dialogProps.phase).toBe("done"));
    expect(apiMock.downloadExportFile).toHaveBeenCalledTimes(1);
  });

  it("429 does not open the dialog and exposes the error", async () => {
    const err = { status: 429, code: "too_many_exports", message: "Terlalu banyak" };
    apiMock.requestExport.mockRejectedValue(err);
    const { result } = renderHook(() => useExportFlow(), { wrapper: wrapper() });
    let r: unknown;
    await act(async () => {
      r = await result.current.request(params);
    });
    expect(r).toEqual({ error: err });
    expect(result.current.requestError).toEqual(err);
    expect(result.current.dialogProps.open).toBe(false);
    expect(apiMock.getExportJob).not.toHaveBeenCalled();
  });
});
