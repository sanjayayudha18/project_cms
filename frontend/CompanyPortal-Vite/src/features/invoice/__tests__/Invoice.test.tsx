import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

// ─── Mock data modules (hoisted so vi.mock factory can reference them) ────────

const { MOCK_INVOICES, MOCK_VENDORS } = vi.hoisted(() => ({
  MOCK_INVOICES: [
    {
      id: "INV-2024-001",
      vendorId: "V-001",
      period: "2024-01-15 to 2024-01-21",
      totalAmount: 1520000000,
      lineItemsCount: 3,
      validationStatus: "Approved",
      validatorName: "Budi Santoso",
      approverName: "Dewi Putri",
      approvedAt: "2024-01-22T10:30:00Z",
      lineItems: [
        {
          id: "LI-001-01",
          description: "CIT Service ATM-JKT-001 (16 Jan)",
          invoicedAmount: 250000000,
          matchedOrderRef: "CIT-20240115-001",
          expectedAmount: 250000000,
          variance: 0,
          matchStatus: "Matched",
        },
        {
          id: "LI-001-02",
          description: "CIT Service ATM-JKT-007 (17 Jan)",
          invoicedAmount: 400000000,
          matchedOrderRef: "CIT-20240116-003",
          expectedAmount: 400000000,
          variance: 0,
          matchStatus: "Matched",
        },
        {
          id: "LI-001-03",
          description: "CIT Service ATM-SBY-004 (19 Jan)",
          invoicedAmount: 280000000,
          matchedOrderRef: "CIT-20240118-001",
          expectedAmount: 280000000,
          variance: 0,
          matchStatus: "Matched",
        },
      ],
    },
    {
      id: "INV-2024-002",
      vendorId: "V-002",
      period: "2024-01-15 to 2024-01-21",
      totalAmount: 1300000000,
      lineItemsCount: 4,
      validationStatus: "Mismatch Detected",
      validatorName: "Rina Wati",
      approverName: null,
      approvedAt: null,
      lineItems: [
        {
          id: "LI-002-01",
          description: "CIT Service ATM-JKT-003 (16 Jan)",
          invoicedAmount: 350000000,
          matchedOrderRef: "CIT-20240115-002",
          expectedAmount: 350000000,
          variance: 0,
          matchStatus: "Matched",
        },
        {
          id: "LI-002-02",
          description: "CIT Service ATM-BDG-005 (18 Jan)",
          invoicedAmount: 300000000,
          matchedOrderRef: "CIT-20240117-001",
          expectedAmount: 300000000,
          variance: 0,
          matchStatus: "Matched",
        },
        {
          id: "LI-002-03",
          description: "CIT Service ATM-JKT-004 (18 Jan)",
          invoicedAmount: 320000000,
          matchedOrderRef: "CIT-20240117-002",
          expectedAmount: 300000000,
          variance: 20000000,
          matchStatus: "Mismatch",
        },
        {
          id: "LI-002-04",
          description: "CIT Service ATM-JKT-008 (19 Jan)",
          invoicedAmount: 300000000,
          matchedOrderRef: "CIT-20240118-002",
          expectedAmount: 300000000,
          variance: 0,
          matchStatus: "Matched",
        },
      ],
    },
  ],
  MOCK_VENDORS: [
    { id: "V-001", name: "PT Gardanet" },
    { id: "V-002", name: "PT SSI" },
  ],
}));

vi.mock("@/data/invoices.json", () => ({ default: MOCK_INVOICES }));
vi.mock("@/data/vendors.json", () => ({ default: MOCK_VENDORS }));

import { InvoiceDetail } from "../InvoiceDetail";
import { InvoiceFlow } from "../InvoiceFlow";
import type { InvoiceLineItem } from "../types";

// ─── InvoiceFlow — list table rendering ───────────────────────────────────────

describe("InvoiceFlow", () => {
  it("renders the page heading", () => {
    render(<InvoiceFlow />);
    expect(screen.getByRole("heading", { name: "Daftar Invoice" })).toBeInTheDocument();
  });

  it("renders all expected column headers", () => {
    render(<InvoiceFlow />);

    expect(screen.getByText("No. Invoice")).toBeInTheDocument();
    expect(screen.getByText("Periode")).toBeInTheDocument();
    expect(screen.getByText("Total (Rp)")).toBeInTheDocument();
    expect(screen.getByText("Jumlah Item")).toBeInTheDocument();
    expect(screen.getByText("Status Validasi")).toBeInTheDocument();
  });

  it("renders invoice rows with correct data", () => {
    render(<InvoiceFlow />);

    expect(screen.getByText("INV-2024-001")).toBeInTheDocument();
    expect(screen.getByText("INV-2024-002")).toBeInTheDocument();
  });

  it("renders validation status badges with Indonesian labels", () => {
    render(<InvoiceFlow />);

    expect(screen.getByText("Disetujui")).toBeInTheDocument();
    expect(screen.getByText("Selisih Terdeteksi")).toBeInTheDocument();
  });

  it("expands a row to show InvoiceDetail when clicked", async () => {
    const user = userEvent.setup();
    render(<InvoiceFlow />);

    const firstRow = screen.getByText("INV-2024-001").closest("tr")!;
    await user.click(firstRow);

    // After expansion, line item descriptions should be visible
    expect(screen.getByText("CIT Service ATM-JKT-001 (16 Jan)")).toBeInTheDocument();
    expect(screen.getByText("CIT Service ATM-JKT-007 (17 Jan)")).toBeInTheDocument();
    expect(screen.getByText("CIT Service ATM-SBY-004 (19 Jan)")).toBeInTheDocument();
  });

  it("shows InvoiceDetail column headers when row is expanded", async () => {
    const user = userEvent.setup();
    render(<InvoiceFlow />);

    const firstRow = screen.getByText("INV-2024-001").closest("tr")!;
    await user.click(firstRow);

    expect(screen.getByText("Deskripsi")).toBeInTheDocument();
    expect(screen.getByText("Jumlah Ditagih (Rp)")).toBeInTheDocument();
    expect(screen.getByText("Ref. Order")).toBeInTheDocument();
    expect(screen.getByText("Jumlah Ekspektasi (Rp)")).toBeInTheDocument();
    expect(screen.getByText("Selisih (Rp)")).toBeInTheDocument();
    expect(screen.getByText("Status Pencocokan")).toBeInTheDocument();
  });

  it("collapses the expanded row when clicked again", async () => {
    const user = userEvent.setup();
    render(<InvoiceFlow />);

    const firstRow = screen.getByText("INV-2024-001").closest("tr")!;
    await user.click(firstRow);
    expect(screen.getByText("CIT Service ATM-JKT-001 (16 Jan)")).toBeInTheDocument();

    // Click again to collapse
    await user.click(firstRow);
    expect(screen.queryByText("CIT Service ATM-JKT-001 (16 Jan)")).not.toBeInTheDocument();
  });
});

// ─── InvoiceFlow — empty state ────────────────────────────────────────────────

describe("InvoiceFlow — empty state", () => {
  it("shows empty state message when no invoices available", async () => {
    vi.resetModules();
    vi.doMock("@/data/invoices.json", () => ({ default: [] }));
    vi.doMock("@/data/vendors.json", () => ({ default: MOCK_VENDORS }));

    const { InvoiceFlow: EmptyInvoiceFlow } = await import("../InvoiceFlow");
    render(<EmptyInvoiceFlow />);

    expect(screen.getByText("Tidak ada invoice yang tersedia.")).toBeInTheDocument();
  });
});

// ─── InvoiceDetail Tests ──────────────────────────────────────────────────────

describe("InvoiceDetail", () => {
  const sampleLineItems: InvoiceLineItem[] = [
    {
      id: "LI-TEST-01",
      description: "CIT Service Test ATM",
      invoicedAmount: 250000000,
      matchedOrderRef: "CIT-20240101-001",
      expectedAmount: 250000000,
      variance: 0,
      matchStatus: "Matched",
    },
    {
      id: "LI-TEST-02",
      description: "CIT Emergency Service",
      invoicedAmount: 320000000,
      matchedOrderRef: null,
      expectedAmount: 300000000,
      variance: 20000000,
      matchStatus: "Mismatch",
    },
    {
      id: "LI-TEST-03",
      description: "CIT Service Pending",
      invoicedAmount: 150000000,
      matchedOrderRef: "CIT-20240102-003",
      expectedAmount: 150000000,
      variance: 0,
      matchStatus: "Pending Review",
    },
  ];

  it("renders all line item descriptions", () => {
    render(<InvoiceDetail lineItems={sampleLineItems} />);

    expect(screen.getByText("CIT Service Test ATM")).toBeInTheDocument();
    expect(screen.getByText("CIT Emergency Service")).toBeInTheDocument();
    expect(screen.getByText("CIT Service Pending")).toBeInTheDocument();
  });

  it("renders matched order references (or dash for null)", () => {
    render(<InvoiceDetail lineItems={sampleLineItems} />);

    expect(screen.getByText("CIT-20240101-001")).toBeInTheDocument();
    expect(screen.getByText("—")).toBeInTheDocument();
    expect(screen.getByText("CIT-20240102-003")).toBeInTheDocument();
  });

  it("renders match status badges with Indonesian labels", () => {
    render(<InvoiceDetail lineItems={sampleLineItems} />);

    expect(screen.getByText("Cocok")).toBeInTheDocument();
    expect(screen.getByText("Tidak Cocok")).toBeInTheDocument();
    expect(screen.getByText("Menunggu Review")).toBeInTheDocument();
  });

  it("renders all detail column headers", () => {
    render(<InvoiceDetail lineItems={sampleLineItems} />);

    expect(screen.getByText("Deskripsi")).toBeInTheDocument();
    expect(screen.getByText("Jumlah Ditagih (Rp)")).toBeInTheDocument();
    expect(screen.getByText("Ref. Order")).toBeInTheDocument();
    expect(screen.getByText("Jumlah Ekspektasi (Rp)")).toBeInTheDocument();
    expect(screen.getByText("Selisih (Rp)")).toBeInTheDocument();
    expect(screen.getByText("Status Pencocokan")).toBeInTheDocument();
  });

  it("renders exactly as many rows as line items", () => {
    const { container } = render(<InvoiceDetail lineItems={sampleLineItems} />);

    const tbody = container.querySelector("tbody")!;
    const rows = tbody.querySelectorAll("tr");
    expect(rows).toHaveLength(3);
  });
});
