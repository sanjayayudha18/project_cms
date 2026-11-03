import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ImportBatchPage } from "../api";
import { ImportHistoryTab, toBatchFilter } from "./ImportHistoryTab";

const router = vi.hoisted(() => ({ search: {} as Record<string, unknown>, navigate: vi.fn() }));
vi.mock("@tanstack/react-router", () => ({
  useSearch: () => router.search,
  useNavigate: () => router.navigate,
}));

const list = vi.fn();
vi.mock("../api", async (orig) => ({
  ...(await orig<typeof import("../api")>()),
  listImportBatches: (...a: unknown[]) => list(...a),
}));

const page = (over: Partial<ImportBatchPage> = {}): ImportBatchPage => ({
  items: [],
  total: 0,
  page: 1,
  page_size: 20,
  ...over,
});

const renderTab = () =>
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <ImportHistoryTab />
    </QueryClientProvider>,
  );

describe("ImportHistoryTab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    router.search = { tab: "history" };
    list.mockResolvedValue(page());
  });

  it("maps URL params to the API filter with page size 20", () => {
    expect(toBatchFilter({ h_entity: "atms", h_status: "stale", h_mine: true, h_page: 3 })).toEqual(
      { entity: "atms", status: "stale", mine: true, page: 3, page_size: 20 },
    );
    expect(toBatchFilter({})).toMatchObject({ page: 1, mine: undefined });
  });

  it("queries with the URL filters", async () => {
    router.search = { tab: "history", h_entity: "vendors", h_status: "pending", h_mine: true };
    renderTab();
    await screen.findByText(/tidak ada impor yang cocok/i);
    expect(list).toHaveBeenCalledWith({
      entity: "vendors",
      status: "pending",
      mine: true,
      page: 1,
      page_size: 20,
    });
  });

  it("writes a changed filter to the URL and resets to page 1", async () => {
    router.search = { tab: "history", h_page: 4 };
    renderTab();
    await userEvent.selectOptions(await screen.findByLabelText("Status"), "applied");
    expect(router.navigate).toHaveBeenLastCalledWith({
      to: ".",
      search: { tab: "history", h_status: "applied" },
    });
  });

  it("shows the plain empty message without filters", async () => {
    renderTab();
    expect(
      await screen.findByText("Belum ada impor. Ajukan file lewat tab Impor baru."),
    ).toBeTruthy();
  });

  it("renders rows with status label, dash for a missing file name, and opens a batch", async () => {
    list.mockResolvedValue(
      page({
        total: 1,
        items: [
          {
            id: 10,
            entity: "vendor-branches",
            file_name: null,
            row_count: 5,
            maker_id: 1,
            maker_name: "Admin",
            created_at: "2026-10-28T01:45:00Z",
            status: "approved",
            approval_request_id: 3,
            error: null,
            decided_at: null,
            decided_by_name: null,
          },
        ],
      }),
    );
    renderTab();
    const table = within(await screen.findByRole("table"));
    expect(table.getByText("Disetujui belum diterapkan")).toBeTruthy();
    expect(table.getByText("Cabang vendor")).toBeTruthy();
    expect(table.getAllByText("—").length).toBe(2);
    expect(table.getByText(/^28 \w+ 2026 08:45$/)).toBeTruthy();
    await userEvent.click(screen.getByRole("button", { name: "Buka batch 10" }));
    expect(router.navigate).toHaveBeenLastCalledWith({
      to: ".",
      search: { tab: "history", batch: 10 },
    });
  });

  const item = (id: number, status: ImportBatchPage["items"][number]["status"]) => ({
    id,
    entity: "atms" as const,
    file_name: `f${id}.csv`,
    row_count: 2,
    maker_id: 1,
    maker_name: "Admin",
    created_at: "2026-10-28T01:45:00Z",
    status,
    approval_request_id: id,
    error: null,
    decided_at: null,
    decided_by_name: null,
  });

  it("shows an icon + label badge for each of the five statuses", async () => {
    list.mockResolvedValue(
      page({
        total: 5,
        items: [
          item(1, "pending"),
          item(2, "approved"),
          item(3, "rejected"),
          item(4, "applied"),
          item(5, "stale"),
        ],
      }),
    );
    renderTab();
    const table = within(await screen.findByRole("table"));
    for (const label of [
      "Menunggu persetujuan",
      "Disetujui belum diterapkan",
      "Ditolak",
      "Diterapkan",
      "Gagal diterapkan",
    ]) {
      const badge = table.getByText(label).closest("span");
      expect(badge?.querySelector("svg"), label).not.toBeNull();
    }
  });

  it("pages the list: page 2 goes to the URL", async () => {
    list.mockResolvedValue(page({ total: 45, items: [item(1, "applied")] }));
    renderTab();
    await screen.findByRole("table");
    expect(screen.getByText(/menampilkan 1-1 dari 45 batch/i)).toBeTruthy();
    await userEvent.click(screen.getByRole("button", { name: "Halaman 2 dari 3" }));
    expect(router.navigate).toHaveBeenLastCalledWith({
      to: ".",
      search: { tab: "history", h_page: 2 },
    });
  });

  it("queries the requested page and returns to page 1 without an h_page param", async () => {
    router.search = { tab: "history", h_page: 2 };
    list.mockResolvedValue(page({ total: 45, items: [item(1, "applied")] }));
    renderTab();
    await screen.findByRole("table");
    expect(list).toHaveBeenCalledWith(expect.objectContaining({ page: 2 }));
    await userEvent.click(screen.getByRole("button", { name: "Halaman 1 dari 3" }));
    expect(router.navigate).toHaveBeenLastCalledWith({ to: ".", search: { tab: "history" } });
  });

  it("writes the entity and 'mine' filters to the URL", async () => {
    renderTab();
    await userEvent.selectOptions(await screen.findByLabelText("Entitas"), "atms");
    expect(router.navigate).toHaveBeenLastCalledWith({
      to: ".",
      search: { tab: "history", h_entity: "atms" },
    });
    await userEvent.click(screen.getByLabelText("Hanya milik saya"));
    expect(router.navigate).toHaveBeenLastCalledWith({
      to: ".",
      search: { tab: "history", h_mine: true },
    });
  });

  it("opens a batch with the keyboard", async () => {
    list.mockResolvedValue(page({ total: 1, items: [item(7, "applied")] }));
    renderTab();
    const open = await screen.findByRole("button", { name: "Buka batch 7" });
    open.focus();
    await userEvent.keyboard("{Enter}");
    expect(router.navigate).toHaveBeenLastCalledWith({
      to: ".",
      search: { tab: "history", batch: 7 },
    });
  });

  it("shows an error with a retry button", async () => {
    list.mockRejectedValueOnce(new Error("boom"));
    renderTab();
    expect(await screen.findByText("Riwayat impor gagal dimuat.")).toBeTruthy();
    await userEvent.click(screen.getByRole("button", { name: "Coba lagi" }));
    expect(list).toHaveBeenCalledTimes(2);
  });
});
