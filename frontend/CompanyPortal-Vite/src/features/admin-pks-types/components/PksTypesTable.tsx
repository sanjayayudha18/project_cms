import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { DataTable } from "@/components/ui/DataTable";
import { PendingApprovalBadge } from "@/features/master-data/PendingApprovalBadge";
import type { ColumnDef } from "@tanstack/react-table";
import { Ban, CheckCircle } from "lucide-react";
import type { PksType } from "../types";

interface PksTypesTableProps {
  types: PksType[];
  pendingIds: Set<number>;
  busy?: boolean;
  onEdit: (pksType: PksType) => void;
  onDisable: (pksType: PksType) => void;
  onEnable: (pksType: PksType) => void;
  emptyMessage?: string;
}

export function PksTypesTable({
  types,
  pendingIds,
  busy = false,
  onEdit,
  onDisable,
  onEnable,
  emptyMessage = "Belum ada data type PKS",
}: PksTypesTableProps) {
  const columns: ColumnDef<PksType, unknown>[] = [
    {
      accessorKey: "code",
      header: "Kode",
      cell: ({ row }) => (
        <span className="font-mono tabular-nums font-medium text-[var(--n-800)]">
          {row.original.code}
        </span>
      ),
    },
    {
      accessorKey: "name",
      header: "Nama",
      cell: ({ row }) => <span>{row.original.name}</span>,
    },
    {
      id: "status",
      header: "Status",
      cell: ({ row }) => {
        const isActive = row.original.is_active;
        const isPending = pendingIds.has(row.original.id);
        return (
          <span className="inline-flex flex-wrap items-center gap-1.5">
            {isActive ? (
              <Badge variant="success" icon={CheckCircle} label="Aktif" />
            ) : (
              <Badge variant="danger" icon={Ban} label="Nonaktif" />
            )}
            {isPending && <PendingApprovalBadge />}
          </span>
        );
      },
    },
    {
      id: "in_use",
      header: "Dipakai",
      cell: ({ row }) =>
        row.original.in_use ? (
          <Badge variant="info" label="Ya" />
        ) : (
          <Badge variant="neutral" label="Tidak" />
        ),
    },
    {
      id: "actions",
      header: "Aksi",
      meta: { align: "right" },
      cell: ({ row }) => {
        const item = row.original;
        const isRowBusy = busy || pendingIds.has(item.id);

        return (
          <div className="flex flex-col items-end gap-1">
            <div className="flex items-center justify-end gap-2">
              <Button variant="secondary" disabled={isRowBusy} onClick={() => onEdit(item)}>
                Ubah
              </Button>
              {item.is_active ? (
                <Button
                  variant="danger"
                  disabled={isRowBusy || item.in_use}
                  onClick={() => onDisable(item)}
                >
                  Nonaktifkan
                </Button>
              ) : (
                <Button variant="secondary" disabled={isRowBusy} onClick={() => onEnable(item)}>
                  Aktifkan
                </Button>
              )}
            </div>
            {item.is_active && item.in_use && (
              <span className="text-xs text-[var(--n-500)] font-normal" aria-live="polite">
                Sedang digunakan oleh PKS vendor
              </span>
            )}
          </div>
        );
      },
    },
  ];

  return <DataTable data={types} columns={columns} emptyMessage={emptyMessage} />;
}
