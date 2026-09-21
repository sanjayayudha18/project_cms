import { Button } from "@/components/ui/Button";
import { ConfirmActionDialog } from "@/components/ui/ConfirmActionDialog";
import { EmptyState } from "@/components/ui/EmptyState";
import { PageHeader } from "@/components/ui/PageHeader";
import { useToast } from "@/lib/hooks/useToast";
import { AlertCircle } from "lucide-react";
import { useState } from "react";
import { pendingApprovalMessage } from "../../master-data/changeRequest";
import { useDisableVendor, useEnableVendor, useVendorsList } from "../hooks";
import type { AdminVendor } from "../types";
import { useAdminVendorsUrlState } from "../useAdminVendorsUrlState";
import { VendorFilterBar } from "./VendorFilterBar";
import { VendorFormDialog } from "./VendorFormDialog";
import { VendorsTable } from "./VendorsTable";

const SKELETON_ROW_KEYS = ["row-1", "row-2", "row-3", "row-4", "row-5"];

/** Admin Vendors screen (Req 6-8): filterable/paginated table + create/edit/disable/enable. */
export function AdminVendorsPage() {
  const { params, searchInput, setSearchInput, setParams } = useAdminVendorsUrlState();
  const { toast } = useToast();

  const vendorsQuery = useVendorsList(params);
  const disableMutation = useDisableVendor();
  const enableMutation = useEnableVendor();

  const [formVendor, setFormVendor] = useState<AdminVendor | null>(null);
  const [isFormOpen, setFormOpen] = useState(false);
  const [pendingAction, setPendingAction] = useState<{
    type: "disable" | "enable";
    vendor: AdminVendor;
  } | null>(null);

  const vendors = vendorsQuery.data?.vendors ?? [];
  const total = vendorsQuery.data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / params.page_size));

  function openCreate(): void {
    setFormVendor(null);
    setFormOpen(true);
  }

  function openEdit(vendor: AdminVendor): void {
    setFormVendor(vendor);
    setFormOpen(true);
  }

  function confirmPendingAction(): void {
    if (!pendingAction) return;
    if (pendingAction.type === "disable") {
      disableMutation.mutate(pendingAction.vendor.id, {
        onSuccess: (res) => {
          // Req 8.5: disable still succeeds when the vendor has linked active
          // users -- surface that as a warning toast instead of success.
          toast({
            type: res.warning ? "warning" : "success",
            message: res.warning
              ? `${pendingApprovalMessage(res)}. ${res.warning}`
              : pendingApprovalMessage(res),
          });
          setPendingAction(null);
        },
        onError: (err) => {
          toast({ type: "error", message: err.message });
          setPendingAction(null);
        },
      });
      return;
    }
    enableMutation.mutate(pendingAction.vendor.id, {
      onSuccess: (res) => {
        toast({ type: "success", message: pendingApprovalMessage(res) });
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
        title="Manajemen Vendor"
        actions={<Button onClick={openCreate}>Tambah Vendor</Button>}
      />

      <VendorFilterBar
        params={params}
        searchInput={searchInput}
        onSearchInputChange={setSearchInput}
        onParamsChange={setParams}
      />

      {vendorsQuery.isLoading && (
        <div className="flex flex-col gap-2">
          {SKELETON_ROW_KEYS.map((key) => (
            <div key={key} className="h-10 w-full animate-pulse rounded bg-[var(--n-200)]" />
          ))}
        </div>
      )}

      {!vendorsQuery.isLoading && vendorsQuery.isError && (
        <div className="flex flex-col items-center gap-3 py-12">
          <AlertCircle className="h-10 w-10 text-[var(--danger-fg)]" aria-hidden="true" />
          <p className="text-sm text-[var(--n-700)]">Gagal memuat daftar vendor</p>
          <Button variant="secondary" onClick={() => vendorsQuery.refetch()}>
            Coba Lagi
          </Button>
        </div>
      )}

      {!vendorsQuery.isLoading && !vendorsQuery.isError && vendors.length === 0 && (
        <EmptyState message="Tidak ada vendor yang sesuai dengan filter saat ini" />
      )}

      {!vendorsQuery.isLoading && !vendorsQuery.isError && vendors.length > 0 && (
        <VendorsTable
          vendors={vendors}
          onEdit={openEdit}
          onDisable={(vendor) => setPendingAction({ type: "disable", vendor })}
          onEnable={(vendor) => setPendingAction({ type: "enable", vendor })}
        />
      )}

      <div className="flex flex-wrap items-center justify-between gap-4 text-sm text-[var(--n-600)]">
        <span>
          Halaman {params.page} dari {totalPages} ({total} vendor)
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

      <VendorFormDialog open={isFormOpen} onClose={() => setFormOpen(false)} vendor={formVendor} />

      <ConfirmActionDialog
        open={pendingAction !== null}
        title={pendingAction?.type === "disable" ? "Nonaktifkan Vendor" : "Aktifkan Vendor"}
        message={
          pendingAction?.type === "disable"
            ? `Nonaktifkan vendor "${pendingAction.vendor.name}"? Jika masih ada pengguna aktif yang terhubung, mereka tetap perlu dinonaktifkan terpisah.`
            : `Aktifkan kembali vendor "${pendingAction?.vendor.name}"?`
        }
        confirmLabel={pendingAction?.type === "disable" ? "Nonaktifkan" : "Aktifkan"}
        isPending={disableMutation.isPending || enableMutation.isPending}
        onConfirm={confirmPendingAction}
        onClose={() => setPendingAction(null)}
      />
    </div>
  );
}
