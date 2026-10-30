import { fireEvent, render, screen } from "@testing-library/react";
import fc from "fast-check";
import type { ReactNode } from "react";
import { beforeEach, describe, it, vi } from "vitest";

import { ReplenishmentScreen } from "../ReplenishmentScreen";
import { CLASSIFICATION_LABELS } from "../api";

// Feature: report-export-migration, Property 12: Tombol ekspor mengirim filter layar
// Validates: Requirements 7.2, 7.4, 7.6

const { request } = vi.hoisted(() => ({ request: vi.fn() }));

vi.mock("@/features/exports", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/features/exports")>();
  return {
    ...actual,
    useExportFlow: () => ({
      request,
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
vi.mock("../api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../api")>();
  return {
    ...actual,
    useClassifications: () => ({
      isLoading: false,
      isError: false,
      data: { items: [], total: 0, page: 1, page_size: 20 },
      refetch: vi.fn(),
    }),
  };
});
vi.mock("@/features/vendor-request/hooks", () => ({
  useVendorOptions: () => ({ data: { vendors: [{ id: 2, name: "Abacus" }], regions: ["MDN"] } }),
}));
vi.mock("@tanstack/react-router", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    Link: ({ children }: { children?: ReactNode }) => <a href="/r">{children}</a>,
  };
});

const BASE = Date.UTC(2026, 0, 1);
const DAY = 86_400_000;
const iso = (offset: number) => new Date(BASE + offset * DAY).toISOString().slice(0, 10);

const screenState = fc.record({
  fromOffset: fc.integer({ min: 0, max: 90 }),
  // Independent oracle: valid iff 0 <= (to - from) <= 30 days (max 31 days inclusive).
  span: fc.integer({ min: -5, max: 45 }),
  vendor: fc.constantFrom(null, "2"),
  region: fc.constantFrom(null, "MDN"),
  classification: fc.constantFrom(null, ...Object.keys(CLASSIFICATION_LABELS)),
});

beforeEach(() => request.mockReset());

describe("Property 12: Replenishment export buttons send the screen filter", () => {
  it("requests equal the filter; classification only for amount-mismatch; none on an invalid range", () => {
    fc.assert(
      fc.property(screenState, (s) => {
        request.mockReset();
        const from = iso(s.fromOffset);
        const to = iso(s.fromOffset + s.span);
        const valid = s.span >= 0 && s.span <= 30;

        const { unmount } = render(<ReplenishmentScreen />);
        fireEvent.change(screen.getByLabelText("Dari tanggal"), { target: { value: from } });
        fireEvent.change(screen.getByLabelText("Sampai tanggal"), { target: { value: to } });
        if (s.vendor)
          fireEvent.change(screen.getByLabelText("Vendor"), { target: { value: s.vendor } });
        if (s.region)
          fireEvent.change(screen.getByLabelText("Region"), { target: { value: s.region } });
        if (s.classification) {
          fireEvent.change(screen.getByLabelText("Klasifikasi"), {
            target: { value: s.classification },
          });
        }
        const mismatch = screen.getByRole("button", { name: /Selisih nominal/ });
        const trips = screen.getByRole("button", { name: /Trip per ATM/ });
        const disabled =
          (mismatch as HTMLButtonElement).disabled && (trips as HTMLButtonElement).disabled;
        const enabled =
          !(mismatch as HTMLButtonElement).disabled && !(trips as HTMLButtonElement).disabled;
        fireEvent.click(mismatch);
        fireEvent.click(trips);
        unmount();

        if (!valid) return disabled && request.mock.calls.length === 0;
        if (!enabled || request.mock.calls.length !== 2) return false;

        const common: Record<string, unknown> = { date_from: from, date_to: to };
        if (s.vendor) common.vendor_id = 2;
        if (s.region) common.region_code = s.region;
        const mismatchParams = s.classification
          ? { ...common, classification: s.classification }
          : common;
        return (
          same(request.mock.calls[0]?.[0], {
            kind: "report",
            report: "replenishment-amount-mismatch",
            params: mismatchParams,
          }) &&
          same(request.mock.calls[1]?.[0], {
            kind: "report",
            report: "replenishment-trips",
            params: common,
          })
        );
      }),
      { numRuns: 50 },
    );
  });
});

function same(a: unknown, b: unknown): boolean {
  return JSON.stringify(sortKeys(a)) === JSON.stringify(sortKeys(b));
}
function sortKeys(v: unknown): unknown {
  if (v && typeof v === "object") {
    return Object.fromEntries(
      Object.entries(v as Record<string, unknown>)
        .sort(([x], [y]) => x.localeCompare(y))
        .map(([k, val]) => [k, sortKeys(val)]),
    );
  }
  return v;
}
