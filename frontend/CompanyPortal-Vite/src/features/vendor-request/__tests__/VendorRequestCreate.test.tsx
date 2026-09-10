/**
 * Component tests for VendorRequestCreate (Req 12). Renders the page
 * standalone (no RouterProvider) — useNavigate/Link are mocked, same
 * approach as atm-portal-components.test.tsx — since it needs a real
 * backend to exercise via the browser (no live backend in this
 * environment), this is the meaningful automated check for its
 * non-trivial logic: total calculation, per-item amount validation, and
 * the no-selection empty state.
 */

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { VendorRequestCreate } from "../VendorRequestCreate";
import { usePendingVendorRequestSelection } from "../selectionStore";
import type { ForecastRow } from "../types";

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
};
const ROW_B: ForecastRow = {
  terminal_id: "ATM002",
  periode_pred: "2026-09-12",
  denom: 50000,
  amount_replenish: 2_000_000,
  amount_refund: 0,
  dmaa_file_id: 1,
};

beforeEach(() => {
  navigateSpy.mockReset();
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
      .setPending({ forecastDate: "2026-09-12", items: [ROW_A, ROW_B] });

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
