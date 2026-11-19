import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { DataTable } from "@/components/ui/DataTable";
import { Link } from "@tanstack/react-router";
import type { ColumnDef } from "@tanstack/react-table";
import { CheckCircle, XCircle } from "lucide-react";
import { useMemo } from "react";
import { PendingApprovalBadge } from "../../master-data/PendingApprovalBadge";
import type { AdminVendor } from "../types";

interface VendorsTableProps {
  vendors: AdminVendor[];
  onEdit: (vendor: AdminVendor) => void;
  onDisable: (vendor: AdminVendor) => void;
  onEnable: (vendor: AdminVendor) => void;
  /** Records with a change waiting for approval: badged, and their actions locked. */
  pendingIds?: Set<number>;
  /** false (ATM-SPV, read-only) renders no Aksi column. */
  canWrite?: boolean;
}

/** PKS column: "N PKS" or "Belum ada", both linking to the vendor's PKS tab; ROH (INTERNAL) shows "-". */
function PksCountCell({ vendor }: { vendor: AdminVendor }) {
  if (vendor.kind === "INTERNAL") return <span>-</span>;
  const count = vendor.pks_count ?? 0;
  return (
    <Link
      to="/settings/admin/vendors/$vendorId"
      params={{ vendorId: String(vendor.id) }}
      // the vendor detail page reads ?tab=
      search={{ tab: "pks" } as never}
      className="tabular-nums text-[var(--red-600)] underline"
    >
      {count > 0 ? `${count} PKS` : "Belum ada"}
    </Link>
  );
}

/** Vendors table (Req 7.3-7.5): one PKS column; status badge always carries icon + label (a11y). */
export function VendorsTable({
  vendors,
  onEdit,
  onDisable,
  onEnable,
  pendingIds,
  canWrite = true,
}: VendorsTableProps) {
  const columns: ColumnDef<AdminVendor, unknown>[] = useMemo(() => {
    const cols: ColumnDef<AdminVendor, unknown>[] = [
      {
        accessorKey: "code",
        header: "Kode",
        cell: ({ row }) => (
          <Link
            to="/settings/admin/vendors/$vendorId"
            params={{ vendorId: String(row.original.id) }}
            className="font-medium text-[var(--red-600)] underline"
          >
            {row.original.code}
          </Link>
        ),
      },
      { accessorKey: "name", header: "Nama" },
      {
        id: "director",
        header: "Nama Direktur Utama",
        cell: ({ row }) => row.original.director?.name ?? "-",
      },
      { id: "pks", header: "PKS", cell: ({ row }) => <PksCountCell vendor={row.original} /> },
      { accessorKey: "contact_email", header: "Email Kontak" },
      { accessorKey: "contact_phone", header: "Telepon" },
      {
        id: "status",
        header: "Status",
        cell: ({ row }) => (
          <span className="inline-flex flex-wrap items-center gap-1">
            {row.original.is_active ? (
              <Badge variant="success" icon={CheckCircle} label="Aktif" />
            ) : (
              <Badge variant="danger" icon={XCircle} label="Nonaktif" />
            )}
            {pendingIds?.has(row.original.id) && <PendingApprovalBadge />}
          </span>
        ),
      },
    ];
    if (canWrite) {
      cols.push({
        id: "actions",
        header: "Aksi",
        meta: { align: "right" },
        cell: ({ row }) => {
          const vendor = row.original;
          const locked = pendingIds?.has(vendor.id);
          return (
            <div className="flex justify-end gap-2">
              <Button variant="secondary" disabled={locked} onClick={() => onEdit(vendor)}>
                Ubah
              </Button>
              {vendor.is_active ? (
                <Button variant="danger" disabled={locked} onClick={() => onDisable(vendor)}>
                  Nonaktifkan
                </Button>
              ) : (
                <Button variant="secondary" disabled={locked} onClick={() => onEnable(vendor)}>
                  Aktifkan
                </Button>
              )}
            </div>
          );
        },
      });
    }
    return cols;
  }, [onEdit, onDisable, onEnable, pendingIds, canWrite]);

  return (
    <DataTable
      data={vendors}
      columns={columns}
      emptyMessage="Tidak ada vendor yang sesuai dengan filter saat ini"
    />
  );
}
