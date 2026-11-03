import { fireEvent, render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { useAuthStore } from "@/lib/auth/store";
import { NAV_CONFIG, filterNavByRoles } from "@/lib/config/navigation";
import { DsrLateReportScreen } from "../DsrLateReportScreen";
import type { DsrLateDailyRow, DsrLateStatus, DsrLateSummaryRow, DsrLateWarnings } from "../api";

const m = vi.hoisted(() => ({
  daily: { current: null as unknown },
  summary: { current: null as unknown },
  warnings: { current: null as unknown },
  dailyArgs: vi.fn(),
  summaryArgs: vi.fn(),
  request: vi.fn(),
  requestError: { current: null as unknown },
}));

vi.mock("@/features/exports", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/features/exports")>();
  return {
    ...actual,
    useExportFlow: () => ({
      request: m.request,
      requestError: m.requestError.current,
      isRequesting: false,
      dialogProps: {
        open: false,
        phase: "idle",
        job: null,
        message: null,
        onClose: vi.fn(),
        onRetry: vi.fn(),
      },
    }),
  };
});

vi.mock("../api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../api")>();
  return {
    ...actual,
    useDsrLateDaily: (...a: unknown[]) => {
      m.dailyArgs(...a);
      return m.daily.current;
    },
    useDsrLateSummary: (...a: unknown[]) => {
      m.summaryArgs(...a);
      return m.summary.current;
    },
    useDsrLateWarnings: () => m.warnings.current,
  };
});
vi.mock("@/features/vendor-request/hooks", () => ({
  useVendorOptions: () => ({ data: { vendors: [{ id: 2, name: "Abacus" }], regions: [] } }),
}));
vi.mock("@tanstack/react-router", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    Link: ({ children }: { children?: ReactNode }) => <a href="/x">{children}</a>,
  };
});

const STATUSES: DsrLateStatus[] = ["ok", "late", "not_sent", "pending"];

function dailyRow(status: DsrLateStatus, i: number): DsrLateDailyRow {
  return {
    report_date: `2026-10-0${i + 1}`,
    vendor_id: 2,
    vendor_code: "ABA",
    vendor_name: "Abacus",
    vendor_branch_id: 10 + i,
    branch_code: `ABA-0${i}`,
    branch_name: "Cabang",
    region_code: "MDN",
    deadline: "2026-10-01T02:00:00Z",
    first_received_at: status === "ok" || status === "late" ? "2026-10-01T02:30:05Z" : null,
    receipt_checksum: "",
    matched_locations: [],
    status,
    late_minutes: status === "late" ? 30 : 0,
  };
}

const SUMMARY_ROW: DsrLateSummaryRow = {
  vendor_id: 2,
  vendor_code: "ABA",
  vendor_name: "Abacus",
  vendor_branch_id: 10,
  branch_code: "ABA-00",
  branch_name: "Cabang Satu",
  region_code: "MDN",
  obliged_days: 20,
  ok_days: 15,
  late_days: 3,
  not_sent_days: 2,
  pending_days: 0,
  late_minutes_total: 90,
};

const ok = (data: unknown) => ({ isLoading: false, isError: false, data, refetch: vi.fn() });
const EMPTY_WARN: DsrLateWarnings = {
  unmapped_vaults: [],
  unmapped_labels: [],
  unresolved_vendors: [],
  start_date: "2026-10-01",
};

function setAll() {
  m.daily.current = ok({
    items: STATUSES.map(dailyRow),
    total: 4,
    page: 1,
    page_size: 20,
    start_date: "2026-10-01",
  });
  m.summary.current = ok({ items: [SUMMARY_ROW], start_date: "2026-10-01" });
  m.warnings.current = ok(EMPTY_WARN);
}

beforeEach(() => {
  for (const f of [m.dailyArgs, m.summaryArgs, m.request]) f.mockReset();
  m.requestError.current = null;
  setAll();
  useAuthStore.setState({ user: { role: "ATM-SPV" } as never });
});

describe("DsrLateReportScreen", () => {
  it("renders every status with a text label and an icon", () => {
    render(<DsrLateReportScreen />);
    fireEvent.click(screen.getByRole("tab", { name: "Detail harian" }));
    for (const label of ["OK", "TELAT", "TIDAK KIRIM", "BELUM JATUH TEMPO"]) {
      const badge = screen
        .getAllByText(label)
        .find((e) => e.tagName === "SPAN" && e.querySelector("svg"));
      expect(badge, label).toBeDefined();
    }
    expect(screen.getByLabelText(/^TELAT\. Batas kirim 09:00 WIB/)).toBeInTheDocument();
  });

  it("shows received time in WIB and late minutes right-aligned", () => {
    render(<DsrLateReportScreen />);
    fireEvent.click(screen.getByRole("tab", { name: "Detail harian" }));
    expect(screen.getAllByText("01/10/2026 09:30:05")[0]).toHaveClass("tabular-nums");
    expect(screen.getByText("30")).toHaveClass("text-right", "tabular-nums");
  });

  it("shows the summary tab with per-branch totals", () => {
    render(<DsrLateReportScreen />);
    expect(screen.getByText("ABA-00 – Cabang Satu")).toBeInTheDocument();
    expect(screen.getByText("90")).toHaveClass("tabular-nums");
  });

  it("passes filters to the queries and resets the page", () => {
    render(<DsrLateReportScreen />);
    fireEvent.click(screen.getByRole("tab", { name: "Detail harian" }));
    fireEvent.change(screen.getByLabelText("Bulan"), { target: { value: "2026-09" } });
    fireEvent.change(screen.getByLabelText("Vendor"), { target: { value: "2" } });
    fireEvent.change(screen.getByLabelText("Status"), { target: { value: "late" } });
    const [f, page] = m.dailyArgs.mock.calls.at(-1) as [Record<string, unknown>, number];
    expect(f).toMatchObject({ month: "2026-09", vendorId: 2, status: "late" });
    expect(page).toBe(1);
    fireEvent.change(screen.getByLabelText("Cabang"), { target: { value: "10" } });
    expect((m.dailyArgs.mock.calls.at(-1) as [Record<string, unknown>])[0]).toMatchObject({
      vendorBranchId: 10,
    });
  });

  it("summary tab exports dsr-late-summary with the filter and no status", () => {
    render(<DsrLateReportScreen />);
    fireEvent.change(screen.getByLabelText("Vendor"), { target: { value: "2" } });
    fireEvent.click(screen.getByRole("button", { name: /Ekspor CSV/ }));
    expect(m.request).toHaveBeenCalledWith({
      kind: "report",
      report: "dsr-late-summary",
      params: { month: expect.stringMatching(/^\d{4}-\d{2}$/), vendor_id: 2 },
    });
  });

  it("detail tab exports dsr-late-daily with the status and branch filter", () => {
    render(<DsrLateReportScreen />);
    fireEvent.change(screen.getByLabelText("Vendor"), { target: { value: "2" } });
    fireEvent.change(screen.getByLabelText("Cabang"), { target: { value: "10" } });
    fireEvent.click(screen.getByRole("tab", { name: "Detail harian" }));
    fireEvent.change(screen.getByLabelText("Status"), { target: { value: "late" } });
    fireEvent.click(screen.getByRole("button", { name: /Ekspor CSV/ }));
    expect(m.request).toHaveBeenCalledWith({
      kind: "report",
      report: "dsr-late-daily",
      params: {
        month: expect.stringMatching(/^\d{4}-\d{2}$/),
        vendor_id: 2,
        vendor_branch_id: 10,
        status: "late",
      },
    });
  });

  it("a status chosen on the detail tab is not sent from the summary tab", () => {
    render(<DsrLateReportScreen />);
    fireEvent.click(screen.getByRole("tab", { name: "Detail harian" }));
    fireEvent.change(screen.getByLabelText("Status"), { target: { value: "late" } });
    fireEvent.click(screen.getByRole("tab", { name: "Rekap per cabang" }));
    fireEvent.click(screen.getByRole("button", { name: /Ekspor CSV/ }));
    expect(m.request.mock.calls[0]?.[0].params).not.toHaveProperty("status");
  });

  it("shows a 429 inline with text and no dialog, plus the Ekspor Saya link", () => {
    const { rerender } = render(<DsrLateReportScreen />);
    expect(screen.getByRole("link", { name: "Lihat Ekspor Saya" })).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    m.requestError.current = { status: 429, code: "too_many_exports", message: "x" };
    rerender(<DsrLateReportScreen />);
    expect(screen.getByRole("alert")).toHaveTextContent("Terlalu banyak ekspor");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("shows the warnings panel with the excluded-branch marker and admin link", () => {
    useAuthStore.setState({ user: { role: "ADMIN" } as never });
    m.warnings.current = ok({
      ...EMPTY_WARN,
      unmapped_vaults: [
        {
          vendor_id: 2,
          vendor_name: "Abacus",
          vendor_branch_id: 10,
          branch_code: "ABA-00",
          branch_name: "Cabang Satu",
          vendor_vault_id: 5,
          vault_code: "V5",
          branch_excluded: true,
        },
      ],
      unmapped_labels: [
        {
          vendor_id: 2,
          vendor_name: "Abacus",
          dsr_location: "LOKASI X",
          receipt_count: 3,
          first_report_date: null,
          last_report_date: null,
        },
      ],
      unresolved_vendors: [
        {
          vendor_label: "Vendor Asing",
          receipt_count: 1,
          first_report_date: null,
          last_report_date: null,
        },
      ],
    });
    render(<DsrLateReportScreen />);
    expect(screen.getByText(/cabang dikeluarkan dari laporan/)).toBeInTheDocument();
    expect(screen.getByText(/LOKASI X/)).toBeInTheDocument();
    expect(screen.getByText(/Vendor Asing/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "master data vendor" })).toBeInTheDocument();
  });

  it("hides the master data link for non-admin and the panel when there are no gaps", () => {
    const { rerender } = render(<DsrLateReportScreen />);
    expect(screen.queryByLabelText("Peringatan pemetaan")).not.toBeInTheDocument();
    m.warnings.current = ok({
      ...EMPTY_WARN,
      unresolved_vendors: [
        { vendor_label: "Z", receipt_count: 1, first_report_date: null, last_report_date: null },
      ],
    });
    rerender(<DsrLateReportScreen />);
    expect(screen.getByLabelText("Peringatan pemetaan")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "master data vendor" })).not.toBeInTheDocument();
  });

  it("shows loading, empty-before-start-date and feature-off states", () => {
    m.summary.current = { isLoading: true, isError: false, data: undefined, refetch: vi.fn() };
    const { rerender } = render(<DsrLateReportScreen />);
    expect(screen.getByText("Memuat data...")).toBeInTheDocument();

    m.summary.current = ok({ items: [], start_date: "2026-10-01" });
    rerender(<DsrLateReportScreen />);
    expect(screen.getByText(/Laporan dihitung mulai/)).toBeInTheDocument();

    m.summary.current = ok({ items: [], start_date: null });
    m.daily.current = ok({ items: [], total: 0, page: 1, page_size: 20, start_date: null });
    rerender(<DsrLateReportScreen />);
    expect(screen.getByText("Laporan belum aktif.")).toBeInTheDocument();
  });

  it("shows a forbidden message on 403 and a retry on other errors", () => {
    m.summary.current = {
      isLoading: false,
      isError: true,
      error: { status: 403 },
      refetch: vi.fn(),
    };
    const { rerender } = render(<DsrLateReportScreen />);
    expect(screen.getByRole("alert")).toHaveTextContent("Anda tidak memiliki akses");

    const refetch = vi.fn();
    m.summary.current = { isLoading: false, isError: true, error: { status: 500 }, refetch };
    rerender(<DsrLateReportScreen />);
    fireEvent.click(screen.getByRole("button", { name: /Coba lagi/ }));
    expect(refetch).toHaveBeenCalled();
  });
});

describe("menu Laporan DSR Telat", () => {
  const visible = (role: Parameters<typeof filterNavByRoles>[1]) =>
    filterNavByRoles(NAV_CONFIG, role).some((i) => i.id === "dsr-late-report");

  it.each(["ATM-USER", "ATM-SPV", "BRANCH-ATM-SPV", "ADMIN"] as const)("visible for %s", (r) => {
    expect(visible(r)).toBe(true);
  });

  it.each(["VENDOR-USER", "ACM-USER", "BRANCH-ATM-USER"] as const)("hidden for %s", (r) => {
    expect(visible(r)).toBe(false);
  });

  it("sits in the monitoring group at /dsr-late-report", () => {
    expect(NAV_CONFIG.find((i) => i.id === "dsr-late-report")).toMatchObject({
      group: "monitoring",
      href: "/dsr-late-report",
    });
  });
});
