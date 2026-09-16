import { Button } from "@/components/ui/Button";
import { ConfirmActionDialog } from "@/components/ui/ConfirmActionDialog";
import { EmptyState } from "@/components/ui/EmptyState";
import { PageHeader } from "@/components/ui/PageHeader";
import { useVendorsList } from "@/features/admin-vendors/hooks";
import { useToast } from "@/lib/hooks/useToast";
import { AlertCircle } from "lucide-react";
import { useState } from "react";
import { useDisableUser, useEnableUser, useUsersList } from "../hooks";
import type { AdminUser } from "../types";
import { useAdminUsersUrlState } from "../useAdminUsersUrlState";
import { UserFilterBar } from "./UserFilterBar";
import { UserFormDialog } from "./UserFormDialog";
import { UsersTable } from "./UsersTable";

/** Admin Users screen (Req 2-5): filterable/paginated table + create/edit/disable/enable. */
export function AdminUsersPage() {
  const { params, searchInput, setSearchInput, setParams } = useAdminUsersUrlState();
  const { toast } = useToast();

  const usersQuery = useUsersList(params);
  const vendorsQuery = useVendorsList({ page: 1, page_size: 100, status: "all" });
  const vendorsById = new Map((vendorsQuery.data?.vendors ?? []).map((v) => [v.id, v]));

  const disableMutation = useDisableUser();
  const enableMutation = useEnableUser();

  const [formUser, setFormUser] = useState<AdminUser | null>(null);
  const [isFormOpen, setFormOpen] = useState(false);
  const [pendingAction, setPendingAction] = useState<{
    type: "disable" | "enable";
    user: AdminUser;
  } | null>(null);

  const users = usersQuery.data?.users ?? [];
  const total = usersQuery.data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / params.page_size));

  function openCreate(): void {
    setFormUser(null);
    setFormOpen(true);
  }

  function openEdit(user: AdminUser): void {
    setFormUser(user);
    setFormOpen(true);
  }

  function confirmPendingAction(): void {
    if (!pendingAction) return;
    const mutation = pendingAction.type === "disable" ? disableMutation : enableMutation;
    mutation.mutate(pendingAction.user.id, {
      onSuccess: (res) => {
        toast({ type: "success", message: res.message });
        setPendingAction(null);
      },
      onError: (err) => {
        toast({ type: "error", message: err.message });
        setPendingAction(null);
      },
    });
  }

  return (
    <div className="flex flex-col gap-6 p-6">
      <PageHeader
        eyebrow="Pengaturan"
        title="Manajemen Pengguna"
        actions={<Button onClick={openCreate}>Tambah Pengguna</Button>}
      />

      <UserFilterBar
        params={params}
        searchInput={searchInput}
        onSearchInputChange={setSearchInput}
        onParamsChange={setParams}
      />

      {usersQuery.isLoading && <SkeletonTable />}

      {!usersQuery.isLoading && usersQuery.isError && (
        <div className="flex flex-col items-center gap-3 py-12">
          <AlertCircle className="h-10 w-10 text-[var(--danger-fg)]" aria-hidden="true" />
          <p className="text-sm text-[var(--n-700)]">Gagal memuat daftar pengguna</p>
          <Button variant="secondary" onClick={() => usersQuery.refetch()}>
            Coba Lagi
          </Button>
        </div>
      )}

      {!usersQuery.isLoading && !usersQuery.isError && users.length === 0 && (
        <EmptyState message="Tidak ada pengguna yang sesuai dengan filter saat ini" />
      )}

      {!usersQuery.isLoading && !usersQuery.isError && users.length > 0 && (
        <UsersTable
          users={users}
          vendorsById={vendorsById}
          onEdit={openEdit}
          onDisable={(user) => setPendingAction({ type: "disable", user })}
          onEnable={(user) => setPendingAction({ type: "enable", user })}
        />
      )}

      <div className="flex flex-wrap items-center justify-between gap-4 text-sm text-[var(--n-600)]">
        <span>
          Halaman {params.page} dari {totalPages} ({total} pengguna)
        </span>
        <div className="flex items-center gap-2">
          <Button
            variant="secondary"
            disabled={params.page <= 1}
            onClick={() => setParams({ page: params.page - 1 })}
          >
            Sebelumnya
          </Button>
          <Button
            variant="secondary"
            disabled={params.page >= totalPages}
            onClick={() => setParams({ page: params.page + 1 })}
          >
            Berikutnya
          </Button>
        </div>
      </div>

      <UserFormDialog open={isFormOpen} onClose={() => setFormOpen(false)} user={formUser} />

      <ConfirmActionDialog
        open={pendingAction !== null}
        title={pendingAction?.type === "disable" ? "Nonaktifkan Pengguna" : "Aktifkan Pengguna"}
        message={
          pendingAction?.type === "disable"
            ? `Nonaktifkan akun "${pendingAction.user.full_name}"? Pengguna tidak akan bisa login sampai diaktifkan kembali.`
            : `Aktifkan kembali akun "${pendingAction?.user.full_name}"?`
        }
        confirmLabel={pendingAction?.type === "disable" ? "Nonaktifkan" : "Aktifkan"}
        isPending={disableMutation.isPending || enableMutation.isPending}
        onConfirm={confirmPendingAction}
        onClose={() => setPendingAction(null)}
      />
    </div>
  );
}

const SKELETON_ROW_KEYS = ["row-1", "row-2", "row-3", "row-4", "row-5"];

function SkeletonTable() {
  return (
    <div className="flex flex-col gap-2">
      {SKELETON_ROW_KEYS.map((key) => (
        <div key={key} className="h-10 w-full animate-pulse rounded bg-[var(--n-200)]" />
      ))}
    </div>
  );
}
