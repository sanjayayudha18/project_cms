import { DataTable } from "@/components/ui/DataTable";
import { Link } from "@tanstack/react-router";
import type { ColumnDef } from "@tanstack/react-table";
import { type ReactNode, useMemo } from "react";
import { PendingApprovalBadge } from "../../master-data/PendingApprovalBadge";
import { formatMasaBerlaku } from "../lib/pks";
import type { PksListItem } from "../types";
import { PksStatusBadge } from "./PksStatusBadge";

export interface PksTableProps {
  items: PksListItem[];
  /** Set when scoped to one vendor (vendor tab): the Nama Vendor column is hidden. */
  vendorCode?: string;
  /**
   * Slot for per-row write actions (task 7.3); return null for rows without actions.
   * Omit or pass false to render no Aksi column (read-only, e.g. ATM-SPV).
   */
  actions?: ((item: PksListItem) => ReactNode) | false;
  emptyMessage?: string;
}

const dateCell = (v: string) => <span className="tabular-nums">{v}</span>;

/** A real link in the Judul cell opens the vendor's PKS tab, so keyboard users can activate the row (Req 5.5). */
function JudulLink({ item }: { item: PksListItem }) {
  return (
    <Link
      to="/settings/admin/vendors/$vendorId"
      params={{ vendorId: String(item.vendor_id) }}
      // the vendor detail page reads ?tab= (task 7.4)
      search={{ tab: "pks" } as never}
      className="font-medium text-[var(--red-600)] underline"
    >
      {item.pks_name}
    </Link>
  );
}

/** Daftar PKS table, columns per D12: Status, Type, Nama Vendor, Judul, No. WOD, dates, Masa Berlaku, Reminder. */
export function PksTable({ items, vendorCode, actions, emptyMessage }: PksTableProps) {
  const columns = useMemo<ColumnDef<PksListItem, unknown>[]>(() => {
    const cols: ColumnDef<PksListItem, unknown>[] = [
      {
        id: "status",
        header: "Status",
        cell: ({ row }) => (
          <span className="inline-flex flex-wrap items-center gap-1">
            <PksStatusBadge status={row.original.status} />
            {row.original.pending && row.original.status !== "menunggu_persetujuan" && (
              <span className="inline-flex flex-col gap-0.5">
                <PendingApprovalBadge />
                <span className="text-xs text-[var(--n-600)]">{row.original.pending.op}</span>
              </span>
            )}
          </span>
        ),
      },
      {
        id: "type",
        header: "Type",
        cell: ({ row }) => <span title={row.original.pks_type_name}>{row.original.pks_type}</span>,
      },
    ];
    if (!vendorCode) {
      cols.push({
        id: "vendor",
        header: "Nama Vendor",
        cell: ({ row }) => row.original.vendor_name,
      });
    }
    cols.push(
      { id: "pks_name", header: "Judul PKS", cell: ({ row }) => <JudulLink item={row.original} /> },
      { id: "wod", header: "No. WOD", cell: ({ row }) => row.original.wod_number },
      {
        id: "effective",
        header: "Tgl Efektif",
        cell: ({ row }) => dateCell(row.original.effective_date),
      },
      {
        id: "expiry",
        header: "Tgl Masa Akhir",
        cell: ({ row }) => dateCell(row.original.expiry_date),
      },
      {
        id: "masa",
        header: "Masa Berlaku",
        meta: { align: "right" },
        cell: ({ row }) => formatMasaBerlaku(row.original.status, row.original.remaining_days),
      },
      {
        id: "reminder",
        header: "Reminder (hari)",
        meta: { align: "right" },
        cell: ({ row }) => row.original.renewal_days,
      },
    );
    if (actions) {
      cols.push({
        id: "actions",
        header: "Aksi",
        meta: { align: "right" },
        cell: ({ row }) => actions(row.original),
      });
    }
    return cols;
  }, [vendorCode, actions]);

  return (
    <DataTable
      data={items}
      columns={columns}
      emptyMessage={emptyMessage ?? "Tidak ada PKS yang sesuai dengan filter saat ini"}
    />
  );
}
