import {
  type ExpandedState,
  createColumnHelper,
  flexRender,
  getCoreRowModel,
  getExpandedRowModel,
  useReactTable,
} from "@tanstack/react-table";
import {
  AlertTriangle,
  CheckCircle,
  ChevronDown,
  ChevronRight,
  Upload,
  XCircle,
} from "lucide-react";
import { Fragment, useMemo, useState } from "react";

import { Badge } from "@/components/ui/Badge";
import type { BadgeVariant } from "@/components/ui/Badge";
import { EmptyState } from "@/components/ui/EmptyState";
import { formatIDR } from "@/lib/utils/formatCurrency";
import { InvoiceDetail } from "./InvoiceDetail";
import type { Invoice, InvoiceWithVendor, ValidationStatus } from "./types";

import invoicesData from "@/data/invoices.json";
import vendorsData from "@/data/vendors.json";

// ─── Data Loading ─────────────────────────────────────────────────────────────

const vendorMap = new Map(vendorsData.map((v) => [v.id, v.name]));

function loadInvoices(): InvoiceWithVendor[] {
  return (invoicesData as Invoice[]).map((invoice) => ({
    ...invoice,
    vendorName: vendorMap.get(invoice.vendorId) ?? "Vendor Tidak Diketahui",
  }));
}

// ─── Status Config ────────────────────────────────────────────────────────────

const statusConfig: Record<
  ValidationStatus,
  { variant: BadgeVariant; icon: typeof Upload; label: string }
> = {
  Uploaded: { variant: "info", icon: Upload, label: "Diunggah" },
  Validated: { variant: "warning", icon: AlertTriangle, label: "Tervalidasi" },
  Approved: { variant: "success", icon: CheckCircle, label: "Disetujui" },
  "Mismatch Detected": { variant: "danger", icon: XCircle, label: "Selisih Terdeteksi" },
};

// ─── Table Column Definitions ─────────────────────────────────────────────────

const columnHelper = createColumnHelper<InvoiceWithVendor>();

const columns = [
  columnHelper.display({
    id: "expander",
    cell: ({ row }) => (
      <button
        type="button"
        onClick={(e) => {
          e.stopPropagation();
          row.getToggleExpandedHandler()();
        }}
        className="p-1 text-[var(--n-500)] hover:text-[var(--n-700)] transition-colors duration-100"
        aria-label={row.getIsExpanded() ? "Tutup detail" : "Buka detail"}
      >
        {row.getIsExpanded() ? (
          <ChevronDown className="h-4 w-4" />
        ) : (
          <ChevronRight className="h-4 w-4" />
        )}
      </button>
    ),
  }),
  columnHelper.accessor("id", {
    header: "No. Invoice",
    cell: (info) => <span className="font-medium text-[var(--n-900)]">{info.getValue()}</span>,
  }),
  columnHelper.accessor("period", {
    header: "Periode",
    cell: (info) => info.getValue(),
  }),
  columnHelper.accessor("totalAmount", {
    header: "Total (Rp)",
    cell: (info) => <span className="tabular-nums">{formatIDR(info.getValue())}</span>,
  }),
  columnHelper.accessor("lineItemsCount", {
    header: "Jumlah Item",
    cell: (info) => <span className="tabular-nums">{info.getValue()}</span>,
  }),
  columnHelper.accessor("validationStatus", {
    header: "Status Validasi",
    cell: (info) => {
      const config = statusConfig[info.getValue()];
      return <Badge variant={config.variant} icon={config.icon} label={config.label} />;
    },
  }),
];

// ─── InvoiceFlow Component ────────────────────────────────────────────────────

export function InvoiceFlow() {
  const invoices = useMemo(() => loadInvoices(), []);
  const [expanded, setExpanded] = useState<ExpandedState>({});

  const table = useReactTable({
    data: invoices,
    columns,
    state: { expanded },
    onExpandedChange: setExpanded,
    getCoreRowModel: getCoreRowModel(),
    getExpandedRowModel: getExpandedRowModel(),
    getRowCanExpand: () => true,
  });

  if (invoices.length === 0) {
    return (
      <div className="flex flex-col gap-[var(--space-4)]">
        <h1 className="text-xl font-semibold text-[var(--n-900)]">Daftar Invoice</h1>
        <EmptyState message="Tidak ada invoice yang tersedia." />
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-[var(--space-4)]">
      <h1 className="text-xl font-semibold text-[var(--n-900)]">Daftar Invoice</h1>

      <div className="overflow-x-auto rounded-[var(--radius-lg)] border border-[var(--n-200)]">
        <table className="w-full border-collapse text-sm">
          <thead>
            {table.getHeaderGroups().map((headerGroup) => (
              <tr key={headerGroup.id}>
                {headerGroup.headers.map((header) => (
                  <th
                    key={header.id}
                    className={`px-4 py-3 text-xs font-medium uppercase tracking-wider text-[var(--n-500)] bg-[var(--n-50)] border-b border-[var(--n-100)] ${
                      header.id === "totalAmount" || header.id === "lineItemsCount"
                        ? "text-right"
                        : "text-left"
                    }`}
                  >
                    {header.isPlaceholder
                      ? null
                      : flexRender(header.column.columnDef.header, header.getContext())}
                  </th>
                ))}
              </tr>
            ))}
          </thead>
          <tbody>
            {table.getRowModel().rows.map((row) => (
              <Fragment key={row.id}>
                <tr
                  className="border-b border-[var(--n-100)] transition-colors duration-100 hover:bg-[var(--red-50)] cursor-pointer"
                  onClick={row.getToggleExpandedHandler()}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" || e.key === " ") {
                      e.preventDefault();
                      row.getToggleExpandedHandler()();
                    }
                  }}
                  tabIndex={0}
                  // biome-ignore lint/a11y/useSemanticElements: must stay a <tr> for valid table semantics — a <button> here would break the table structure for assistive tech.
                  role="button"
                  aria-expanded={row.getIsExpanded()}
                >
                  {row.getVisibleCells().map((cell) => (
                    <td
                      key={cell.id}
                      className={`px-4 py-3 ${
                        cell.column.id === "totalAmount" || cell.column.id === "lineItemsCount"
                          ? "text-right"
                          : ""
                      }`}
                    >
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </td>
                  ))}
                </tr>
                {row.getIsExpanded() && (
                  <tr>
                    <td colSpan={row.getVisibleCells().length} className="p-0">
                      <InvoiceDetail lineItems={row.original.lineItems} />
                    </td>
                  </tr>
                )}
              </Fragment>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
