import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ImportBatchDetail } from "../api";
import { ImportBatchDialog } from "./ImportBatchDialog";

const router = vi.hoisted(() => ({ search: {} as Record<string, unknown>, navigate: vi.fn() }));
vi.mock("@tanstack/react-router", () => ({
  useSearch: () => router.search,
  useNavigate: () => router.navigate,
}));

const getBatch = vi.fn();
const retryApply = vi.fn();
vi.mock("../api", async (orig) => ({
  ...(await orig<typeof import("../api")>()),
  getImportBatch: (...a: unknown[]) => getBatch(...a),
  retryApplyImport: (...a: unknown[]) => retryApply(...a),
}));

const toast = vi.hoisted(() => vi.fn());
vi.mock("@/lib/hooks/useToast", () => ({ useToast: () => ({ toast }) }));

const change = (
  over: Partial<ImportBatchDetail["changes"][number]> = {},
): ImportBatchDetail["changes"][number] => ({
  id: 1,
  entity_type: "vendor_branch",
  entity_id: null,
  op: "create",
  payload: { code: "B01" },
  before: null,
  status: "approved",
  maker_id: 1,
  error: null,
  failed: false,
  ...over,
});

const apiError = (status: number, message: string) => Object.assign(new Error(message), { status });

const batch = (over: Partial<ImportBatchDetail> = {}): ImportBatchDetail => ({
  id: 10,
  entity: "vendor-branches",
  file_name: null,
  row_count: 1,
  maker_id: 1,
  maker_name: "Budi",
  created_at: "2026-11-02T02:40:00Z",
  status: "approved",
  approval_request_id: 77,
  error: null,
  decided_at: null,
  decided_by_name: null,
  counts: { create: 1, update: 0, disable: 0, enable: 0 },
  changes: [change()],
  failure_reason: null,
  total: 1,
  page: 1,
  page_size: 50,
  ...over,
});

const renderDialog = () =>
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <ImportBatchDialog />
    </QueryClientProvider>,
  );

describe("ImportBatchDialog", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    router.search = { tab: "history", batch: 10, h_status: "approved" };
  });

  it("offers retry for an approved batch and calls the API with the approval id", async () => {
    getBatch.mockResolvedValue(batch());
    retryApply.mockResolvedValue(undefined);
    renderDialog();
    await userEvent.click(await screen.findByRole("button", { name: /coba terapkan lagi/i }));
    expect(retryApply).toHaveBeenCalledTimes(1);
    expect(retryApply.mock.calls[0]?.[0]).toBe(77);
  });

  it.each(["pending", "rejected", "applied", "stale"] as const)(
    "hides retry for a %s batch",
    async (status) => {
      getBatch.mockResolvedValue(batch({ status }));
      renderDialog();
      await screen.findByText("B01");
      expect(screen.queryByRole("button", { name: /coba terapkan lagi/i })).toBeNull();
    },
  );

  it("hides retry and explains when the approval request is missing", async () => {
    getBatch.mockResolvedValue(batch({ approval_request_id: null }));
    renderDialog();
    expect(await screen.findByText(/pengajuan belum lengkap\. ajukan ulang/i)).toBeTruthy();
    expect(screen.queryByRole("button", { name: /coba terapkan lagi/i })).toBeNull();
  });

  it("refetches the batch and confirms after a successful retry", async () => {
    getBatch.mockResolvedValue(batch());
    retryApply.mockResolvedValue(undefined);
    renderDialog();
    await userEvent.click(await screen.findByRole("button", { name: /coba terapkan lagi/i }));
    await waitFor(() => expect(getBatch).toHaveBeenCalledTimes(2));
    expect(toast).toHaveBeenCalledWith({ type: "success", message: "Batch berhasil diterapkan" });
  });

  it.each([
    [409, "Batch sudah berubah"],
    [500, "Server error"],
  ])("shows a %i retry failure as a toast and still refetches", async (status, message) => {
    getBatch.mockResolvedValue(batch());
    retryApply.mockRejectedValue(apiError(status, message));
    renderDialog();
    await userEvent.click(await screen.findByRole("button", { name: /coba terapkan lagi/i }));
    await waitFor(() => expect(toast).toHaveBeenCalledWith({ type: "error", message }));
    await waitFor(() => expect(getBatch).toHaveBeenCalledTimes(2));
  });

  it("sends a double click on retry only once", async () => {
    getBatch.mockResolvedValue(batch());
    retryApply.mockReturnValue(new Promise(() => {}));
    renderDialog();
    const user = userEvent.setup();
    const retry = await screen.findByRole("button", { name: /coba terapkan lagi/i });
    await user.dblClick(retry);
    expect(retryApply).toHaveBeenCalledTimes(1);
    expect((retry as HTMLButtonElement).disabled).toBe(true);
  });

  it("discloses the field diff of a row, with a Berubah label for changed update fields", async () => {
    getBatch.mockResolvedValue(
      batch({
        changes: [
          change({
            op: "update",
            payload: { name: "Baru", city: "Jakarta" },
            before: { name: "Lama", city: "Jakarta" },
          }),
        ],
      }),
    );
    renderDialog();
    const toggle = await screen.findByRole("button", { name: /lihat perubahan/i });
    expect(toggle.getAttribute("aria-expanded")).toBe("false");
    await userEvent.click(toggle);
    expect(toggle.getAttribute("aria-expanded")).toBe("true");
    const diff = within(
      screen.getByRole("columnheader", { name: "Bidang" }).closest("table") as HTMLElement,
    );
    expect(diff.getByText("Lama")).toBeTruthy();
    expect(diff.getByText("Baru")).toBeTruthy();
    expect(diff.getAllByText("Berubah")).toHaveLength(1);
  });

  it("labels the failing row Gagal with its message and the other stale rows Dibatalkan", async () => {
    getBatch.mockResolvedValue(
      batch({
        status: "stale",
        failure_reason: "kode cabang sudah ada",
        changes: [
          change({ id: 1, status: "stale", error: "batch rolled back: change request 2 failed" }),
          change({
            id: 2,
            status: "stale",
            failed: true,
            error: "kode cabang sudah ada",
            payload: { code: "B02" },
          }),
        ],
        counts: { create: 2, update: 0, disable: 0, enable: 0 },
        total: 2,
      }),
    );
    renderDialog();
    expect(await screen.findByText("Gagal")).toBeTruthy();
    expect(screen.getAllByText("Dibatalkan")).toHaveLength(1);
    expect(screen.getByText(/tidak ada data yang diterapkan/i)).toBeTruthy();
  });

  it("names the cause from failure_reason when the failing row is on another page", async () => {
    getBatch.mockResolvedValue(
      batch({
        status: "stale",
        failure_reason: "kode cabang sudah ada",
        error: "batch rolled back: change request 12 failed",
        changes: [
          change({ status: "stale", error: "batch rolled back: change request 12 failed" }),
        ],
        total: 120,
      }),
    );
    renderDialog();
    expect(
      await screen.findByText(/kode cabang sudah ada\. file yang sama boleh diajukan ulang/i),
    ).toBeTruthy();
    expect(screen.queryByText(/rolled back/i)).toBeNull();
  });

  it("shows 'Penyebab tidak tercatat' for an old stale batch and never the rollback text", async () => {
    getBatch.mockResolvedValue(
      batch({
        status: "stale",
        error: "batch rolled back: change request 12 failed",
        changes: [
          change({ status: "stale", error: "batch rolled back: change request 12 failed" }),
        ],
      }),
    );
    renderDialog();
    expect(await screen.findByText(/penyebab tidak tercatat/i)).toBeTruthy();
    expect(screen.queryByText(/rolled back/i)).toBeNull();
  });

  it("pages rows on the server", async () => {
    getBatch.mockResolvedValue(batch({ total: 120 }));
    renderDialog();
    await screen.findByText("B01");
    expect(getBatch).toHaveBeenLastCalledWith(10, 1, 50);
    await userEvent.click(screen.getByRole("button", { name: "Halaman 2 dari 3" }));
    await waitFor(() => expect(getBatch).toHaveBeenLastCalledWith(10, 2, 50));
  });

  it("drops `batch` from the URL and toasts on a 404", async () => {
    getBatch.mockRejectedValue(apiError(404, "not found"));
    renderDialog();
    await waitFor(() =>
      expect(router.navigate).toHaveBeenCalledWith({
        to: ".",
        search: { tab: "history", h_status: "approved" },
      }),
    );
    expect(toast).toHaveBeenCalledWith({ type: "error", message: "Batch tidak ditemukan" });
  });

  it("closing removes only `batch` from the URL", async () => {
    getBatch.mockResolvedValue(batch());
    renderDialog();
    const close = (await screen.findAllByRole("button", { name: "Tutup" }))[0] as HTMLElement;
    await userEvent.click(close);
    expect(router.navigate).toHaveBeenCalledWith({
      to: ".",
      search: { tab: "history", h_status: "approved" },
    });
  });
});
