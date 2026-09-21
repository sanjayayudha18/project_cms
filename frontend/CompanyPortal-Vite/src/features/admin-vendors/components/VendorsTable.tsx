import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { DataTable } from "@/components/ui/DataTable";
import { Link } from "@tanstack/react-router";
import type { ColumnDef } from "@tanstack/react-table";
import { CheckCircle, XCircle } from "lucide-react";
import { PendingApprovalBadge } from "../../master-data/PendingApprovalBadge";
import type { AdminVendor } from "../types";

interface VendorsTableProps {
  vendors: AdminVendor[];
  onEdit: (vendor: AdminVendor) => void;
  onDisable: (vendor: AdminVendor) => void;
  onEnable: (vendor: AdminVendor) => void;
  /** Records with a change waiting for approval: badged, and their actions locked. */
  pendingIds?: Set<number>;
}

/** Vendors table (Req 6.1-6.4): status badge always carries icon + label (Sec 13, a11y). */
export function VendorsTable({
  vendors,
  onEdit,
  onDisable,
  onEnable,
  pendingIds,
}: VendorsTableProps) {
  const columns: ColumnDef<AdminVendor, unknown>[] = [
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
    {
      id: "actions",
      header: "Aksi",
      meta: { align: "right" },
      cell: ({ row }) => {
        const vendor = row.original;
        return (
          <div className="flex justify-end gap-2">
            <Button
              variant="secondary"
              disabled={pendingIds?.has(vendor.id)}
              onClick={() => onEdit(vendor)}
            >
              Ubah
            </Button>
            {vendor.is_active ? (
              <Button
                variant="danger"
                disabled={pendingIds?.has(vendor.id)}
                onClick={() => onDisable(vendor)}
              >
                Nonaktifkan
              </Button>
            ) : (
              <Button
                variant="secondary"
                disabled={pendingIds?.has(vendor.id)}
                onClick={() => onEnable(vendor)}
              >
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
      data={vendors}
      columns={columns}
      emptyMessage="Tidak ada vendor yang sesuai dengan filter saat ini"
    />
  );
}
