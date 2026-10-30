import { fireEvent, render, screen } from "@testing-library/react";
import fc from "fast-check";
import type { ReactNode } from "react";
import { beforeEach, describe, it, vi } from "vitest";

import { useAuthStore } from "@/lib/auth/store";
import { DsrLateReportScreen } from "../DsrLateReportScreen";
import type { DsrLateStatus } from "../api";

// Feature: report-export-migration, Property 12: Tombol ekspor mengirim filter layar
// Validates: Requirements 7.1, 7.2, 7.4

const m = vi.hoisted(() => ({ request: vi.fn() }));

vi.mock("@/features/exports", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/features/exports")>();
  return {
    ...actual,
    useExportFlow: () => ({
      request: m.request,
      requestError: null,
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

const ok = (data: unknown) => ({ isLoading: false, isError: false, data, refetch: vi.fn() });
const SUMMARY_ROW = {
  vendor_id: 2,
  vendor_code: "ABA",
  vendor_name: "Abacus",
  vendor_branch_id: 10,
  branch_code: "ABA-00",
  branch_name: "Cabang Satu",
  region_code: "MDN",
  obliged_days: 1,
  ok_days: 1,
  late_days: 0,
  not_sent_days: 0,
  pending_days: 0,
  late_minutes_total: 0,
};

vi.mock("../api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../api")>();
  return {
    ...actual,
    useDsrLateDaily: () =>
      ok({ items: [], total: 0, page: 1, page_size: 20, start_date: "2026-10-01" }),
    useDsrLateSummary: () => ok({ items: [SUMMARY_ROW], start_date: "2026-10-01" }),
    useDsrLateWarnings: () => ok(undefined),
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

const screenState = fc.record({
  month: fc.constantFrom("2026-01", "2026-05", "2026-09", "2026-10"),
  vendor: fc.constantFrom(null, "2"),
  branch: fc.constantFrom(null, "10"),
  status: fc.constantFrom(null, ...STATUSES),
  visitedDaily: fc.boolean(),
  finalTab: fc.constantFrom("summary", "daily"),
});

beforeEach(() => {
  m.request.mockReset();
  useAuthStore.setState({ user: { role: "ATM-SPV" } as never });
});

describe("Property 12: DSR late export button sends the screen filter", () => {
  it("request equals exactly the filter state; status only on the daily tab", () => {
    fc.assert(
      fc.property(screenState, (s) => {
        m.request.mockReset();
        const { unmount } = render(<DsrLateReportScreen />);
        fireEvent.change(screen.getByLabelText("Bulan"), { target: { value: s.month } });
        if (s.vendor)
          fireEvent.change(screen.getByLabelText("Vendor"), { target: { value: s.vendor } });
        if (s.vendor && s.branch) {
          fireEvent.change(screen.getByLabelText("Cabang"), { target: { value: s.branch } });
        }
        const wantsDaily = s.finalTab === "daily";
        if (s.status || wantsDaily || s.visitedDaily) {
          fireEvent.click(screen.getByRole("tab", { name: "Detail harian" }));
          if (s.status)
            fireEvent.change(screen.getByLabelText("Status"), { target: { value: s.status } });
        }
        if (!wantsDaily) fireEvent.click(screen.getByRole("tab", { name: "Rekap per cabang" }));

        fireEvent.click(screen.getByRole("button", { name: /Ekspor CSV/ }));

        const params: Record<string, unknown> = { month: s.month };
        if (s.vendor) params.vendor_id = 2;
        if (s.vendor && s.branch) params.vendor_branch_id = 10;
        if (wantsDaily && s.status) params.status = s.status;
        const expected = {
          kind: "report",
          report: wantsDaily ? "dsr-late-daily" : "dsr-late-summary",
          params,
        };
        unmount();
        if (m.request.mock.calls.length !== 1) return false;
        return (
          JSON.stringify(sortKeys(m.request.mock.calls[0]?.[0])) ===
          JSON.stringify(sortKeys(expected))
        );
      }),
      { numRuns: 40 },
    );
  });
});

function sortKeys(v: unknown): unknown {
  if (Array.isArray(v)) return v.map(sortKeys);
  if (v && typeof v === "object") {
    return Object.fromEntries(
      Object.entries(v as Record<string, unknown>)
        .sort(([a], [b]) => a.localeCompare(b))
        .map(([k, val]) => [k, sortKeys(val)]),
    );
  }
  return v;
}
