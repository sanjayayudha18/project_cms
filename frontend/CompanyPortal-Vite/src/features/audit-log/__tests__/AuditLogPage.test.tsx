import { formatWibDateTimeSec } from "@/features/eod-monitoring/utils";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AuditLogPage } from "../components/AuditLogPage";
import type { AuditLogDetail, AuditLogListItem } from "../types";

function item(overrides: Partial<AuditLogListItem> = {}): AuditLogListItem {
  return {
    id: 1,
    actor_id: 3,
    action: "approve",
    entity_type: "vendor_request",
    entity_id: 42,
    ip: "10.0.0.5",
    created_at: "2026-09-21T03:04:05Z",
    ...overrides,
  };
}

const listMock = vi.fn();
const detailMock = vi.fn();
const navigateMock = vi.fn();
let searchState: Record<string, unknown> = {};

vi.mock("../hooks/useAuditQueries", () => ({
  useAuditLogList: (...args: unknown[]) => listMock(...args),
  useAuditLogDetail: (...args: unknown[]) => detailMock(...args),
}));

vi.mock("@tanstack/react-router", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return { ...actual, useNavigate: () => navigateMock, useSearch: () => searchState };
});

function listResult(rows: AuditLogListItem[], total = rows.length) {
  return {
    data: { data: rows, page: 1, page_size: 25, total },
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  };
}

// Month abbreviation is locale/ICU-dependent ("Sep" vs "Sept"), so derive it from the formatter.
const TIMESTAMP_BUTTON = formatWibDateTimeSec("2026-09-21T03:04:05Z");

beforeEach(() => {
  listMock.mockReset();
  detailMock.mockReset();
  navigateMock.mockReset();
  searchState = {};
  listMock.mockReturnValue(listResult([item()]));
  detailMock.mockReturnValue({ data: undefined, isLoading: false, isError: false });
});

describe("AuditLogPage list", () => {
  it("shows the six columns with a WIB timestamp and an icon+label action badge", () => {
    render(<AuditLogPage />);

    for (const h of ["Waktu (WIB)", "Aktor", "Aksi", "Tipe Entitas", "ID Entitas", "IP"]) {
      expect(screen.getByRole("columnheader", { name: h })).toBeInTheDocument();
    }
    expect(screen.getByText(TIMESTAMP_BUTTON)).toBeInTheDocument();
    expect(TIMESTAMP_BUTTON).toMatch(/^21 \w+ 2026 10:04:05$/); // 03:04:05Z is 10:04:05 WIB
    const badge = within(screen.getByRole("table")).getByText("approve"); // not the Aksi <option>s
    expect(badge.parentElement?.querySelector("svg")).toBeInTheDocument();
  });

  it("shows the empty state, skeleton and error states", () => {
    listMock.mockReturnValue(listResult([]));
    const empty = render(<AuditLogPage />);
    expect(screen.getByText("Tidak ada log audit")).toBeInTheDocument();
    empty.unmount();

    listMock.mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
      refetch: vi.fn(),
    });
    const loading = render(<AuditLogPage />);
    expect(loading.container.querySelectorAll(".animate-pulse").length).toBeGreaterThan(0);
    loading.unmount();

    listMock.mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      refetch: vi.fn(),
    });
    render(<AuditLogPage />);
    expect(screen.getByText("Gagal memuat log audit")).toBeInTheDocument();
  });

  it("paginates via the URL, keeping page in range", async () => {
    const user = userEvent.setup();
    searchState = { page: 2 };
    listMock.mockReturnValue(listResult([item()], 80)); // 80 rows / 25 = 4 pages
    render(<AuditLogPage />);

    expect(screen.getByText(/Halaman 2 dari 4/)).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Berikutnya" }));
    expect(navigateMock).toHaveBeenLastCalledWith({ to: ".", search: { page: 3 } });
    await user.click(screen.getByRole("button", { name: "Sebelumnya" }));
    expect(navigateMock).toHaveBeenLastCalledWith({ to: ".", search: {} });
  });

  it("passes the URL filters and page to the list query", () => {
    searchState = { page: 3, action: "reject", actor_id: 9 };
    render(<AuditLogPage />);

    expect(listMock).toHaveBeenCalledWith(
      expect.objectContaining({ action: "reject", actor_id: "9" }),
      3,
      25,
    );
  });
});

describe("AuditFilterBar", () => {
  it("changing a filter navigates with page reset", async () => {
    const user = userEvent.setup();
    searchState = { page: 3 };
    render(<AuditLogPage />);

    await user.selectOptions(screen.getByLabelText("Aksi"), "reject");

    expect(navigateMock).toHaveBeenLastCalledWith({ to: ".", search: { action: "reject" } });
  });

  it("blocks a from-date after the to-date and explains why", async () => {
    const user = userEvent.setup();
    searchState = { date_to: "2026-09-10" };
    render(<AuditLogPage />);

    await user.type(screen.getByLabelText("Dari Tanggal"), "2026-09-20");

    expect(screen.getByRole("alert")).toHaveTextContent(/tidak boleh setelah/);
    expect(navigateMock).not.toHaveBeenCalled();
  });

  it("only accepts a numeric actor id", async () => {
    const user = userEvent.setup();
    render(<AuditLogPage />);

    await user.type(screen.getByLabelText("ID Aktor"), "12{Enter}");

    expect(navigateMock).toHaveBeenLastCalledWith({ to: ".", search: { actor_id: 12 } });
  });
});

describe("AuditDetailDrawer", () => {
  const CREATED: AuditLogDetail = {
    ...item({ action: "create" }),
    before: null,
    after: { status: "draft", amount: 5 },
  };

  function mockDetail(data: AuditLogDetail) {
    detailMock.mockImplementation((id: number | null) => ({
      data: id === null ? undefined : data,
      isLoading: false,
      isError: false,
    }));
  }

  it("opens on click with metadata and a Dibuat diff, and closes on Escape", async () => {
    const user = userEvent.setup();
    mockDetail(CREATED);
    render(<AuditLogPage />);

    await user.click(screen.getByRole("button", { name: TIMESTAMP_BUTTON }));

    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByText("Dibuat")).toBeInTheDocument();
    expect(within(dialog).getByText("status")).toBeInTheDocument();
    expect(within(dialog).getByText("draft")).toBeInTheDocument();

    await user.keyboard("{Escape}");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("returns focus to the row button that opened it after Escape", async () => {
    const user = userEvent.setup();
    mockDetail(CREATED);
    render(<AuditLogPage />);

    const trigger = screen.getByRole("button", { name: TIMESTAMP_BUTTON });
    await user.click(trigger);
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    await user.keyboard("{Escape}");

    // Same node, still mounted and focused: columns must not remount the button on re-render.
    expect(trigger).toBeInTheDocument();
    expect(trigger).toHaveFocus();
  });

  it("shows an inline error with retry and stays open", async () => {
    const user = userEvent.setup();
    const refetch = vi.fn();
    detailMock.mockImplementation((id: number | null) => ({
      data: undefined,
      isLoading: false,
      isError: id !== null,
      error: { message: "boom" },
      refetch,
    }));
    render(<AuditLogPage />);

    await user.click(screen.getByRole("button", { name: TIMESTAMP_BUTTON }));

    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByRole("alert")).toHaveTextContent("boom");
    await user.click(within(dialog).getByRole("button", { name: "Coba Lagi" }));
    expect(refetch).toHaveBeenCalled();
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });

  it("labels a modification and an unchanged field with icon+label badges", async () => {
    const user = userEvent.setup();
    mockDetail({ ...CREATED, before: { a: 1, b: 2 }, after: { a: 1, b: 3 } });
    render(<AuditLogPage />);

    await user.click(screen.getByRole("button", { name: TIMESTAMP_BUTTON }));

    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByText("Berubah").parentElement?.querySelector("svg")).toBeTruthy();
    expect(within(dialog).getByText("Tetap")).toBeInTheDocument();
  });

  it("labels a removal Dihapus", async () => {
    const user = userEvent.setup();
    mockDetail({ ...CREATED, before: { a: 1 }, after: null });
    render(<AuditLogPage />);

    await user.click(screen.getByRole("button", { name: TIMESTAMP_BUTTON }));

    expect(within(screen.getByRole("dialog")).getAllByText("Dihapus").length).toBeGreaterThan(0);
  });
});
