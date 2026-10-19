/**
 * fe-coverage C5: vendor "Schedule" and "Invoices" prototype screens (hidden from the route
 * tree by fe-tech-debt S2, still counted per fe-coverage Q1). Data comes from src/data/*.json.
 */

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, renderHook, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const m = vi.hoisted(() => ({
  vendorId: "vendor-gardanet" as string | null,
  schedule: null as null | Record<string, unknown>,
  invoices: null as null | Record<string, unknown>,
}));
vi.mock("@/features/auth/useAuth", () => ({
  useAuth: () => ({ state: { user: m.vendorId ? { vendorId: m.vendorId } : null } }),
}));
vi.mock("@/features/schedule/useSchedule", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/features/schedule/useSchedule")>();
  return { useSchedule: () => m.schedule ?? actual.useSchedule() };
});
vi.mock("@/features/invoices/useInvoices", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/features/invoices/useInvoices")>();
  return { useInvoices: () => m.invoices ?? actual.useInvoices() };
});

import { SchedulePage } from "@/features/schedule/SchedulePage";
import { useSchedule } from "@/features/schedule/useSchedule";
import { InvoicesPage } from "../InvoicesPage";
import { useInvoices } from "../useInvoices";

function wrapper({ children }: { children: ReactNode }) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}

beforeEach(() => {
  m.vendorId = "vendor-gardanet";
  m.schedule = null;
  m.invoices = null;
});

describe("schedule + invoice data hooks", () => {
  it("return only the signed-in vendor's rows", async () => {
    const schedule = renderHook(() => useSchedule(), { wrapper });
    const invoices = renderHook(() => useInvoices(), { wrapper });

    await waitFor(() => expect(schedule.result.current.isSuccess).toBe(true));
    await waitFor(() => expect(invoices.result.current.isSuccess).toBe(true));
    expect(schedule.result.current.data?.length).toBeGreaterThan(0);
    expect(schedule.result.current.data?.every((s) => s.vendorId === "vendor-gardanet")).toBe(true);
    expect(invoices.result.current.data?.every((i) => i.vendorId === "vendor-gardanet")).toBe(true);
  });

  it("stay idle without a vendor", () => {
    m.vendorId = null;

    expect(renderHook(() => useSchedule(), { wrapper }).result.current.fetchStatus).toBe("idle");
    expect(renderHook(() => useInvoices(), { wrapper }).result.current.fetchStatus).toBe("idle");
  });
});

const sched = (over: Record<string, unknown>) => ({
  id: "S1",
  atmId: "ATM-1",
  location: "Sudirman",
  scheduledDate: "2026-10-16",
  recommendedAmount: 100_000_000,
  priority: "Low",
  status: "Pending",
  vendorId: "v",
  ...over,
});

describe("SchedulePage", () => {
  it("shows loading", () => {
    m.schedule = { isLoading: true, data: undefined };
    render(<SchedulePage />, { wrapper });

    expect(screen.getByText("Memuat data...")).toBeInTheDocument();
  });

  it("groups by date (oldest first), high priority first, with the day total", () => {
    m.schedule = {
      isLoading: false,
      data: [
        sched({ id: "S3", scheduledDate: "2026-10-17", status: "Executed" }),
        sched({ id: "S1", priority: "Low" }),
        sched({ id: "S2", priority: "High", recommendedAmount: 250_000_000, status: "Cancelled" }),
        sched({ id: "S4", priority: "Medium", status: "Confirmed" }),
      ],
    };
    render(<SchedulePage />, { wrapper });

    const sections = screen.getAllByRole("region");
    expect(sections).toHaveLength(2);
    const first = within(sections[0] as HTMLElement);
    expect(first.getByText("3 jadwal")).toBeInTheDocument();
    expect(first.getByText("Rp 450.000.000")).toBeInTheDocument();
    const ids = first
      .getAllByRole("row")
      .slice(1)
      .map((r) => r.firstChild?.textContent);
    expect(ids).toEqual(["S2", "S4", "S1"]);
    expect(within(sections[1] as HTMLElement).getByText("S3")).toBeInTheDocument();
  });

  it("filters by date range and shows the empty state when nothing matches", async () => {
    m.schedule = {
      isLoading: false,
      data: [sched({ id: "S1" }), sched({ id: "S2", scheduledDate: "2026-10-20" })],
    };
    render(<SchedulePage />, { wrapper });

    fireEvent.change(document.getElementById("date-start") as HTMLInputElement, {
      target: { value: "2026-10-18" },
    });
    expect(screen.queryByText("S1")).not.toBeInTheDocument();
    expect(screen.getByText("S2")).toBeInTheDocument();

    fireEvent.change(document.getElementById("date-end") as HTMLInputElement, {
      target: { value: "2026-10-19" },
    });
    expect(screen.getByText("Tidak ada jadwal ditemukan")).toBeInTheDocument();

    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Hapus tanggal akhir" }));
    await user.click(screen.getByRole("button", { name: "Hapus tanggal mulai" }));
    expect(screen.getByText("S1")).toBeInTheDocument();
  });
});

const invoice = (over: Record<string, unknown>) => ({
  id: "I1",
  invoiceNumber: "INV/1",
  period: "Okt 2026",
  totalAmount: 100_000_000,
  lineItemsCount: 1,
  validationStatus: "Uploaded",
  vendorId: "v",
  lineItems: [
    {
      description: "CIT 1",
      invoicedAmount: 100_000_000,
      matchedOrderRef: "CIT-1",
      expectedAmount: 100_000_000,
      variance: 0,
      matchStatus: "Match",
    },
  ],
  ...over,
});

describe("InvoicesPage", () => {
  it("shows loading and the empty state", () => {
    m.invoices = { isLoading: true, data: undefined };
    const { rerender } = render(<InvoicesPage />, { wrapper });
    expect(screen.getByText("Memuat data...")).toBeInTheDocument();

    m.invoices = { isLoading: false, data: [] };
    rerender(<InvoicesPage />);
    expect(screen.getByText("Belum ada invoice")).toBeInTheDocument();
  });

  it("summarises totals and status counts, and expands a row into its line items", async () => {
    m.invoices = {
      isLoading: false,
      data: [
        invoice({
          id: "A",
          invoiceNumber: "INV/A",
          validationStatus: "Approved",
          totalAmount: 300_000_000,
        }),
        invoice({
          id: "B",
          invoiceNumber: "INV/B",
          validationStatus: "Mismatch Detected",
          lineItems: [
            {
              description: "CIT selisih",
              invoicedAmount: 120_000_000,
              matchedOrderRef: "CIT-9",
              expectedAmount: 100_000_000,
              variance: 20_000_000,
              matchStatus: "Mismatch",
            },
          ],
        }),
        invoice({ id: "C", invoiceNumber: "INV/C", validationStatus: "Validated" }),
      ],
    };
    const user = userEvent.setup();
    render(<InvoicesPage />, { wrapper });

    expect(screen.getByText("IDR 500.000.000")).toBeInTheDocument(); // total invoiced
    expect(screen.getAllByText("IDR 300.000.000").length).toBeGreaterThan(0); // approved
    expect(screen.getByText("1 Approved")).toBeInTheDocument();
    expect(screen.getByText("1 Mismatch")).toBeInTheDocument();
    expect(screen.getByText("1 Validated")).toBeInTheDocument();
    expect(screen.getByText("0 Uploaded")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Invoice INV/B, expand details" }));
    expect(screen.getByText("CIT selisih")).toBeInTheDocument();
    expect(screen.getByText("IDR 20.000.000")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Invoice INV/B, collapse details" })).toHaveAttribute(
      "aria-expanded",
      "true",
    );

    await user.click(screen.getByText("INV/B"));
    expect(screen.queryByText("CIT selisih")).not.toBeInTheDocument();
  });

  it("a matched line item shows a dash for variance", async () => {
    m.invoices = { isLoading: false, data: [invoice({})] };
    render(<InvoicesPage />, { wrapper });

    await userEvent.setup().click(screen.getByRole("button", { name: /INV\/1, expand/ }));

    expect(screen.getByText("—")).toBeInTheDocument();
    expect(screen.getByText("Match")).toBeInTheDocument();
  });
});
