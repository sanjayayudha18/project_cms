import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { DataTable } from "@/components/ui/DataTable";
import { Link } from "@tanstack/react-router";
import type { ColumnDef } from "@tanstack/react-table";
import { CheckCircle, XCircle } from "lucide-react";
import type { AdminVendor } from "../types";

interface VendorsTableProps {
  vendors: AdminVendor[];
  onEdit: (vendor: AdminVendor) => void;
  onDisable: (vendor: AdminVendor) => void;
  onEnable: (vendor: AdminVendor) => void;
}

/** Vendors table (Req 6.1-6.4): status badge always carries icon + label (Sec 13, a11y). */
export function VendorsTable({ vendors, onEdit, onDisable, onEnable }: VendorsTableProps) {
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
        const vendor = row.original;
        return (
          <div className="flex justify-end gap-2">
            <Button variant="secondary" onClick={() => onEdit(vendor)}>
              Ubah
            </Button>
            {vendor.is_active ? (
              <Button variant="danger" onClick={() => onDisable(vendor)}>
                Nonaktifkan
              </Button>
            ) : (
              <Button variant="secondary" onClick={() => onEnable(vendor)}>
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
