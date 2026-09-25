import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { DataTable } from "@/components/ui/DataTable";
import type { ColumnDef } from "@tanstack/react-table";
import { Ban, CheckCircle } from "lucide-react";
import type { AdminRegion } from "../types";

interface RegionsTableProps {
  regions: AdminRegion[];
  onEdit: (region: AdminRegion) => void;
  onDisable: (region: AdminRegion) => void;
  onEnable: (region: AdminRegion) => void;
}

/** Regions table (Req 1.6, 1.8): status is always icon + label, never color alone (Sec 13, a11y). */
export function RegionsTable({ regions, onEdit, onDisable, onEnable }: RegionsTableProps) {
  const columns: ColumnDef<AdminRegion, unknown>[] = [
    {
      accessorKey: "code",
      header: "Code",
      cell: ({ row }) => <span className="font-mono tabular-nums">{row.original.code}</span>,
    },
    {
      id: "region",
      header: "Nama",
      cell: ({ row }) => row.original.region ?? "-",
    },
    {
      id: "location_count",
      header: "Jumlah Lokasi",
      meta: { align: "right" },
      cell: ({ row }) => row.original.location_count,
    },
    {
      id: "status",
      header: "Status",
      cell: ({ row }) =>
        row.original.is_active ? (
          <Badge variant="success" icon={CheckCircle} label="Aktif" />
        ) : (
          <Badge variant="danger" icon={Ban} label="Nonaktif" />
        ),
    },
    {
      id: "actions",
      header: "Aksi",
      meta: { align: "right" },
      cell: ({ row }) => {
        const region = row.original;
        return (
          <div className="flex justify-end gap-2">
            <Button variant="secondary" onClick={() => onEdit(region)}>
              Ubah
            </Button>
            {region.is_active ? (
              <Button variant="danger" onClick={() => onDisable(region)}>
                Nonaktifkan
              </Button>
            ) : (
              <Button variant="secondary" onClick={() => onEnable(region)}>
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
      data={regions}
      columns={columns}
      emptyMessage="Tidak ada region yang sesuai dengan filter saat ini"
    />
  );
}
