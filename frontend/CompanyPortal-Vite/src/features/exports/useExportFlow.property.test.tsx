import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import fc from "fast-check";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const apiMock = vi.hoisted(() => ({
  requestExport: vi.fn(),
  listMyExports: vi.fn(),
  getExportJob: vi.fn(),
  downloadExportFile: vi.fn(),
}));
vi.mock("./api", async (orig) => ({ ...(await orig<typeof import("./api")>()), ...apiMock }));

import { resetAutoDownloadClaims } from "./autoDownload";
import { exportKeys, useExportFlow } from "./hooks";

const STATUSES = ["queued", "running", "succeeded", "failed", "expired"] as const;
type Op =
  | { t: "poll"; s: (typeof STATUSES)[number] }
  | { t: "close" }
  | { t: "reopen" }
  | { t: "rerender" };

const arbOp: fc.Arbitrary<Op> = fc.oneof(
  { weight: 5, arbitrary: fc.constantFrom(...STATUSES).map((s) => ({ t: "poll", s }) as Op) },
  { weight: 1, arbitrary: fc.constant<Op>({ t: "close" }) },
  { weight: 1, arbitrary: fc.constant<Op>({ t: "reopen" }) },
  { weight: 1, arbitrary: fc.constant<Op>({ t: "rerender" }) },
);

const mkJob = (status: string) => ({
  id: 7,
  status,
  file_name: "f.csv",
  row_count: 0,
  error_message: null,
});
const params = { kind: "master_data_global", entity: "atms" } as const;

beforeEach(() => {
  resetAutoDownloadClaims();
  for (const m of Object.values(apiMock)) m.mockReset();
});

// Feature: vendor-data-export, Property 18: Unduh otomatis paling banyak sekali
describe("useExportFlow auto-download property", () => {
  it("downloads at most once per job, and only after a succeeded status was observed", async () => {
    await fc.assert(
      fc.asyncProperty(fc.array(arbOp, { minLength: 1, maxLength: 12 }), async (ops) => {
        resetAutoDownloadClaims();
        for (const m of Object.values(apiMock)) m.mockReset();
        apiMock.requestExport.mockResolvedValue({ job: mkJob("queued"), created: true });
        apiMock.downloadExportFile.mockResolvedValue(undefined);
        let next = "queued";
        let seenSucceeded = false;
        apiMock.getExportJob.mockImplementation(async () => {
          if (next === "succeeded") seenSucceeded = true;
          return mkJob(next);
        });

        const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
        const wrapper = ({ children }: { children: ReactNode }) => (
          <QueryClientProvider client={qc}>{children}</QueryClientProvider>
        );
        const { result, rerender, unmount } = renderHook(() => useExportFlow(), { wrapper });
        await act(async () => {
          await result.current.request(params);
        });

        for (const op of ops) {
          if (op.t === "poll") {
            next = op.s;
            await act(async () => {
              await qc.refetchQueries({ queryKey: exportKeys.job(7) });
            });
          } else if (op.t === "close") {
            act(() => result.current.dialogProps.onClose());
          } else if (op.t === "reopen") {
            await act(async () => {
              await result.current.request(params);
            });
          } else {
            rerender();
          }
          await act(async () => {});
          expect(apiMock.downloadExportFile.mock.calls.length).toBeLessThanOrEqual(1);
          if (!seenSucceeded) expect(apiMock.downloadExportFile).not.toHaveBeenCalled();
        }
        await waitFor(() => expect(result.current.dialogProps.phase).not.toBe("downloading"));
        expect(apiMock.downloadExportFile.mock.calls.length).toBeLessThanOrEqual(1);
        unmount();
      }),
      { numRuns: 25 },
    );
  }, 60_000);

  it("guards against leaving the page only while waiting or downloading", async () => {
    const add = vi.spyOn(window, "addEventListener");
    apiMock.requestExport.mockResolvedValue({ job: mkJob("queued"), created: true });
    apiMock.getExportJob.mockResolvedValue(mkJob("running"));
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const { result } = renderHook(() => useExportFlow(), {
      wrapper: ({ children }: { children: ReactNode }) => (
        <QueryClientProvider client={qc}>{children}</QueryClientProvider>
      ),
    });
    expect(add.mock.calls.some((c) => c[0] === "beforeunload")).toBe(false);
    await act(async () => {
      await result.current.request(params);
    });
    expect(add.mock.calls.some((c) => c[0] === "beforeunload")).toBe(true);
    const remove = vi.spyOn(window, "removeEventListener");
    act(() => result.current.dialogProps.onClose());
    expect(remove.mock.calls.some((c) => c[0] === "beforeunload")).toBe(true);
    add.mockRestore();
    remove.mockRestore();
  });
});
