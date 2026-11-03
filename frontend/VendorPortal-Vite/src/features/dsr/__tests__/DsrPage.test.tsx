import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// fe-coverage C4: DSR Monitor list. The uploads hook is mocked; both dialogs are stubbed
// (they have their own tests).
const m = vi.hoisted(() => ({
  state: {} as Record<string, unknown>,
  dates: [] as (string | undefined)[],
}));
vi.mock("@/features/dsr/useDsrUploads", () => ({
  useDsrUploads: (date?: string) => {
    m.dates.push(date);
    return m.state;
  },
}));
vi.mock("@/features/dsr/DsrUploadDialog", () => ({
  DsrUploadDialog: ({ onClose, onConfirmed }: { onClose: () => void; onConfirmed: () => void }) => (
    <dialog open aria-label="upload">
      <button type="button" onClick={onClose}>
        batal unggah
      </button>
      <button type="button" onClick={onConfirmed}>
        konfirmasi unggah
      </button>
    </dialog>
  ),
}));
vi.mock("@/features/dsr/DsrDetailDialog", () => ({
  DsrDetailDialog: ({ item, onClose }: { item: { report_date: string }; onClose: () => void }) => (
    <dialog open aria-label="detail">
      detail {item.report_date}
      <button type="button" onClick={onClose}>
        tutup detail
      </button>
    </dialog>
  ),
}));

import { DsrPage } from "../DsrPage";

const ROWS = [
  {
    report_date: "2026-10-14",
    daily: { file_id: 1, status: "completed" },
    rencana_isi: { file_id: 2, status: "failed" },
  },
  { report_date: "2026-10-13", daily: { file_id: 3, status: "weird" }, rencana_isi: null },
];

function renderPage() {
  const queryClient = new QueryClient();
  const invalidate = vi.spyOn(queryClient, "invalidateQueries");
  render(
    <QueryClientProvider client={queryClient}>
      <DsrPage />
    </QueryClientProvider>,
  );
  return invalidate;
}

beforeEach(() => {
  m.dates = [];
  m.state = { isLoading: false, data: { data: ROWS, total: 2 } };
});

describe("DsrPage", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("opens filtered on today's date in Asia/Jakarta, not UTC", () => {
    // 20:00 UTC on 15 Oct = 03:00 WIB on 16 Oct.
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date("2026-10-15T20:00:00Z"));
    renderPage();

    expect(m.dates[0]).toBe("2026-10-16");
    expect(screen.getByLabelText("Tanggal")).toHaveValue("2026-10-16");
  });

  it("shows a loading message (upload still reachable)", () => {
    m.state = { isLoading: true, data: undefined };
    renderPage();

    expect(screen.getByText("Memuat data...")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Add DSR" })).toBeInTheDocument();
  });

  it("shows the empty state when there is no upload for the date", () => {
    m.state = { isLoading: false, data: { data: [], total: 0 } };
    renderPage();

    expect(screen.getByText("Tidak ada data DSR")).toBeInTheDocument();
  });

  it("lists one row per report date with a status per sheet (text, not colour only)", () => {
    renderPage();

    expect(screen.getByText("2026-10-14")).toBeInTheDocument();
    expect(screen.getByText("completed")).toBeInTheDocument();
    expect(screen.getByText("failed")).toBeInTheDocument();
    expect(screen.getByText("weird")).toBeInTheDocument();
    expect(screen.getByText("-")).toBeInTheDocument(); // no rencana isi uploaded
  });

  it("filters by the chosen date, and 'Tampilkan Semua' drops the filter", async () => {
    renderPage();

    fireEvent.change(screen.getByLabelText("Tanggal"), { target: { value: "2026-10-14" } });
    expect(m.dates.at(-1)).toBe("2026-10-14");

    await userEvent.setup().click(screen.getByRole("button", { name: "Tampilkan Semua" }));
    expect(m.dates.at(-1)).toBeUndefined();
  });

  it("clicking a row opens its detail; closing it returns to the list", async () => {
    const user = userEvent.setup();
    renderPage();

    await user.click(screen.getByText("2026-10-13"));
    expect(screen.getByRole("dialog", { name: "detail" })).toHaveTextContent("detail 2026-10-13");

    await user.click(screen.getByRole("button", { name: "tutup detail" }));
    expect(screen.queryByRole("dialog", { name: "detail" })).not.toBeInTheDocument();
  });

  it("a confirmed upload closes the dialog and refreshes the list; cancel just closes", async () => {
    const user = userEvent.setup();
    const invalidate = renderPage();

    await user.click(screen.getByRole("button", { name: "Add DSR" }));
    await user.click(screen.getByRole("button", { name: "batal unggah" }));
    expect(screen.queryByRole("dialog", { name: "upload" })).not.toBeInTheDocument();
    expect(invalidate).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "Add DSR" }));
    await user.click(screen.getByRole("button", { name: "konfirmasi unggah" }));
    expect(screen.queryByRole("dialog", { name: "upload" })).not.toBeInTheDocument();
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ["dsr-uploads"] });
  });
});
