import { Button } from "@/components/ui/Button";
import { DataTable } from "@/components/ui/DataTable";
import { PageHeader } from "@/components/ui/PageHeader";
import { RoleBadge, StatusMessage } from "@/features/rbac-settings";
import type { ColumnDef } from "@tanstack/react-table";
import { useState } from "react";
import { useCatalog, useRoles } from "../hooks/useRoleQueries";
import type { RoleWithPermissions } from "../types";
import { CreateRoleDialog } from "./CreateRoleDialog";
import { PermissionEditor } from "./PermissionEditor";

/**
 * Role Management screen (Req 3.5, 7.2, 7.4): role list on the left,
 * PermissionEditor for the selected role on the right. Tema Merah Sirih —
 * red reserved for the primary "Buat Peran" action and focus halos only.
 */
export function RoleManagementPage() {
  const rolesQuery = useRoles();
  const catalogQuery = useCatalog();
  const [selectedRoleId, setSelectedRoleId] = useState<number | null>(null);
  const [isCreateOpen, setCreateOpen] = useState(false);

  const roles = rolesQuery.data?.roles ?? [];
  const catalog = catalogQuery.data?.catalog ?? [];
  const selectedRole = roles.find((r) => r.id === selectedRoleId) ?? null;

  const columns: ColumnDef<RoleWithPermissions, unknown>[] = [
    {
      id: "role",
      header: "Peran",
      cell: ({ row }) => <RoleBadge role={row.original.role} />,
    },
    {
      id: "description",
      header: "Deskripsi",
      cell: ({ row }) => row.original.description ?? "-",
    },
    {
      id: "permissions",
      header: "Jumlah Akses",
      meta: { align: "right" },
      cell: ({ row }) => row.original.permissions.length,
    },
    {
      id: "actions",
      header: "Aksi",
      meta: { align: "right" },
      cell: ({ row }) => (
        <div className="flex justify-end">
          <Button variant="secondary" onClick={() => setSelectedRoleId(row.original.id)}>
            Atur Izin
          </Button>
        </div>
      ),
    },
  ];

  const isLoading = rolesQuery.isLoading || catalogQuery.isLoading;
  const isError = rolesQuery.isError || catalogQuery.isError;

  return (
    <div className="flex flex-col gap-6 p-6">
      <PageHeader
        eyebrow="Pengaturan"
        title="Manajemen Peran"
        description="Kelola peran dan atur menu/fitur yang dapat diakses tiap peran."
        actions={<Button onClick={() => setCreateOpen(true)}>Buat Peran</Button>}
      />

      <StatusMessage
        isLoading={isLoading}
        isError={isError}
        isEmpty={!isLoading && !isError && roles.length === 0}
        loadingLabel="Memuat daftar peran…"
        errorMessage="Gagal memuat daftar peran. Coba lagi."
        emptyMessage="Belum ada peran."
      />

      {!isLoading && !isError && roles.length > 0 && (
        <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
          <DataTable data={roles} columns={columns} />

          <div className="flex flex-col gap-3">
            <h2 className="text-sm font-semibold uppercase tracking-wider text-[var(--n-600)]">
              {selectedRole ? `Izin — ${selectedRole.role}` : "Pilih peran untuk mengatur izin"}
            </h2>
            {selectedRole && <PermissionEditor role={selectedRole} catalog={catalog} />}
          </div>
        </div>
      )}

      <CreateRoleDialog open={isCreateOpen} onClose={() => setCreateOpen(false)} />
    </div>
  );
}
