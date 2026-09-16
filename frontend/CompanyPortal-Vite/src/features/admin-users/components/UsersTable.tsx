import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { DataTable } from "@/components/ui/DataTable";
import type { AdminVendor } from "@/features/admin-vendors/types";
import type { ColumnDef } from "@tanstack/react-table";
import { CheckCircle, XCircle } from "lucide-react";
import type { AdminUser } from "../types";

interface UsersTableProps {
  users: AdminUser[];
  vendorsById: Map<number, AdminVendor>;
  onEdit: (user: AdminUser) => void;
  onDisable: (user: AdminUser) => void;
  onEnable: (user: AdminUser) => void;
}

/** Users table (Req 2.1-2.4): status badge always carries icon + label (Sec 13, a11y). */
export function UsersTable({ users, vendorsById, onEdit, onDisable, onEnable }: UsersTableProps) {
  const columns: ColumnDef<AdminUser, unknown>[] = [
    { accessorKey: "full_name", header: "Nama" },
    { accessorKey: "username", header: "Nama Pengguna" },
    { accessorKey: "email", header: "Email" },
    {
      id: "role",
      header: "Role",
      cell: ({ row }) => <Badge variant="neutral" label={row.original.role} />,
    },
    {
      id: "vendor",
      header: "Vendor",
      cell: ({ row }) => {
        const vendorId = row.original.vendor_id;
        if (vendorId === null) return "-";
        return vendorsById.get(vendorId)?.name ?? `#${vendorId}`;
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
        const user = row.original;
        return (
          <div className="flex justify-end gap-2">
            <Button variant="secondary" onClick={() => onEdit(user)}>
              Ubah
            </Button>
            {user.is_active ? (
              <Button variant="danger" onClick={() => onDisable(user)}>
                Nonaktifkan
              </Button>
            ) : (
              <Button variant="secondary" onClick={() => onEnable(user)}>
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
      data={users}
      columns={columns}
      emptyMessage="Tidak ada pengguna yang sesuai dengan filter saat ini"
    />
  );
}
