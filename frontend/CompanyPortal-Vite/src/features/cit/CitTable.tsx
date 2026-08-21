import type { ColumnDef } from "@tanstack/react-table";
import { Calendar, CheckCircle, ExternalLink, Truck, XCircle } from "lucide-react";

import { Badge, type BadgeVariant } from "@/components/ui/Badge";
import { DataTable } from "@/components/ui/DataTable";
import { formatIDR } from "@/lib/utils/formatCurrency";

import type { CitStatus, EnrichedCitOrder } from "./types";

const statusConfig: Record<
  CitStatus,
  { variant: BadgeVariant; icon: typeof Calendar; label: string }
> = {
  Scheduled: { variant: "info", icon: Calendar, label: "Scheduled" },
  "In Transit": { variant: "warning", icon: Truck, label: "In Transit" },
  Completed: { variant: "success", icon: CheckCircle, label: "Completed" },
  Failed: { variant: "danger", icon: XCircle, label: "Failed" },
};

const columns: ColumnDef<EnrichedCitOrder, unknown>[] = [
  {
    accessorKey: "id",
    header: "Order ID",
  },
  {
    accessorKey: "atmId",
    header: "ATM ID",
  },
  {
    accessorKey: "vendorName",
    header: "Vendor",
  },
  {
    accessorKey: "orderDate",
    header: "Tanggal Order",
  },
  {
    accessorKey: "scheduledDate",
    header: "Tanggal Jadwal",
  },
  {
    accessorKey: "amount",
    header: "Jumlah (IDR)",
    meta: { align: "right" },
    cell: ({ getValue }) => formatIDR(getValue() as number),
  },
  {
    accessorKey: "status",
    header: "Status",
    cell: ({ getValue }) => {
      const status = getValue() as CitStatus;
      const config = statusConfig[status];
      return <Badge variant={config.variant} icon={config.icon} label={config.label} />;
    },
  },
  {
    accessorKey: "evidenceUrl",
    header: "Bukti",
    enableSorting: false,
    cell: ({ getValue }) => {
      const url = getValue() as string | null;
      if (!url) {
        return <span className="text-[var(--n-400)]">&mdash;</span>;
      }
      return (
        <a
          href={url}
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex items-center gap-1 min-h-[44px] min-w-[44px] px-2 text-sm text-[var(--red-600)] hover:text-[var(--red-700)] underline"
          aria-label="Lihat bukti (buka di tab baru)"
        >
          Lihat
          <ExternalLink className="h-3.5 w-3.5" aria-hidden="true" />
        </a>
      );
    },
  },
];

interface CitTableProps {
  data: EnrichedCitOrder[];
}

export function CitTable({ data }: CitTableProps) {
  return (
    <DataTable
      data={data}
      columns={columns}
      defaultSorting={[{ id: "scheduledDate", desc: true }]}
      emptyMessage="Tidak ada order CIT yang sesuai filter"
    />
  );
}
