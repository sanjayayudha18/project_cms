/**
 * Component tests for VendorRequestCreate (Req 12). Renders the page
 * standalone (no RouterProvider) — useNavigate/Link are mocked, same
 * approach as atm-portal-components.test.tsx — since it needs a real
 * backend to exercise via the browser (no live backend in this
 * environment), this is the meaningful automated check for its
 * non-trivial logic: total calculation, per-item amount validation, and
 * the no-selection empty state.
 */

import { api } from "@/lib/api/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { VendorRequestCreate } from "../VendorRequestCreate";
import { jakartaCalendarDateISO, tomorrowJakartaISO } from "../lib/nextBusinessDay";
import { usePendingVendorRequestSelection } from "../selectionStore";
import type { ForecastRow, VendorOptionsResponse } from "../types";

const navigateSpy = vi.fn();

vi.mock("@tanstack/react-router", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    useNavigate: () => navigateSpy,
    Link: ({ children, className }: { children?: ReactNode; className?: string }) => (
      <a href="/replenishment/forecast-browser" className={className}>
        {children}
      </a>
    ),
  };
});

// CIT-2 (Task 12): manual mode's vendor select needs GET /vendors data.
vi.mock("@/lib/api/client", () => ({ api: { get: vi.fn() } }));
const mockApiGet = vi.mocked(api.get);
const VENDOR_OPTIONS_RESPONSE: VendorOptionsResponse = {
  vendors: [{ id: 1, name: "TAG" }],
  regions: ["Jawa Barat"],
};

function renderWithProviders() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <VendorRequestCreate />
    </QueryClientProvider>,
  );
}

const ROW_A: ForecastRow = {
  terminal_id: "ATM001",
  periode_pred: "2026-09-12",
  denom: 100000,
  amount_replenish: 5_000_000,
  amount_refund: 0,
  dmaa_file_id: 1,
  lokasi_atm: "",
  brand: "",
  priority_class: "",
  paket: "",
  escrow: null,
  flm_vendor: "",
  flm_vendor_region: "",
};
const ROW_B: ForecastRow = {
  terminal_id: "ATM002",
  periode_pred: "2026-09-12",
  denom: 50000,
  amount_replenish: 2_000_000,
  amount_refund: 0,
  dmaa_file_id: 1,
  lokasi_atm: "",
  brand: "",
  priority_class: "",
  paket: "",
  escrow: null,
  flm_vendor: "",
  flm_vendor_region: "",
};

beforeEach(() => {
  navigateSpy.mockReset();
  mockApiGet.mockReset();
  mockApiGet.mockResolvedValue({ data: VENDOR_OPTIONS_RESPONSE, status: 200 });
});

afterEach(() => {
  usePendingVendorRequestSelection.getState().clearPending();
});

describe("VendorRequestCreate", () => {
  it("shows an empty state with a link back to Forecast Browser when nothing was selected", () => {
    renderWithProviders();

    expect(screen.getByText(/tidak ada item yang dipilih/i)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /kembali ke forecast browser/i })).toBeInTheDocument();
  });

  it("renders selected items and computes the correct total", () => {
    usePendingVendorRequestSelection
      .getState()
      .setPending({ forecastDate: "2026-09-12", items: [ROW_A, ROW_B], vendorId: 1 });

    renderWithProviders();

    expect(screen.getByText("2026-09-12")).toBeInTheDocument();
    expect(screen.getByText("ATM001")).toBeInTheDocument();
    expect(screen.getByText("ATM002")).toBeInTheDocument();
    // Total: 5,000,000 + 2,000,000 = 7,000,000 -> "7.000.000" (id-ID grouping)
    expect(screen.getByText(/7\.000\.000/)).toBeInTheDocument();
  });

  it("disables both action buttons when an amount is edited to an invalid value", async () => {
    usePendingVendorRequestSelection.getState().setPending({
      forecastDate: "2026-09-12",
      items: [ROW_A],
      vendorId: 1,
    });
    const user = userEvent.setup();

    renderWithProviders();

    const amountInput = screen.getByLabelText(/amount replenish untuk atm001/i);
    await user.clear(amountInput);
    await user.type(amountInput, "0");

    const saveDraft = screen.getByRole("button", { name: /simpan sebagai draft/i });
    const submitForApproval = screen.getByRole("button", { name: /kirim untuk approval/i });

    expect(saveDraft).toBeDisabled();
    expect(submitForApproval).toBeDisabled();
  });

  it("removing the only row drops the count to zero and lets the total recompute", async () => {
    usePendingVendorRequestSelection.getState().setPending({
      forecastDate: "2026-09-12",
      items: [ROW_A],
      vendorId: 1,
    });
    const user = userEvent.setup();

    renderWithProviders();

    await user.click(screen.getByRole("button", { name: /hapus baris atm001/i }));

    expect(screen.queryByText("ATM001")).not.toBeInTheDocument();
    // Item count and total live in separate text nodes ("0" then " item —
    // Total: " then "Rp 0"), so match on the summary container's combined
    // text rather than a single getByText string.
    const summary = screen.getByText(/item — Total:/i).closest("div");
    expect(summary).toHaveTextContent("0 item");
    expect(summary).toHaveTextContent("Rp 0");
  });
});

describe("VendorRequestCreate — CIT-2 context columns + replenish_date (Task 12.2)", () => {
  // replenishment-request-enhancements Req 1.1/1.2: Kategori Request now
  // starts unselected on the DMAA flow too, so the date locks to H+1 only
  // after Planned is explicitly chosen — it's no longer pre-filled.
  it("locks Tanggal Replenish to calendar H+1 (Asia/Jakarta) once Planned is chosen on the DMAA flow", async () => {
    usePendingVendorRequestSelection
      .getState()
      .setPending({ forecastDate: "2026-09-12", items: [ROW_A], vendorId: 1 });
    const user = userEvent.setup();

    renderWithProviders();
    await user.selectOptions(screen.getByLabelText(/kategori request/i), "planned");

    const replenishInput = screen.getByLabelText(/tanggal replenish/i) as HTMLInputElement;
    expect(replenishInput.value).toBe(tomorrowJakartaISO());
    expect(replenishInput).toBeDisabled();
  });

  it("renders a plain hyphen for empty brand/flm_vendor/flm_vendor_region context columns", () => {
    usePendingVendorRequestSelection
      .getState()
      .setPending({ forecastDate: "2026-09-12", items: [ROW_A], vendorId: 1 }); // ROW_A has "" for all three

    renderWithProviders();

    const row = screen.getByText("ATM001").closest("tr");
    // 3 context columns (Brand, FLM Vendor, FLM Vendor Region), each "-".
    expect(row?.textContent?.match(/-/g)?.length).toBeGreaterThanOrEqual(3);
    expect(row?.textContent).not.toContain("—"); // never an em-dash
  });
});

describe("VendorRequestCreate — CIT-2 manual mode (Task 12.3)", () => {
  async function enterManualMode() {
    const user = userEvent.setup();
    renderWithProviders();
    await user.click(screen.getByRole("button", { name: /buat manual/i }));
    return user;
  }

  it("Planned locks Tanggal Replenish to H+1 and disables the input", async () => {
    const user = await enterManualMode();
    await user.selectOptions(screen.getByLabelText(/kategori request/i), "planned");

    const replenishInput = screen.getByLabelText(/tanggal replenish/i) as HTMLInputElement;
    expect(replenishInput.value).toBe(jakartaCalendarDateISO(1));
    expect(replenishInput).toBeDisabled();
  });

  it("Emergency locks Tanggal Replenish to H+0", async () => {
    const user = await enterManualMode();

    await user.selectOptions(screen.getByLabelText(/kategori request/i), "emergency");

    const replenishInput = screen.getByLabelText(/tanggal replenish/i) as HTMLInputElement;
    expect(replenishInput.value).toBe(jakartaCalendarDateISO(0));
    expect(replenishInput).toBeDisabled();
  });

  it("Additional offers a H+0/H+1/H+2 select instead of a locked input", async () => {
    const user = await enterManualMode();

    await user.selectOptions(screen.getByLabelText(/kategori request/i), "additional");

    const replenishSelect = screen.getByLabelText(/tanggal replenish/i) as HTMLSelectElement;
    expect(replenishSelect.tagName).toBe("SELECT");
    const optionValues = Array.from(replenishSelect.options).map((o) => o.value);
    expect(optionValues).toEqual([
      jakartaCalendarDateISO(0),
      jakartaCalendarDateISO(1),
      jakartaCalendarDateISO(2),
    ]);
  });

  it("accepts a manually added row's terminal_id/denom/amount_replenish", async () => {
    const user = await enterManualMode();
    await user.selectOptions(screen.getByLabelText(/kategori request/i), "planned");

    await user.click(screen.getByRole("button", { name: /tambah baris/i }));
    await user.type(screen.getByLabelText(/atm id baris 1/i), "T-MANUAL-1");
    await user.type(screen.getByLabelText(/denominasi baris 1/i), "100000");
    // aria-label falls back to "baris N", not the typed terminal_id: useFieldArray's
    // `field` snapshot doesn't reactively track keystrokes in the uncontrolled input.
    const amountInput = screen.getByLabelText(/amount replenish untuk baris 1/i);
    await user.clear(amountInput);
    await user.type(amountInput, "5000000");

    await user.selectOptions(await screen.findByLabelText(/^vendor \*/i), "1");

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /kirim untuk approval/i })).not.toBeDisabled();
    });
  });
});
