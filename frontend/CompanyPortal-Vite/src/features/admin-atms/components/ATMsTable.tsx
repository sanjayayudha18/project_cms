import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { DataTable } from "@/components/ui/DataTable";
import type { ColumnDef } from "@tanstack/react-table";
import { CheckCircle, Star, XCircle } from "lucide-react";
import type { AdminATM } from "../types";

interface ATMsTableProps {
  atms: AdminATM[];
  onEdit: (atm: AdminATM) => void;
  onDisable: (atm: AdminATM) => void;
  onEnable: (atm: AdminATM) => void;
  onAssignments?: (atm: AdminATM) => void;
}

const PRIORITY_BADGE_VARIANT = {
  VIP: "warning",
  "Non VIP": "neutral",
  Industri: "info",
} as const;

/** ATMs table (Req 8.1-8.11): status and priority badges always carry icon + label (Sec 13, a11y). */
export function ATMsTable({ atms, onEdit, onDisable, onEnable, onAssignments }: ATMsTableProps) {
  const columns: ColumnDef<AdminATM, unknown>[] = [
    { accessorKey: "terminal_id", header: "Terminal ID" },
    {
      id: "location_name",
      header: "Lokasi",
      cell: ({ row }) => row.original.location_name ?? "-",
    },
    { accessorKey: "machine_type", header: "Tipe Mesin" },
    { accessorKey: "brand", header: "Brand" },
    { accessorKey: "deployment_type", header: "Deployment" },
    {
      id: "priority_class",
      header: "Prioritas",
      cell: ({ row }) => {
        const priority = row.original.priority_class;
        if (!priority) return "-";
        return <Badge variant={PRIORITY_BADGE_VARIANT[priority]} icon={Star} label={priority} />;
      },
    },
    {
      id: "status",
      header: "Status",
      cell: ({ row }) =>
        row.original.is_active ? (
          <Badge variant="success" icon={CheckCircle} label="Aktif" />
        ) : (
          <Badge variant="danger" icon={XCircle} label="Nonaktif" />
        ),
    },
    {
      id: "actions",
      header: "Aksi",
      meta: { align: "right" },
      cell: ({ row }) => {
        const atm = row.original;
        return (
          <div className="flex justify-end gap-2">
            <Button variant="secondary" onClick={() => onEdit(atm)}>
              Ubah
            </Button>
            {onAssignments && (
              <Button variant="secondary" onClick={() => onAssignments(atm)}>
                Kelolaan
              </Button>
            )}
            {atm.is_active ? (
              <Button variant="danger" onClick={() => onDisable(atm)}>
                Nonaktifkan
              </Button>
            ) : (
              <Button variant="secondary" onClick={() => onEnable(atm)}>
                Aktifkan
              </Button>
            )}
          </div>
        );
      },
    },
  ];

  return (
    <DataTable
      data={atms}
      columns={columns}
      emptyMessage="Tidak ada ATM yang sesuai dengan filter saat ini"
    />
  );
}
